// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package log

import (
	"os"
	"path/filepath"
)

// LogPlaneSubdir is the log/observability trust-plane subdirectory under a
// split-PKI base directory (<base>/log/ca.crt, ...).
const LogPlaneSubdir = "log"

// LogPlaneCACertFile is the file probed to detect a split-PKI layout.
const LogPlaneCACertFile = "ca.crt"

// ResolveLogCertPath maps a configured TLS base to the log trust plane:
// when <base>/log/ca.crt exists (split PKI from karmor unorchestrated
// setup), the log plane directory is used so log commands authenticate
// with the Log CA + log client identity; otherwise the configured path is
// returned unchanged for backward compatibility with single-CA layouts.
func ResolveLogCertPath(configured string) string {
	if configured == "" {
		return configured
	}
	if _, err := os.Stat(filepath.Join(configured, LogPlaneSubdir, LogPlaneCACertFile)); err == nil {
		return filepath.Join(configured, LogPlaneSubdir)
	}
	return configured
}
