// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package cmd

import (
	"fmt"
    "github.com/spf13/cobra"
	"github.com/kubearmor/kubearmor-client/install"
)

var secureRuntime string
var installOptions install.Options

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install KubeArmor",
	Long:  `Install KubeArmor in either Kubernetes or non-Kubernetes mode.`,
}

// k8sInstallCmd represents the install command for Kubernetes mode
var k8sInstallCmd = &cobra.Command{
	Use:   "k8s",
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

// nonK8sInstallCmd represents the install command for non-Kubernetes mode
var nonK8sInstallCmd = &cobra.Command{
	Use:   "non-k8s",
	Short: "Install KubeArmor in non-Kubernetes mode",
	Long: "Install KubeArmor in non-Kubernetes mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		availableRuntimes := install.DetectRuntimes()
		if len(availableRuntimes) == 0 {
			return fmt.Errorf("no supported container runtime found")
		}
		runtime := install.SelectRuntime(secureRuntime, availableRuntimes)

		composeFilePath, err := install.EnsureComposeFile()
		if err != nil {
			return fmt.Errorf("failed to ensure Compose file: %v", err)
		}
      
		err = install.ParseAndValidateComposeFile(composeFilePath, runtime)
		if err != nil {
			return fmt.Errorf("error validating Compose file: %v", err)
		}
		err = install.RunCompose(runtime, composeFilePath)
		if err != nil {
			return fmt.Errorf("error running Compose file: %v", err)
		}
		fmt.Println("😄 KubeArmor installed successfully in non-Kubernetes mode.")
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
	// Add subcommands for k8s and non-k8s modes
	installCmd.AddCommand(k8sInstallCmd)
	installCmd.AddCommand(nonK8sInstallCmd)

    //these flags should only be availabe only for mode k8s
	k8sInstallCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	k8sInstallCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "kubearmor/kubearmor:stable", "Kubearmor daemonset image to use")
	k8sInstallCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "kubearmor/kubearmor-init:stable", "Kubearmor daemonset init container image to use")
	k8sInstallCmd.Flags().StringVarP(&installOptions.OperatorImage, "operator-image", "", "kubearmor/kubearmor-operator:stable", "Kubearmor operator container image to use")
	k8sInstallCmd.Flags().StringVarP(&installOptions.ControllerImage, "controller-image", "", "kubearmor/kubearmor-controller:stable", "Kubearmor controller image to use")
	k8sInstallCmd.Flags().StringVarP(&installOptions.RelayImage, "relay-image", "", "kubearmor/kubearmor-relay-server:stable", "Kubearmor relay image to use")
	k8sInstallCmd.Flags().StringVarP(&installOptions.KubeArmorTag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	k8sInstallCmd.Flags().StringVarP(&installOptions.KubeArmorRelayTag, "relay-tag", "", "", "Change image tag/version for default kubearmor-relay image (This will overwrite the tag provided in --relay-image)")
	k8sInstallCmd.Flags().StringVarP(&installOptions.KubeArmorControllerTag, "controller-tag", "", "", "Change image tag/version for default kubearmor-controller image (This will overwrite the tag provided in --controller-image)")
	k8sInstallCmd.Flags().StringVarP(&installOptions.KubeArmorOperatorTag, "operator-tag", "", "", "Change image tag/version for default kubearmor-operator image (This will overwrite the tag provided in --operator-image)")
	k8sInstallCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	k8sInstallCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	k8sInstallCmd.Flags().StringVarP(&installOptions.Visibility, "viz", "", "", "Kubearmor Telemetry Visibility [process,file,network,none]")
	k8sInstallCmd.Flags().BoolVar(&installOptions.Save, "save", false, "Save KubeArmor Manifest ")
	k8sInstallCmd.Flags().BoolVar(&installOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are created, running and also probes whether KubeArmor has armored the cluster or not")
	k8sInstallCmd.Flags().BoolVar(&installOptions.Local, "local", false, "Use Local KubeArmor Images (sets ImagePullPolicy to 'IfNotPresent') ")
	k8sInstallCmd.Flags().StringVarP(&installOptions.ImageRegistry, "registry", "r", "", "Image registry to use to pull the images")
	k8sInstallCmd.Flags().BoolVar(&installOptions.Legacy, "legacy", false, "Installs kubearmor in legacy mode if set to true")
	k8sInstallCmd.Flags().BoolVar(&installOptions.SkipDeploy, "skip-deploy", false, "Saves kubearmor operator CR manifest rather than deploying it")
	k8sInstallCmd.Flags().BoolVar(&installOptions.PreserveUpstream, "preserve-upstream", true, "Do not override the image registry when using -r flag, prefix only")
	k8sInstallCmd.Flags().StringVarP(&installOptions.Env.Environment, "env", "e", "", "Supported KubeArmor Environment [k0s,k3s,microK8s,minikube,gke,bottlerocket,eks,docker,oke,generic]")
	k8sInstallCmd.MarkFlagsMutuallyExclusive("verify", "save")
	markDeprecated(k8sInstallCmd, "env", "Only relevant when using legacy")
	markDeprecated(k8sInstallCmd, "legacy", "KubeArmor now utilizes operator-based installation. This command may not set up KubeArmor in the intended way.")
	//this flag --secure should only be availabe only for mode non-k8s
    nonK8sInstallCmd.Flags().StringVar(&secureRuntime, "secure", "", "Specify the container runtime (e.g., podman, docker)")
}

