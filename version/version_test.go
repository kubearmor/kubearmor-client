// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package version

import (
	"testing"

	"github.com/kubearmor/kubearmor-client/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetKubeArmorVersionNoPods(t *testing.T) {
	c := &k8s.Client{K8sClientset: k8sfake.NewSimpleClientset()}

	version, err := getKubeArmorVersion(c)
	if err != nil {
		t.Fatalf("expected no error with no pods running, got: %v", err)
	}
	if version != "" {
		t.Errorf("expected an empty version with no pods running, got: %q", version)
	}
}

func TestGetKubeArmorVersionWithPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubearmor-abc123",
			Namespace: "kube-system",
			Labels:    map[string]string{"kubearmor-app": "kubearmor"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Image: "kubearmor/kubearmor:v1.4.0"},
			},
		},
	}
	c := &k8s.Client{K8sClientset: k8sfake.NewSimpleClientset(pod)}

	version, err := getKubeArmorVersion(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if version != "kubearmor/kubearmor:v1.4.0" {
		t.Errorf("expected the running kubearmor pod's image, got: %q", version)
	}
}
