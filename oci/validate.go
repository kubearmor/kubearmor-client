// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package oci

import (
	"github.com/kubearmor/kubearmor-client/oci/data"
	"sigs.k8s.io/kubectl-validate/pkg/cmd"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
	"sigs.k8s.io/kubectl-validate/pkg/validatorfactory"
)

// ValidatePolicies checks o.Files using kubectl-validate. It returns
// boolean value indicating success or failure and message if
// validation failed.
func (o *OCIRegistry) ValidatePolicies() (string, bool, error) {
	factory, err := validatorfactory.New(
		openapiclient.NewOverlay(
			openapiclient.PatchLoaderFromDirectory(nil, ""),
			openapiclient.NewComposite(
				openapiclient.NewLocalCRDFiles(data.KubeArmorCRDs, "KubeArmorCRDs"),
			),
		),
	)
	if err != nil {
		return "", false, err
	}
	for _, path := range o.Files {
		var msg string
		for _, err := range cmd.ValidateFile(path, factory) {
			if err != nil {
				msg = msg + err.Error()
			}
		}
		if msg != "" {
			return msg, false, nil
		}
	}
	return "", true, nil
}
