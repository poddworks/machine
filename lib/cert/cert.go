package cert

import (
	"github.com/codegangsta/cli"

	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	path "path/filepath"
	"strings"
	"time"
)

func NewX509Certificate(org string) (*x509.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour * 24 * 1080)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	} else {
		keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement
		return &x509.Certificate{
			SerialNumber:          serialNumber,
			Subject:               pkix.Name{Organization: []string{org}},
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              keyUsage,
			BasicConstraintsValid: true,
		}, nil
	}
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
	key, err := os.Create(output)
	if err != nil {
		return err
	}
	defer key.Close()
	return pem.Encode(key, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

func gencacert(c *cli.Context, certpath string) {
	tmpl, err := NewX509Certificate(c.String("organization"))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	tmpl.IsCA = true
	tmpl.KeyUsage |= x509.KeyUsageCertSign

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = WriteCertificate(path.Join(certpath, "ca.pem"), derBytes)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = WriteKey(path.Join(certpath, "ca-key.pem"), priv)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func gencert(c *cli.Context, certpath string) {
	tmpl, err := NewX509Certificate(c.String("organization"))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	tmpl.KeyUsage = x509.KeyUsageDigitalSignature

	caFile, caKeyFile := path.Join(certpath, "ca.pem"), path.Join(certpath, "ca-key.pem")
	tlsCert, err := tls.LoadX509KeyPair(caFile, caKeyFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, x509Cert, &priv.PublicKey, priv)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = WriteCertificate(path.Join(certpath, "cert.pem"), derBytes)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = WriteKey(path.Join(certpath, "key.pem"), priv)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func parseCertPath(c *cli.Context) (certpath string, err error) {
	certpath = c.Parent().String("certpath")
	certpath = strings.Replace(certpath, "~", os.Getenv("HOME"), 1)
	certpath, err = path.Abs(certpath)
	if err != nil {
		return
	}
	err = os.MkdirAll(certpath, 0700)
	return
}

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "tls",
		Usage: "Utility for generating certificate for TLS",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "certpath", Value: "~/.machine", Usage: "Certificate path"},
		},
		Subcommands: []cli.Command{
			{
				Name:  "bootstrap",
				Usage: "Generate certificate for TLS",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "organization", Value: "podd.org", Usage: "Organization for CA"},
				},
				Action: func(c *cli.Context) {
					certpath, err := parseCertPath(c)
					if err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					gencacert(c, certpath)
					gencert(c, certpath)
				},
			},
			{
				Name:  "install",
				Usage: "Install server certificate",
			},
		},
	}
}
