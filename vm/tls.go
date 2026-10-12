// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package vm

import (
	"github.com/kubearmor/kubearmor-client/internal/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DefaultManagementTLSCertPath is the default management trust-plane
// directory (management/ca.crt + management/client.crt/client.key).
// It mirrors the agent's managementTLSCertPath default.
const DefaultManagementTLSCertPath = transport.DefaultManagementTLSCertPath

// DefaultManagementGRPCAddress is the default management-plane endpoint.
// PolicyService/ProbeService live on the management server (32765), not on
// the observability log server (32767).
const DefaultManagementGRPCAddress = transport.DefaultManagementGRPCAddress

// ManagementTLSCertPath resolves the management plane directory:
// explicit flag value wins, then KUBEARMOR_MANAGEMENT_TLS_CERT_PATH,
// then the compiled-in default.
func ManagementTLSCertPath(configured string) string {
	return transport.ManagementTLSCertPath(configured)
}

// ManagementGRPCAddress resolves the management-plane endpoint:
// explicit flag value wins, then KUBEARMOR_MANAGEMENT_SERVICE,
// then the compiled-in default.
func ManagementGRPCAddress(configured string) string {
	return transport.ManagementGRPCAddress(configured)
}

// ManagementClientCredentials builds mutual-TLS client credentials from the
// management plane's persisted material (ca.crt + client.crt/client.key).
// The configured management directory supplies the client trust roots.
func ManagementClientCredentials(mgmtDir string) (credentials.TransportCredentials, error) {
	return transport.ClientCredentials(mgmtDir)
}

// NewManagementGRPCClient dials the management plane with mutual TLS.
func NewManagementGRPCClient(address, mgmtDir string) (*grpc.ClientConn, error) {
	return transport.NewManagementGRPCClient(address, mgmtDir)
}
