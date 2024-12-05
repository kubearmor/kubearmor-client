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
			if err := installOptions.Env.CheckAndSetValidEnvironmentOption(cmd.Flag("env").Value.String()); err != nil {
				return fmt.Errorf("error in checking environment option: %v", err)
			}
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

func markDeprecated(cmd *cobra.Command, flag, message string) {
	if err := cmd.Flags().MarkDeprecated(flag, message); err != nil {
		fmt.Printf("Error marking '%s' as deprecated: %v\n", flag, err)
	}
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	installCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "kubearmor/kubearmor:stable", "Kubearmor daemonset image to use")
	installCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "kubearmor/kubearmor-init:stable", "Kubearmor daemonset init container image to use")
	installCmd.Flags().StringVarP(&installOptions.OperatorImage, "operator-image", "", "kubearmor/kubearmor-operator:stable", "Kubearmor operator container image to use")
	installCmd.Flags().StringVarP(&installOptions.ControllerImage, "controller-image", "", "kubearmor/kubearmor-controller:stable", "Kubearmor controller image to use")
	installCmd.Flags().StringVarP(&installOptions.RelayImage, "relay-image", "", "kubearmor/kubearmor-relay-server:stable", "Kubearmor relay image to use")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorTag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorRelayTag, "relay-tag", "", "", "Change image tag/version for default kubearmor-relay image (This will overwrite the tag provided in --relay-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorControllerTag, "controller-tag", "", "", "Change image tag/version for default kubearmor-controller image (This will overwrite the tag provided in --controller-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorOperatorTag, "operator-tag", "", "", "Change image tag/version for default kubearmor-operator image (This will overwrite the tag provided in --operator-image)")
	installCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Visibility, "viz", "", "", "Kubearmor Telemetry Visibility [process,file,network,none]")
	installCmd.Flags().BoolVar(&installOptions.Save, "save", false, "Save KubeArmor Manifest ")
	installCmd.Flags().BoolVar(&installOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are created, running and also probes whether KubeArmor has armored the cluster or not")
	installCmd.Flags().BoolVar(&installOptions.Local, "local", false, "Use Local KubeArmor Images (sets ImagePullPolicy to 'IfNotPresent') ")
	installCmd.Flags().StringVarP(&installOptions.ImageRegistry, "registry", "r", "", "Image registry to use to pull the images")
	installCmd.Flags().BoolVar(&installOptions.Legacy, "legacy", false, "Installs kubearmor in legacy mode if set to true")
	installCmd.Flags().BoolVar(&installOptions.SkipDeploy, "skip-deploy", false, "Saves kubearmor operator CR manifest rather than deploying it")
	installCmd.Flags().BoolVar(&installOptions.PreserveUpstream, "preserve-upstream", true, "Do not override the image registry when using -r flag, prefix only")
	installCmd.Flags().StringVarP(&installOptions.Env.Environment, "env", "e", "", "Supported KubeArmor Environment [k0s,k3s,microK8s,minikube,gke,bottlerocket,eks,docker,oke,generic]")
	installCmd.MarkFlagsMutuallyExclusive("verify", "save")
	installCmd.Flags().BoolVar(&installOptions.AlertThrottling, "alertThrottling", true, "Enable/Disable Alert Throttling, by default it's enabled")
	installCmd.Flags().Int32Var(&installOptions.MaxAlertPerSec, "maxAlertPerSec", 10, "Maximum number of alerts required to trigger alert throttling")
	installCmd.Flags().Int32Var(&installOptions.ThrottleSec, "throttleSec", 30, "Time window(in sec) for which there will be no alerts genrated after alert throttling is triggered")

	markDeprecated(installCmd, "env", "Only relevant when using legacy")
	markDeprecated(installCmd, "legacy", "KubeArmor now utilizes operator-based installation. This command may not set up KubeArmor in the intended way.")
}
