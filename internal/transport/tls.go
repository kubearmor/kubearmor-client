// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

// Package transport provides TLS credentials and management endpoint settings.
package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// DefaultManagementTLSCertPath is the agent's management credential directory.
	DefaultManagementTLSCertPath = "/var/lib/kubearmor/tls/management"
	// DefaultManagementGRPCAddress is the standalone management endpoint.
	DefaultManagementGRPCAddress = "localhost:32765"
	// RPCTimeout bounds standalone policy and probe requests.
	RPCTimeout = 10 * time.Second
)

// ManagementTLSCertPath resolves an explicit directory, environment, or default.
func ManagementTLSCertPath(configured string) string {
	if configured != "" {
		return configured
	}
	if directory := os.Getenv("KUBEARMOR_MANAGEMENT_TLS_CERT_PATH"); directory != "" {
		return directory
	}
	return DefaultManagementTLSCertPath
}

// ManagementGRPCAddress resolves an explicit endpoint, environment, or default.
func ManagementGRPCAddress(configured string) string {
	if configured != "" {
		return configured
	}
	if address := os.Getenv("KUBEARMOR_MANAGEMENT_SERVICE"); address != "" {
		return address
	}
	return DefaultManagementGRPCAddress
}

// ClientCredentials loads a persisted client identity without a CA private key.
// gRPC supplies the selected endpoint's hostname for server verification.
func ClientCredentials(directory string) (credentials.TransportCredentials, error) {
	caPath := filepath.Join(directory, "ca.crt")
	// #nosec G304 -- the user explicitly selects the TLS credential directory.
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read client trust roots: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no CA certificates found in %s", caPath)
	}
	certificate, err := tls.LoadX509KeyPair(filepath.Join(directory, "client.crt"), filepath.Join(directory, "client.key"))
	if err != nil {
		return nil, fmt.Errorf("load client identity from %s: %w", directory, err)
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
	}), nil
}

// NewManagementGRPCClient creates an authenticated management connection.
func NewManagementGRPCClient(address, directory string) (*grpc.ClientConn, error) {
	creds, err := ClientCredentials(directory)
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(address, grpc.WithTransportCredentials(creds))
}
