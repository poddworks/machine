package cert

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	path "path/filepath"
	"time"
)

func NewX509Certificate(org string) (*x509.Certificate, error) {
	now := time.Now()
	notBefore := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-5, 0, 0, time.Local)
	notAfter := notBefore.Add(time.Hour * 24 * 1080)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	} else {
		return &x509.Certificate{
			SerialNumber:          serialNumber,
			Subject:               pkix.Name{Organization: []string{org}},
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
			BasicConstraintsValid: true,
		}, nil
	}
}

func LoadCACert(certpath string) (cert tls.Certificate, err error) {
	caFile := path.Join(certpath, "ca.pem")
	caKeyFile := path.Join(certpath, "ca-key.pem")
	cert, err = tls.LoadX509KeyPair(caFile, caKeyFile)
	return
}

func WriteCertificate(output string, data []byte) error {
	cert, err := os.Create(output)
	if err != nil {
		return err
	}
	defer cert.Close()
	return pem.Encode(cert, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: data,
	})
}

func WriteKey(output string, priv *rsa.PrivateKey) error {
	key, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer key.Close()
	return pem.Encode(key, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

func GenerateCertificate(certpath string, tmpl *x509.Certificate, hosts []string) (cert, key *bytes.Buffer, err error) {
	cert, key = new(bytes.Buffer), new(bytes.Buffer)

	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	caCert, err := LoadCACert(certpath)
	if err != nil {
		return // Unable to load CA Certificate
	}
	x509Cert, err := x509.ParseCertificate(caCert.Certificate[0])
	if err != nil {
		return // Unable to Parse CA Certificate
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return // Unable to generate private key for certificate
	}
	err = pem.Encode(key, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	if err != nil {
		return // Unable to encode private key to PEM
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, x509Cert, &priv.PublicKey, caCert.PrivateKey)
	if err != nil {
		return // Unable to create Certificate
	}
	err = pem.Encode(cert, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
	if err != nil {
		return // Unable to encode Certificate to PEM
	}

	// retrieve encoded PEM bytes for Certificate and Key
	return
}

type PemBlock struct {
	Name string
	Buf  *bytes.Buffer
}

func NewPemBlock(name string, block []byte) *PemBlock {
	return &PemBlock{
		Name: name,
		Buf:  bytes.NewBuffer(block),
	}
}

func GenerateServerCertificate(certpath, org string, hosts []string) (ca, cert, key *PemBlock, err error) {
	tmpl, err := NewX509Certificate(org)
	if err != nil {
		return
	}
	Cert, Key, err := GenerateCertificate(certpath, tmpl, hosts)
	if err != nil {
		return
	}
	CA, err := ioutil.ReadFile(path.Join(certpath, "ca.pem"))
	if err != nil {
		return
	}
	ca = NewPemBlock("ca.pem", CA)
	cert = &PemBlock{"server-cert.pem", Cert}
	key = &PemBlock{"server-key.pem", Key}
	return
}

func GenerateCACertificate(org, certpath string) (err error) {
	tmpl, err := NewX509Certificate(org)
	if err != nil {
		return
	}
	tmpl.IsCA = true
	tmpl.KeyUsage |= x509.KeyUsageCertSign
	tmpl.KeyUsage |= x509.KeyUsageKeyEncipherment
	tmpl.KeyUsage |= x509.KeyUsageKeyAgreement

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return
	}

	err = WriteCertificate(path.Join(certpath, "ca.pem"), derBytes)
	if err != nil {
		return
	}

	err = WriteKey(path.Join(certpath, "ca-key.pem"), priv)
	return
}

func GenerateClientCertificate(certpath, org string) (ca, cert, key *PemBlock, err error) {
	certBuf, keyBuf := new(bytes.Buffer), new(bytes.Buffer)

	tmpl, err := NewX509Certificate(org)
	if err != nil {
		return
	}
	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	tmpl.KeyUsage = x509.KeyUsageDigitalSignature

	caCert, err := LoadCACert(certpath)
	if err != nil {
		return // Unable to load CA Certificate
	}
	x509Cert, err := x509.ParseCertificate(caCert.Certificate[0])
	if err != nil {
		return // Unable to Parse CA Certificate
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return // Unable to generate private key for certificate
	}
	err = pem.Encode(keyBuf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	if err != nil {
		return // Unable to encode private key to PEM
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, x509Cert, &priv.PublicKey, caCert.PrivateKey)
	if err != nil {
		return // Unable to create Certificate
	}
	err = pem.Encode(certBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
	if err != nil {
		return // Unable to encode Certificate to PEM
	}

	CA, err := ioutil.ReadFile(path.Join(certpath, "ca.pem"))
	if err != nil {
		return
	}

	ca = NewPemBlock("ca.pem", CA)
	cert = &PemBlock{"cert.pem", certBuf}
	key = &PemBlock{"key.pem", keyBuf}

	// retrieve encoded PEM bytes for Certificate and Key
	return
}
