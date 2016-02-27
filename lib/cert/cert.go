package cert

import (
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"

	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"os/user"
	path "path/filepath"
	"strings"
	"time"
)

const (
	DEFAULT_CERT_PATH = "~/.machine"

	DEFAULT_ORGANIZATION_PLACEMENT_NAME = "podd.org"
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

func gencacert(c *cli.Context, org, certpath string) {
	tmpl, err := NewX509Certificate(org)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	tmpl.IsCA = true
	tmpl.KeyUsage |= x509.KeyUsageCertSign
	tmpl.KeyUsage |= x509.KeyUsageKeyEncipherment
	tmpl.KeyUsage |= x509.KeyUsageKeyAgreement

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

func gencert(c *cli.Context, org, certpath string) {
	tmpl, err := NewX509Certificate(org)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	tmpl.KeyUsage = x509.KeyUsageDigitalSignature

	caCert, err := LoadCACert(certpath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	x509Cert, err := x509.ParseCertificate(caCert.Certificate[0])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, x509Cert, &priv.PublicKey, caCert.PrivateKey)
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
	name string
	buf  *bytes.Buffer
}

func NewPemBlock(name string, block []byte) *PemBlock {
	return &PemBlock{name, bytes.NewBuffer(block)}
}

func SendEngineCertificate(ca, cert, key *PemBlock, cfg ssh.Config) error {
	const attempts = 5

	var sudo = ssh.New(cfg).Sudo()

	// Wait for SSH daemon online
	var idx = 0
	for ; idx < attempts; idx++ {
		if _, err := sudo.Run("date"); err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if idx == attempts {
		return fmt.Errorf("%s - Unable to contact remote", cfg.Server)
	}

	if err := sudo.Copy(cert.buf, int64(cert.buf.Len()), "/etc/docker/"+cert.name, 0644); err != nil {
		return err
	}
	fmt.Println(cfg.Server, "- Cert sent")

	if err := sudo.Copy(key.buf, int64(key.buf.Len()), "/etc/docker/"+key.name, 0600); err != nil {
		return err
	}
	fmt.Println(cfg.Server, "- Key sent")

	if err := sudo.Copy(ca.buf, int64(ca.buf.Len()), "/etc/docker/"+ca.name, 0644); err != nil {
		return err
	}
	fmt.Println(cfg.Server, "- CA sent")

	if err := sudo.ConfigureDockerTLS(); err != nil {
		return err
	}
	fmt.Println(cfg.Server, "- Configured Docker Engine")

	sudo.StopDocker()
	fmt.Println(cfg.Server, "- Stopped Docker Engine")

	if err := sudo.StartDocker(); err != nil {
		return err
	}
	fmt.Println(cfg.Server, "- Started Docker Engine")

	return nil
}

func parseCertArgs(c *cli.Context) (org, certpath string, err error) {
	usr, err := user.Current()
	if err != nil {
		return // Unable to determine user
	}
	org = c.Parent().String("organization")
	certpath = c.Parent().String("certpath")
	certpath = strings.Replace(certpath, "~", usr.HomeDir, 1)
	certpath, err = path.Abs(certpath)
	if err != nil {
		return
	}
	err = os.MkdirAll(certpath, 0700)
	return
}

func generateServerCertificate(c *cli.Context) (ca, cert, key *PemBlock) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		fmt.Println("You must provide hostname to create Certificate for")
		os.Exit(1)
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := parseCertArgs(c)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	ca, cert, key, err = GenerateServerCertificate(certpath, org, hosts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return
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

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "tls",
		Usage: "Utility for generating certificate for TLS",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "certpath", Value: DEFAULT_CERT_PATH, Usage: "Certificate path"},
			cli.StringFlag{Name: "organization", Value: DEFAULT_ORGANIZATION_PLACEMENT_NAME, Usage: "Organization for CA"},
		},
		Subcommands: []cli.Command{
			{
				Name:  "bootstrap",
				Usage: "Generate certificate for TLS",
				Action: func(c *cli.Context) {
					org, certpath, err := parseCertArgs(c)
					if err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					gencacert(c, org, certpath)
					gencert(c, org, certpath)
				},
			},
			{
				Name:  "generate",
				Usage: "Generate server certificate with self-signed CA",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) {
					_, cert, key := generateServerCertificate(c)
					if err := ioutil.WriteFile(cert.name, cert.buf.Bytes(), 0644); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					if err := ioutil.WriteFile(key.name, key.buf.Bytes(), 0600); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
				},
			},
			{
				Name:  "docker-engine-cert",
				Usage: "Generate and install certificate for Docker Enginea",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
					cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) {
					var (
						user    = c.String("user")
						privKey = c.String("cert")
						host    = c.String("host")

						ssh_config = ssh.Config{User: user, Server: host, Key: privKey, Port: "22"}
					)

					ca, cert, key := generateServerCertificate(c)
					fmt.Println(host, "- generated certificate")

					if err := SendEngineCertificate(ca, cert, key, ssh_config); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
				},
			},
		},
	}
}
