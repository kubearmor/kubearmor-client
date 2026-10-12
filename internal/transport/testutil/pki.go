// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

// Package testutil provides independent PKI fixtures for TLS integration tests.
package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// PKI contains separate server and client identities signed by one test CA.
type PKI struct {
	Roots     *x509.CertPool
	Server    tls.Certificate
	Client    tls.Certificate
	ClientDir string
}

// NewPKI creates a fresh CA and client-only credential directory without ca.key.
func NewPKI(t testing.TB) PKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "test-ca-" + serial.String()},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	newIdentity := func(usage x509.ExtKeyUsage, serial int64) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
			DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	}
	result := PKI{
		Roots: x509.NewCertPool(), Server: newIdentity(x509.ExtKeyUsageServerAuth, 2),
		Client: newIdentity(x509.ExtKeyUsageClientAuth, 3), ClientDir: t.TempDir(),
	}
	result.Roots.AddCert(ca)
	if err := os.WriteFile(filepath.Join(result.ClientDir, "ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	WriteClientIdentity(t, result.ClientDir, result.Client)
	return result
}

// WriteClientIdentity replaces client.crt and client.key, leaving ca.crt intact.
func WriteClientIdentity(t testing.TB, directory string, client tls.Certificate) {
	t.Helper()
	key, err := x509.MarshalPKCS8PrivateKey(client.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	var certificates []byte
	for _, certificate := range client.Certificate {
		certificates = append(certificates, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})...)
	}
	for name, contents := range map[string][]byte{
		"client.crt": certificates,
		"client.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
