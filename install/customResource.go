// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

import (
	kcrd "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/crd"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var kspName = "kubearmorpolicies.security.kubearmor.com"
var hspName = "kubearmorhostpolicies.security.kubearmor.com"

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes.
func CreateCustomResourceDefinition(crdName string) apiextensions.CustomResourceDefinition {
	var crd apiextensions.CustomResourceDefinition
	switch crdName {
	case kspName:
		crd = kcrd.GetKspCRD()
	case hspName:
		crd = kcrd.GetHspCRD()
	}

	return crd
}
