// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var uninstallOptions install.Options

// uninstallCmd represents the get command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall KubeArmor from a Kubernetes Cluster",
	Long:  `Uninstall KubeArmor from a Kubernetes Clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := install.K8sUninstaller(client, uninstallOptions)
		return err
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)

	uninstallCmd.Flags().StringVarP(&uninstallOptions.Namespace, "namespace", "n", "kube-system", "Namespace for resources")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.Force, "force", false, "Force remove KubeArmor annotations from deployments. (Deployments might be restarted)")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are cleaned up or not")
}
