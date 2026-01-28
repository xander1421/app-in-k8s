package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// Server wraps HTTP/3, HTTP/2, and HTTP/1.1 servers
type Server struct {
	http3Server *http3.Server
	httpServer  *http.Server
	addr        string
	tlsConfig   *tls.Config
}

// NewServer creates a new multi-protocol server
func NewServer(addr string, handler http.Handler, tlsConfig *tls.Config) *Server {
	return &Server{
		addr:      addr,
		tlsConfig: tlsConfig,
		http3Server: &http3.Server{
			Addr:      addr,
			Handler:   handler,
			TLSConfig: tlsConfig,
			QUICConfig: &quic.Config{
				MaxIdleTimeout:  30 * time.Second,
				EnableDatagrams: true,
			},
		},
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			TLSConfig:    tlsConfig,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// ListenAndServe starts both HTTP/3 (QUIC) and HTTP/2 servers
func (s *Server) ListenAndServe() error {
	errChan := make(chan error, 2)

	go func() {
		log.Printf("Starting HTTP/3 server on %s (UDP/QUIC)", s.addr)
		if err := s.http3Server.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("http3: %w", err)
		}
	}()

	go func() {
		log.Printf("Starting HTTP/2 server on %s (TCP/TLS)", s.addr)
		s.httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Alt-Svc", fmt.Sprintf(`h3="%s"; ma=86400`, s.addr))
			s.http3Server.Handler.ServeHTTP(w, r)
		})
		if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("https: %w", err)
		}
	}()

	return <-errChan
}

// ListenAndServeInsecure starts without TLS (for dev/testing behind proxy)
func (s *Server) ListenAndServeInsecure() error {
	log.Printf("Starting HTTP/1.1 server on %s (no TLS - dev mode)", s.addr)
	s.httpServer.TLSConfig = nil
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down all servers
func (s *Server) Shutdown(ctx context.Context) error {
	var lastErr error
	if err := s.http3Server.Close(); err != nil {
		lastErr = err
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		lastErr = err
	}
	return lastErr
}

// GenerateSelfSignedCert creates a self-signed TLS cert for development
func GenerateSelfSignedCert() (*tls.Config, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Twitter Clone Dev"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
		DNSNames:              []string{"localhost", "*.twitter.svc.cluster.local"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	log.Println("Generated self-signed TLS certificate for HTTP/3")
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3", "h2", "http/1.1"},
	}, nil
}

// GetOutboundIP returns the preferred outbound IP of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "unknown"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
