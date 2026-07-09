// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	now := time.Now().UTC().Format(time.RFC3339)
	content := `{"Operation":"Process","ProcessName":"/bin/bash","PodName":"api-74x","NamespaceName":"default","PID":1482,"ContainerName":"api","UpdatedTime":"` + now + `"}
{"Operation":"Process","ProcessName":"/usr/bin/curl","PodName":"api-74x","NamespaceName":"default","PID":3012,"UpdatedTime":"` + now + `"}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := collectFromFile(CollectOptions{
		EventsFile: path,
		Namespace:  "default",
		Last:       time.Hour,
	})
	if err != nil {
		t.Fatalf("collectFromFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events))
	}
}

func TestRun_offlineEndToEnd(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	eventsPath := filepath.Join(dir, "events.jsonl")
	now := time.Now().UTC().Format(time.RFC3339)

	policyYAML := `apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-shell-access
spec:
  process:
    matchPaths:
      - path: /bin/bash
      - path: /bin/sh
  action: Block
`
	eventsJSONL := `{"Operation":"Process","ProcessName":"/bin/bash","PodName":"api-74x","NamespaceName":"default","PID":1482,"ContainerName":"api","UpdatedTime":"` + now + `"}
{"Operation":"Process","ProcessName":"/usr/bin/curl","PodName":"api-74x","NamespaceName":"default","PID":3012,"ContainerName":"api","UpdatedTime":"` + now + `"}
`
	if err := os.WriteFile(policyPath, []byte(policyYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eventsPath, []byte(eventsJSONL), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Run(nil, Options{
		PolicyFile: policyPath,
		EventsFile: eventsPath,
		Namespace:  "default",
		Last:       "1h",
		Output:     "json",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}
