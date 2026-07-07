// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"context"
	"os/exec"

	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isSystemdMode() bool {
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	return err == nil
}

func isKubernetesMode(c *k8s.Client) bool {
	if c == nil || c.K8sClientset == nil {
		return false
	}

	daemonsets, err := c.K8sClientset.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err == nil && len(daemonsets.Items) > 0 {
		return true
	}

	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err == nil && len(pods.Items) > 0 {
		return true
	}

	return false
}
