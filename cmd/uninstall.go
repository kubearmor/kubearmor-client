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
	Short: "Uninstall KubeArmor",
	Long:  `Uninstall KubeArmor  in either Kubernetes or non-Kubernetes mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// check for systemd or docker installation
		exist := install.CheckAndRemoveKAVmInstallation()
		if !exist {
			if err := install.K8sUninstaller(client, uninstallOptions); err != nil {
				if err := install.K8sLegacyUninstaller(client, uninstallOptions); err != nil {
					return err
				}
			}
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)

	uninstallCmd.Flags().StringVarP(&uninstallOptions.Namespace, "namespace", "n", "", "If no namespace is specified, it defaults to all namespaces and deletes all KubeArmor objects across them.")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.Force, "force", false, "Force remove KubeArmor annotations from deployments. (Deployments might be restarted)")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are cleaned up or not")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.NonK8s, "nonk8s", false, "uninstall docker or systemd KubeArmor")
}
