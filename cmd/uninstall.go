// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"fmt"
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var uninstallOptions install.Options

// uninstallCmd represents the get command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall KubeArmor",
	Long:  `Uninstall KubeArmor  in either Kubernetes or non-Kubernetes mode.`,
}

var k8sUninstallCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Uninstall KubeArmor from a Kubernetes Cluster",
	Long:  `Uninstall KubeArmor from a Kubernetes Clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := install.K8sUninstaller(client, uninstallOptions); err != nil {
			if err := install.K8sLegacyUninstaller(client, uninstallOptions); err != nil {
				return err
			}
		}
		return nil
	},
}
var nonk8sUninstallCmd = &cobra.Command{
	Use:   "non-k8s",
	Short: "Uninstall KubeArmor from a non-Kubernetes mode",
	Long:  `Uninstall KubeArmor from a non-Kubernetes mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime := install.SelectRuntime(install.DetectRuntimes())
		if err := install.Uninstall(runtime); err != nil {
			return err
		}
		fmt.Println("KubeArmor uninstalled successfully.")
		return nil

	},
}

var systemdUninstallCmd = &cobra.Command{
	Use:   "systemd",
	Short: "Uninstall KubeArmor from systemd",
	Long:  `Uninstall KubeArmor from systemd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exist, err := install.KubearmorPresentAsSystemd()

		if err != nil {
			return fmt.Errorf("error checking systemd service: %w", err)
		}

		if exist {
			if err := install.Uninstall(install.SystemdRuntime); err != nil {
				return fmt.Errorf("failed to uninstall KubeArmor: %w", err)
			}
			fmt.Println("KubeArmor uninstalled successfully.")
		} else {
			fmt.Println("KubeArmor is not installed as a systemd service.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.AddCommand(k8sUninstallCmd)
	uninstallCmd.AddCommand(nonk8sUninstallCmd)
	uninstallCmd.AddCommand(systemdUninstallCmd)

	k8sUninstallCmd.Flags().StringVarP(&uninstallOptions.Namespace, "namespace", "n", "", "If no namespace is specified, it defaults to all namespaces and deletes all KubeArmor objects across them.")
	k8sUninstallCmd.Flags().BoolVar(&uninstallOptions.Force, "force", false, "Force remove KubeArmor annotations from deployments. (Deployments might be restarted)")
	k8sUninstallCmd.Flags().BoolVar(&uninstallOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are cleaned up or not")
}
