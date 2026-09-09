// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package vm

import (
	"os"

	"github.com/kubearmor/KubeArmor/KubeArmor/cert"
	"github.com/kubearmor/kubearmor-client/install"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DefaultManagementTLSCertPath is the default management trust-plane
// directory (management/ca.crt + management/client.crt/client.key).
// It mirrors the server default {tlsCertPath}/management so karmor and a
// local KubeArmor agent agree out of the box.
const DefaultManagementTLSCertPath = "/var/lib/kubearmor/tls/management"

// DefaultManagementGRPCAddress is the default management-plane endpoint.
// PolicyService/ProbeService live on the management server (32765), not on
// the observability log server (32767).
const DefaultManagementGRPCAddress = "localhost:32765"

// ManagementTLSCertPath resolves the management plane directory:
// explicit flag value wins, then KUBEARMOR_MANAGEMENT_TLS_CERT_PATH,
// then the compiled-in default.
func ManagementTLSCertPath(configured string) string {
	if configured != "" {
		return configured
	}
	if val, ok := os.LookupEnv("KUBEARMOR_MANAGEMENT_TLS_CERT_PATH"); ok && val != "" {
		return val
	}
	return DefaultManagementTLSCertPath
}

// ManagementPlaneDir maps a split-PKI base directory
// (<base>/management, <base>/log) to its management plane directory.
func ManagementPlaneDir(baseDir string) string {
	return install.VMManagementDir(baseDir)
}

// ManagementGRPCAddress resolves the management-plane endpoint:
// explicit flag value wins, then KUBEARMOR_MANAGEMENT_SERVICE,
// then the compiled-in default.
func ManagementGRPCAddress(configured string) string {
	if configured != "" {
		return configured
	}
	if val, ok := os.LookupEnv("KUBEARMOR_MANAGEMENT_SERVICE"); ok && val != "" {
		return val
	}
	return DefaultManagementGRPCAddress
}

// ManagementClientCredentials builds mutual-TLS client credentials from the
// management plane's persisted material (ca.crt + client.crt/client.key).
// The management CA is the only trust root, so log-plane identities can
// never authenticate here and vice versa.
func ManagementClientCredentials(mgmtDir string) (credentials.TransportCredentials, error) {
	tlsConfig := cert.TlsConfig{
		CertProvider: cert.ExternalCertProvider,
		CACertPath:   cert.GetCACertPath(mgmtDir),
		CertPath:     cert.GetClientCertPath(mgmtDir),
	}
	return cert.NewTlsCredentialManager(&tlsConfig).CreateTlsClientCredentials()
}

// NewManagementGRPCClient dials the management plane with mutual TLS.
func NewManagementGRPCClient(address, mgmtDir string) (*grpc.ClientConn, error) {
	creds, err := ManagementClientCredentials(mgmtDir)
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(address, grpc.WithTransportCredentials(creds))
}
