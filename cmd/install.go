// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var secureRuntime string
var installOptions install.Options
var configOptions install.Config
var composeConfigOptions install.ComposeConfig
var releaseVersion string

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
	Long:  "Install KubeArmor in non-Kubernetes mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		availableRuntimes := install.DetectRuntimes()
		if len(availableRuntimes) == 0 {
			return fmt.Errorf("no supported container runtime found")
		}
		runtime := install.SelectRuntime(availableRuntimes, secureRuntime)

		// err := install.EnsureComposeFile()
		// if err != nil {
		// 	return fmt.Errorf("failed to ensure Compose file: %v", err)
		// }
		err := install.UpdateConfigCommandInComposeFile(composeConfigOptions)
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
var systemdInstallCmd = &cobra.Command{
	Use:   "systemd",
	Short: "Install KubeArmor using systemd",
	Long:  `Install KubeArmor using systemd mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exist, err := install.KubearmorPresentAsSystemd()
		if err != nil {
			return fmt.Errorf("error checking systemd service: %w", err)
		}
		if exist {
			fmt.Println("KubeArmor already present as systemd.")
			return nil
		}

		err = install.EnsureSystemdPackage(releaseVersion)
		if err != nil {
			return fmt.Errorf("error ensuring systemd package: %v", err)
		}

		err = install.GenerateKubeArmorConfig(configOptions)
		if err != nil {
			return fmt.Errorf("error configuring the KubeArmor config: %v", err)
		}

		if err := install.Run(install.SystemdRuntime); err != nil {
			return fmt.Errorf("error running systemd installation: %v", err)
		}

		fmt.Println("KubeArmor installed successfully using systemd.")

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
	nonK8sInstallCmd.Flags().StringVar(&secureRuntime, "secure", "", "Specify the container runtime (e.g., podman, docker)")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.Visibility, "visibility", "process,file,network,capabilities", "Specify visibility settings for container telemetry")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.HostVisibility, "hostVisibility", "process,file,network,capabilities", "Specify visibility settings for host telemetry")
	nonK8sInstallCmd.Flags().BoolVar(&composeConfigOptions.EnableKubeArmorHostPolicy, "enableKubeArmorHostPolicy", true, "Enable KubeArmor host policy")
	nonK8sInstallCmd.Flags().BoolVar(&composeConfigOptions.EnableKubeArmorPolicy, "enableKubeArmorPolicy", true, "Enable KubeArmor policy")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.DefaultFilePosture, "defaultFilePosture", "audit", "Set default file posture")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.DefaultNetworkPosture, "defaultNetworkPosture", "audit", "Set default network posture")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.DefaultCapabilitiesPosture, "defaultCapabilitiesPosture", "audit", "Set default capabilities network posture")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.HostDefaultFilePosture, "hostDefaultFilePosture", "audit", "Set default host file posture")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.HostDefaultNetworkPosture, "hostDefaultNetworkPosture", "audit", "Set default host network posture")
	nonK8sInstallCmd.Flags().StringVar(&composeConfigOptions.HostDefaultCapabilitiesPosture, "hostDefaultCapabilitiesPosture", "audit", "Set default host capabilities network posture")
	//systemd installation takes these flags
	systemdInstallCmd.Flags().StringVar(&releaseVersion, "version", "latest", "Specify the version of Kubearmor")
	systemdInstallCmd.Flags().StringVar(&configOptions.HostVisibility, "hostVisibility", "process,file,network,capabilities", "Specify visibility settings for host telemetry")
	systemdInstallCmd.Flags().BoolVar(&configOptions.EnableKubeArmorHostPolicy, "enableKubeArmorHostPolicy", true, "Enable KubeArmor host policy")
	systemdInstallCmd.Flags().BoolVar(&configOptions.EnableKubeArmorVm, "enableKubeArmorVm", false, "Enable KubeArmor for virtual machines")
	systemdInstallCmd.Flags().BoolVar(&configOptions.AlertThrottling, "alertThrottling", true, "Enable alert throttling")
	systemdInstallCmd.Flags().IntVar(&configOptions.MaxAlertPerSec, "maxAlertPerSec", 10, "Set maximum alerts per second")
	systemdInstallCmd.Flags().IntVar(&configOptions.ThrottleSec, "throttleSec", 30, "Set alert throttle duration in seconds")
}
