// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package k8s

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"golang.org/x/mod/semver"
)

// AutoDetectEnvironment detect the environment for a given k8s context
func AutoDetectEnvironment(c *Client) (name string) {
	env := "none"

	contextName := c.RawConfig.CurrentContext
	clusterContext, exists := c.RawConfig.Contexts[contextName]
	if !exists {
		return env
	}

	clusterName := clusterContext.Cluster
	cluster := c.RawConfig.Clusters[clusterName]
	nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if len(nodes.Items) <= 0 {
		return env
	}
	containerRuntime := nodes.Items[0].Status.NodeInfo.ContainerRuntimeVersion
	nodeImage := nodes.Items[0].Status.NodeInfo.OSImage
	serverInfo, _ := c.K8sClientset.Discovery().ServerVersion()

	// Detecting Environment based on cluster name and context or OSImage
	if clusterName == "minikube" || contextName == "minikube" {
		env = "minikube"
		return env
	}

	if strings.HasPrefix(clusterName, "microk8s-") || contextName == "microk8s" {
		env = "microk8s"
		return env
	}

	if strings.HasPrefix(clusterName, "gke_") {
		env = "gke"
		return env
	}

	if strings.Contains(nodeImage, "Bottlerocket") {
		env = "bottlerocket"
		return env
	}

	if strings.HasSuffix(clusterName, ".eksctl.io") || strings.HasSuffix(cluster.Server, "eks.amazonaws.com") {
		env = "eks"
		return env
	}

	// Environment is Self Managed K8s, checking container runtime, it's version and k8s server version
	if strings.HasSuffix(serverInfo.String(), "k0s") {
		env = "k0s"
		return env
	}

	if strings.Contains(containerRuntime, "k3s") {
		env = "k3s"
		return env
	}

	s := strings.Split(containerRuntime, "://")
	runtime := s[0]
	version := "v" + s[1]

	if runtime == "docker" && semver.Compare(version, "v18.9") >= 0 {
		env = "docker"
		return env
	}

	if runtime == "cri-o" {
		env = "oke"
		return env
	}

	if (runtime == "docker" && semver.Compare(version, "v19.3") >= 0) || runtime == "containerd" {
		env = "generic"
		return env
	}

	return env
}
