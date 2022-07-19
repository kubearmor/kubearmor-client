// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package version

import (
	"context"
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/selfupdate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrintVersion handler for karmor version
func PrintVersion(c *k8s.Client) error {
	fmt.Printf("karmor version %s %s/%s BuildDate=%s\n", selfupdate.GitSummary, runtime.GOOS, runtime.GOARCH, selfupdate.BuildDate)
	latest, latestVer := selfupdate.IsLatest(selfupdate.GitSummary)
	if !latest {
		color.HiMagenta("update available version " + latestVer)
		color.HiMagenta("use [karmor selfupdate] to update to latest")
	}
	kubearmorVersion, err := getKubeArmorVersion(c)
	if err != nil {
		return nil
	}
	if kubearmorVersion == "" {
		fmt.Printf("kubearmor not running\n")
		return nil
	}
	fmt.Printf("kubearmor image (running) version %s\n", kubearmorVersion)
	return nil
}

func getKubeArmorVersion(c *k8s.Client) (string, error) {
	pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{LabelSelector: "kubearmor-app=kubearmor"})
	if err != nil {
		return "", err
	}
	if len(pods.Items) > 0 {
		image := pods.Items[0].Spec.Containers[0].Image
		return image, nil
	}
	return "", nil
}
