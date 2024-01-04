// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"fmt"

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
		if installOptions.Legacy {
			if err := install.K8sLegacyInstaller(client, installOptions); err != nil {
				return fmt.Errorf("error installing kubearmor in legacy mode: %v", err)
			}
		} else {
			if err := install.K8sInstaller(client, installOptions); err != nil {
				return fmt.Errorf("error installing kubearmor: %v", err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	installCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "kubearmor/kubearmor:stable", "Kubearmor daemonset image to use")
	installCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "kubearmor/kubearmor-init:stable", "Kubearmor daemonset init container image to use")
	installCmd.Flags().StringVarP(&installOptions.ControllerImage, "controller-image", "", "kubearmor/kubearmor-controller:latest", "Kubearmor controller image to use")
	installCmd.Flags().StringVarP(&installOptions.RelayImage, "relay-image", "", "kubearmor/kubearmor-relay-server:latest", "Kubearmor relay image to use")
	installCmd.Flags().StringVarP(&installOptions.Tag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	installCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Visibility, "viz", "", "", "Kubearmor Telemetry Visibility [process,file,network,none]")
	installCmd.Flags().BoolVar(&installOptions.Save, "save", false, "Save KubeArmor Manifest ")
	installCmd.Flags().BoolVar(&installOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are created, running and also probes whether KubeArmor has armored the cluster or not")
	installCmd.Flags().BoolVar(&installOptions.Local, "local", false, "Use Local KubeArmor Images (sets ImagePullPolicy to 'IfNotPresent') ")
	installCmd.Flags().StringVarP(&installOptions.ImageRegistry, "registry", "r", "", "Image registry to use to pull the images")
	installCmd.Flags().BoolVar(&installOptions.Legacy, "legacy", false, "Installs kubearmor in legacy mode if set to true")
}
