package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func generateTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}
	certOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("encode key: %v", err)
	}
	keyOut.Close()

	return certFile, keyFile
}

func TestBuild_Disabled(t *testing.T) {
	cfg := &TLSConfig{Enabled: false}
	got, err := cfg.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil config, got %+v", got)
	}
}

func TestBuild_MissingCertFile(t *testing.T) {
	cfg := &TLSConfig{
		Enabled: true,
		KeyFile: "/some/key.pem",
	}
	_, err := cfg.Build()
	if err == nil {
		t.Fatal("expected error for missing cert_file")
	}
	if !strings.Contains(err.Error(), "cert_file or key_file missing") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuild_MissingKeyFile(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:  true,
		CertFile: "/some/cert.pem",
	}
	_, err := cfg.Build()
	if err == nil {
		t.Fatal("expected error for missing key_file")
	}
	if !strings.Contains(err.Error(), "cert_file or key_file missing") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuild_NonExistentFiles(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}
	_, err := cfg.Build()
	if err == nil {
		t.Fatal("expected error for non-existent files")
	}
	if !strings.Contains(err.Error(), "load tls keypair") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuild_ValidCert(t *testing.T) {
	certFile, keyFile := generateTestCert(t)
	cfg := &TLSConfig{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	got, err := cfg.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if len(got.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(got.Certificates))
	}
}

func TestBuild_MinVersionTLS13(t *testing.T) {
	certFile, keyFile := generateTestCert(t)
	cfg := &TLSConfig{
		Enabled:    true,
		CertFile:   certFile,
		KeyFile:    keyFile,
		MinVersion: "1.3",
	}
	got, err := cfg.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.MinVersion != tls.VersionTLS13 {
		t.Fatalf("expected MinVersion %d (TLS 1.3), got %d", tls.VersionTLS13, got.MinVersion)
	}
}

func TestBuild_MinVersionDefaultTLS12(t *testing.T) {
	certFile, keyFile := generateTestCert(t)
	cfg := &TLSConfig{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	got, err := cfg.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected MinVersion %d (TLS 1.2), got %d", tls.VersionTLS12, got.MinVersion)
	}
}
