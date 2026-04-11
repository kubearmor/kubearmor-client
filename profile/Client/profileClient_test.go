// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package profileclient

import (
	"reflect"
	"testing"
)

func TestNormalizePolicyType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "network lowercase", input: "network", want: "network"},
		{name: "file uppercase", input: "FILE", want: "file"},
		{name: "process trimmed", input: "  process ", want: "process"},
		{name: "invalid", input: "syscall", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePolicyType(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizePolicyType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVisibleTabsForPolicyType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "all", input: "", want: []string{"Process", "File", "Network", "Syscall"}},
		{name: "network", input: "network", want: []string{"Network"}},
		{name: "file", input: "file", want: []string{"File"}},
		{name: "process", input: "process", want: []string{"Process"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleTabsForPolicyType(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("visibleTabsForPolicyType(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestOperationAllowed(t *testing.T) {
	tests := []struct {
		name       string
		policyType string
		operation  string
		want       bool
	}{
		{name: "all allowed", policyType: "", operation: "Network", want: true},
		{name: "matching type", policyType: "network", operation: "Network", want: true},
		{name: "case insensitive", policyType: "file", operation: "FILE", want: true},
		{name: "mismatch", policyType: "process", operation: "Network", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := operationAllowed(tt.policyType, tt.operation)
			if got != tt.want {
				t.Fatalf("operationAllowed(%q, %q) = %v, want %v", tt.policyType, tt.operation, got, tt.want)
			}
		})
	}
}
