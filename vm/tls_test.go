// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kubearmor/kubearmor-client/install"
)

func TestManagementTLSCertPathResolution(t *testing.T) {
	if got := ManagementTLSCertPath("/custom/mgmt"); got != "/custom/mgmt" {
		t.Errorf("explicit path = %q, want /custom/mgmt", got)
	}
	t.Setenv("KUBEARMOR_MANAGEMENT_TLS_CERT_PATH", "/env/mgmt")
	if got := ManagementTLSCertPath(""); got != "/env/mgmt" {
		t.Errorf("env path = %q, want /env/mgmt", got)
	}
	os.Unsetenv("KUBEARMOR_MANAGEMENT_TLS_CERT_PATH")
	if got := ManagementTLSCertPath(""); got != DefaultManagementTLSCertPath {
		t.Errorf("default path = %q, want %q", got, DefaultManagementTLSCertPath)
	}
}

func TestManagementGRPCAddressResolution(t *testing.T) {
	if got := ManagementGRPCAddress("host:1"); got != "host:1" {
		t.Errorf("explicit addr = %q, want host:1", got)
	}
	t.Setenv("KUBEARMOR_MANAGEMENT_SERVICE", "host:2")
	if got := ManagementGRPCAddress(""); got != "host:2" {
		t.Errorf("env addr = %q, want host:2", got)
	}
	os.Unsetenv("KUBEARMOR_MANAGEMENT_SERVICE")
	if got := ManagementGRPCAddress(""); got != DefaultManagementGRPCAddress {
		t.Errorf("default addr = %q, want %q", got, DefaultManagementGRPCAddress)
	}
	if got := ManagementGRPCAddress(""); got == "localhost:32767" {
		t.Error("management default must not be the observability port 32767")
	}
}

func TestManagementClientCredentialsUsesManagementPlane(t *testing.T) {
	base := t.TempDir()
	if err := install.EnsureVMPlanePKI(base); err != nil {
		t.Fatal(err)
	}
	mgmtDir := install.VMManagementDir(base)

	creds, err := ManagementClientCredentials(mgmtDir)
	if err != nil {
		t.Fatalf("ManagementClientCredentials failed on management plane dir: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil transport credentials")
	}

	// A missing plane directory must fail fast instead of silently
	// falling back to another trust domain.
	if _, err := ManagementClientCredentials(filepath.Join(base, "does-not-exist")); err == nil {
		t.Error("expected error for missing management plane dir, got nil")
	}
}
