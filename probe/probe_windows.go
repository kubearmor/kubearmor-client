package probe

import "github.com/kubearmor/kubearmor-client/k8s"

func printProbeResult(c *k8s.Client, o Options) error {
	isRunning, daemonsetStatus := isKubeArmorRunning(c)
	if isRunning {
		return printWhenKubeArmorIsRunningInK8s(c, o, daemonsetStatus)
	} else {
		return ErrKubeArmorNotRunningOnK8s
	}
}
