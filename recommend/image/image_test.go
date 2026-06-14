// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package image

import (
	"testing"
)

func TestMkPathFromTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Issue #547: underscores in tag produce invalid RFC1123 names
			name:  "underscores in tag are replaced with hyphens",
			input: "specifyconsortium/specify7-service:v7_12_0_7_base",
			want:  "specifyconsortium-specify7-service-v7-12-0-7-base",
		},
		{
			name:  "underscore only in tag",
			input: "myimage:v1_0",
			want:  "myimage-v1-0",
		},
		{
			name:  "underscore in repo name",
			input: "my_org/myimage:latest",
			want:  "my-org-myimage-latest",
		},
		{
			// Regression: existing characters must still be replaced
			name:  "colon and slash are replaced (regression)",
			input: "ubuntu:18.04",
			want:  "ubuntu-18-04",
		},
		{
			name:  "dots in registry and tag are replaced (regression)",
			input: "registry.example.com/myimage:1.2.3",
			want:  "registry-example-com-myimage-1-2-3",
		},
		{
			name:  "at-sign in digest is replaced (regression)",
			input: "myimage@sha256:abc123",
			want:  "myimage-sha256-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mkPathFromTag(tt.input)
			if got != tt.want {
				t.Errorf("mkPathFromTag(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}
