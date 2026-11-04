package transportsecurity

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
)

type Config struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}

func LoadClientCredentials(cfg Config) (credentials.TransportCredentials, error) {
	if strings.TrimSpace(cfg.CAFile) == "" || strings.TrimSpace(cfg.CertFile) == "" || strings.TrimSpace(cfg.KeyFile) == "" || strings.TrimSpace(cfg.ServerName) == "" || strings.ContainsAny(cfg.ServerName, "* /\\\t\r\n") {
		return nil, errors.New("gRPC requires CA, gateway certificate, private key and server DNS name")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load gRPC gateway certificate: %w", err)
	}
	pem, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("load gRPC server CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("gRPC server CA contains no certificates")
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
		ServerName:   cfg.ServerName,
	}), nil
}
