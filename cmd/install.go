// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var installOptions install.Options

// installCmd represents the get command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install KubeArmor in a Kubernetes Cluster",
	Long:  `Install KubeArmor in a Kubernetes Clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := install.K8sInstaller(client, installOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kube-system", "Namespace for resources")
}
