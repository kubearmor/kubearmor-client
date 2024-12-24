// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

import (
	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/crd"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var kspName = "kubearmorpolicies.security.kubearmor.com"
var hspName = "kubearmorhostpolicies.security.kubearmor.com"
var cspName = "kubearmorclusterpolicies.security.kubearmor.com"
var kocName = "kubearmorconfigs.operator.kubearmor.com"

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes.
func CreateCustomResourceDefinition(crdName string) apiextensions.CustomResourceDefinition {
	var crd apiextensions.CustomResourceDefinition
	switch crdName {
	case kspName:
		crd = ksp.GetKspCRD()
	case hspName:
		crd = ksp.GetHspCRD()
	}

	return crd
}
