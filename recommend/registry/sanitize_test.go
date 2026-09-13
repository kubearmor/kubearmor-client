// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package registry

import "testing"

func TestSanitizeArchivePath(t *testing.T) {
	dir := "/tmp/karmor2847362918"

	if _, err := sanitizeArchivePath(dir, "file.txt"); err != nil {
		t.Errorf("expected a plain file under dir to be allowed, got error: %v", err)
	}

	if _, err := sanitizeArchivePath(dir, "sub/file.txt"); err != nil {
		t.Errorf("expected a nested file under dir to be allowed, got error: %v", err)
	}

	if _, err := sanitizeArchivePath(dir, "../evil.txt"); err == nil {
		t.Error("expected a path escaping dir via .. to be rejected")
	}

	if _, err := sanitizeArchivePath(dir, "../karmor2847362918-evil/file.txt"); err == nil {
		t.Error("expected a sibling directory sharing dir's prefix to be rejected")
	}
}
