// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"testing"
)

func TestDetectDeploymentModeFallback(t *testing.T) {
	mode := DetectDeploymentMode(nil)
	if mode == ModeUnknown {
		t.Log("Detected Mode: Unknown (expected for nil client)")
	}
}

func TestCollectorFactory(t *testing.T) {
	factory := NewCollectorFactory(nil, Options{})

	tests := []struct {
		name string
		mode DeploymentMode
	}{
		{"Kubernetes Collector", ModeKubernetes},
		{"Systemd Collector", ModeSystemd},
		{"Process Collector", ModeProcess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := factory.NewCollector(tt.mode)
			if collector == nil {
				t.Fatalf("Expected collector, got nil for mode %v", tt.mode)
			}
		})
	}
}

func TestIsDirEmpty(t *testing.T) {
	tmpdir := t.TempDir()

	empty, err := IsDirEmpty(tmpdir)
	if err != nil {
		t.Fatalf("IsDirEmpty failed: %v", err)
	}
	if !empty {
		t.Fatal("Expected empty directory")
	}

	if err := writeToFile(tmpdir+"/test.txt", "test"); err != nil {
		t.Fatalf("writeToFile failed: %v", err)
	}

	empty, err = IsDirEmpty(tmpdir)
	if err != nil {
		t.Fatalf("IsDirEmpty failed: %v", err)
	}
	if empty {
		t.Fatal("Expected non-empty directory")
	}
}
