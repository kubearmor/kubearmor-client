// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := `apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-shell-access
spec:
  selector:
    matchLabels:
      app: myapp
  process:
    matchPaths:
      - path: /bin/bash
  action: Block
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if policy.Metadata.Name != "block-shell-access" {
		t.Fatalf("name = %q, want block-shell-access", policy.Metadata.Name)
	}
	if policy.Spec.Action != "Block" {
		t.Fatalf("action = %q, want Block", policy.Spec.Action)
	}
}

func TestLoadPolicy_rejectsHostPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host.yaml")
	content := `apiVersion: security.kubearmor.com/v1
kind: KubeArmorHostPolicy
metadata:
  name: host-policy
spec:
  action: Block
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPolicy(path)
	if err == nil {
		t.Fatal("expected error for host policy")
	}
}
