// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLogCertPathEmpty(t *testing.T) {
	if got := ResolveLogCertPath(""); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}
}

func TestResolveLogCertPathSingleCA(t *testing.T) {
	base := t.TempDir()
	// Legacy single-CA layout: ca.crt directly under base, no log/ subdir.
	if err := os.WriteFile(filepath.Join(base, "ca.crt"), []byte("dummy"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := ResolveLogCertPath(base); got != base {
		t.Errorf("single-CA layout resolved to %q, want %q", got, base)
	}
}

func TestResolveLogCertPathSplitPKI(t *testing.T) {
	base := t.TempDir()
	logDir := filepath.Join(base, LogPlaneSubdir)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, LogPlaneCACertFile), []byte("dummy"), 0600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, LogPlaneSubdir)
	if got := ResolveLogCertPath(base); got != want {
		t.Errorf("split-PKI layout resolved to %q, want %q", got, want)
	}
}
