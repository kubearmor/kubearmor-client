package utils

import "os/exec"

// IsSystemdMode checks if kubearmor is running in systemd mode
func IsSystemdMode() bool {
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	return err == nil
}
