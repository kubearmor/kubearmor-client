// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package get

import (
	"context"
	"fmt"

	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	Namespace string
}

func Resources(c *k8s.Client, o Options) error {
	kspInterface := c.KSPClientset.KubeArmorPolicies(o.Namespace)
	policies, err := kspInterface.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(policies.Items) > 0 {
		fmt.Printf("Resources found in %s namespace: \n", o.Namespace)
		for _, policy := range policies.Items {
			fmt.Printf("%v \n", policy.Name)
		}
	} else {
		fmt.Printf("No Resource found in %s namespace", o.Namespace)
	}
	return nil
}
