// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployWithVMTLSPreflight(t *testing.T) {
	management, err := GeneratePlanePKI(VMManagementCACommonName, VMManagementClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := GeneratePlanePKI(VMLogCACommonName, VMLogClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		plane string
		mode  string
		want  string
	}{
		{"incomplete management PKI", VMPlaneManagement, "missing", "incomplete"},
		{"incomplete log PKI", VMPlaneLog, "missing", "incomplete"},
		{"unsafe management key", VMPlaneManagement, "permissions", "unsafe permissions"},
		{"unsafe log key", VMPlaneLog, "permissions", "unsafe permissions"},
		{"valid existing PKI", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := privatePKIDir(t)
			writeTestPlane(t, VMManagementDir(base), management)
			writeTestPlane(t, VMLogDir(base), logs)
			keyPath := filepath.Join(VMPlaneDir(base, tc.plane), clientKeyFile)
			switch tc.mode {
			case "missing":
				if err := os.Remove(keyPath); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(keyPath, 0644); err != nil {
					t.Fatal(err)
				}
			}

			deploymentErr := errors.New("deployment failed after preflight")
			calls := 0
			// The callback encloses service changes and their failure cleanup.
			// None of those operations may run when the existing PKI is rejected.
			err := deployWithVMTLS(base, func() error {
				calls++
				return deploymentErr
			})
			if tc.want != "" {
				if calls != 0 {
					t.Errorf("entered installation/cleanup phase with invalid PKI %d times", calls)
				}
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want PKI validation error containing %q", err, tc.want)
				}
			} else if calls != 1 || !errors.Is(err, deploymentErr) {
				t.Fatalf("valid PKI: deployment calls = %d, error = %v", calls, err)
			}
		})
	}
}
