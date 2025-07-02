// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/install"
	"github.com/kubearmor/kubearmor-client/utils"
	"github.com/spf13/cobra"
)

var (
	installOptions install.Options
	vmMode         string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install KubeArmor",
	Long: `Install KubeArmor in either Kubernetes or non‑Kubernetes mode.

This command bootstraps KubeArmor in your environment by deploying all necessary components
and configuring them according to the options you specify. It supports two primary modes:

  • Kubernetes mode (default)
    – Deploys the KubeArmor Operator, Controller, Relay Server, and DaemonSets
      into the target cluster.
    – Honors flags like --namespace, --image, --tag, --operator-image, --controller-image,
      --relay-image, and visibility/audit/block posture settings.
    – Use --legacy to invoke the legacy installer path for clusters that don’t support
      Operator‑based installation.

  • Non‑Kubernetes mode (--nonk8s)
    – Installs KubeArmor as a standalone service on a host or VM.
    – Automatically selects Docker or systemd VM mode based on host capabilities,
      or force a mode with --vm-mode (docker|systemd).
    – Supports secure container monitoring, host‑audit/host‑block posture, and
      telemetry visibility flags just like Kubernetes mode.

Examples:
  # Install with default settings into namespace "kubearmor"
  karmor install

  # Use a specific image tag and enable only network telemetry
  karmor install --tag v1.6.0 --viz network

  # Legacy installer for clusters without Operator support
  karmor install --legacy

  # Non‑Krnetes install using Docker mode
  karmor install --nonk8s --vm-mode docker

For more details on each flag, run:
  karmor install --help
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !installOptions.NonK8s {
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
		} else {
			var cfg install.KubeArmorConfig
			_, err := cfg.ValidateEnv()
			installOptions.VmMode = utils.VMMode(vmMode)
			if installOptions.VmMode == "" {
				if err == nil {
					installOptions.VmMode = utils.VMMode_Docker
				} else {
					fmt.Printf("\n⚠️\tWarning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error())
					installOptions.SecureContainers = false
					installOptions.VmMode = utils.VMMode_Systemd
				}
			} else if installOptions.VmMode == utils.VMMode_Docker && err != nil {
				// docker mode specified explicitly but requirements didn't match
				fmt.Printf("\n⚠️\tFailed to validate environment: %s", err.Error())
				return err
			} else if installOptions.VmMode == utils.VMMode_Systemd && err != nil {
				installOptions.SecureContainers = false
			}
			fmt.Println("ℹ️\tInstalling KubeArmor in non-Kubernetes environment")
			config, err := install.SetKAConfig(&installOptions)
			if err != nil {
				return fmt.Errorf("\n⚠️\terror setting KubeArmor config: %v", err)
			}
			switch installOptions.VmMode {
			case utils.VMMode_Docker:
				err = config.DeployKAdocker()
				if err != nil {
					return fmt.Errorf("\n⚠️\terror installing ka in docker: %v", err)
				}

			case utils.VMMode_Systemd:
				err = config.DeployKASystemd()
				if err != nil {
					return fmt.Errorf("\n⚠️\terror installing ka in systemd: %v", err)
				}

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
	// these flags should only be availabe only for mode k8s
	installCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	// Leaving defaults empty here because the KubeArmor systemd installation uses the 'kubearmor/kubearmor-systemd' repo,
	// whereas the regular (Docker/K8s) installation uses the 'kubearmor/kubearmor' repo.
	installCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "", "Kubearmor daemonset image to use")
	installCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "", "Kubearmor daemonset init container image to use")
	installCmd.Flags().StringVarP(&installOptions.OperatorImage, "operator-image", "", "kubearmor/kubearmor-operator:stable", "Kubearmor operator container image to use")
	installCmd.Flags().StringVarP(&installOptions.ControllerImage, "controller-image", "", "kubearmor/kubearmor-controller:stable", "Kubearmor controller image to use")
	installCmd.Flags().StringVarP(&installOptions.RelayImage, "relay-image", "", "kubearmor/kubearmor-relay-server:stable", "Kubearmor relay image to use")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorTag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorRelayTag, "relay-tag", "", "", "Change image tag/version for default kubearmor-relay image (This will overwrite the tag provided in --relay-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorControllerTag, "controller-tag", "", "", "Change image tag/version for default kubearmor-controller image (This will overwrite the tag provided in --controller-image)")
	installCmd.Flags().StringVarP(&installOptions.KubeArmorOperatorTag, "operator-tag", "", "", "Change image tag/version for default kubearmor-operator image (This will overwrite the tag provided in --operator-image)")
	installCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVar(&installOptions.HostAudit, "host-audit", "", "Kubearmor Host Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVar(&installOptions.HostBlock, "host-block", "", "Kubearmor Host block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Visibility, "viz", "", "", "Kubearmor Telemetry Visibility [process,file,network,none]")
	installCmd.Flags().StringVar(&installOptions.HostVisibility, "host-viz", "", "Kubearmor Host Telemetry Visibility [process,file,network,none]")
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
	installCmd.Flags().IntVar(&installOptions.MaxAlertPerSec, "maxAlertPerSec", 10, "Maximum number of alerts required to trigger alert throttling")
	installCmd.Flags().IntVar(&installOptions.ThrottleSec, "throttleSec", 30, "Time window(in sec) for which there will be no alerts genrated after alert throttling is triggered")
	installCmd.Flags().BoolVar(&installOptions.AnnotateExisting, "annotateExisting", false, "when true kubearmor-controller restarts and annotates existing resources, with required annotations")

	// ============ Flags for non-k8s mode ==============
	installCmd.Flags().BoolVar(&installOptions.NonK8s, "nonk8s", false, "Set it to true to install kubearmor on unorchesrated environment")
	installCmd.Flags().StringVar(&vmMode, "vm-mode", "", "docker or systmed")
	installCmd.Flags().StringVar(&installOptions.ImagePullPolicy, "image-pull-policy", "always", "image pull policy to use. Either of: missing | never | always")
	installCmd.Flags().BoolVar(&installOptions.SecureContainers, "secure-containers", true, "to monitor containers")
	markDeprecated(installCmd, "env", "Only relevant when using legacy")
	markDeprecated(installCmd, "legacy", "KubeArmor now utilizes operator-based installation. This command may not set up KubeArmor in the intended way.")
}
