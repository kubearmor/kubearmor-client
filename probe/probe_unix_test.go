//go:build darwin || (linux && !windows)

package probe

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckLsmSupport(t *testing.T) {
	cases := []struct {
		lsm  string
		want string
	}{
		{"capability,yama,apparmor", "Full"},
		{"capability,yama,bpf", "Full"},
		{"capability,yama,selinux,bpf", "Full"},
		{"capability,yama,selinux", "Partial"},
		{"capability,yama", "None"},
	}

	for _, c := range cases {
		buf := &bytes.Buffer{}
		checkLsmSupport(c.lsm, Options{Output: "no-color", Writer: buf})
		if !strings.Contains(buf.String(), c.want) {
			t.Errorf("checkLsmSupport(%q) output = %q, want it to contain %q", c.lsm, buf.String(), c.want)
		}
	}
}
