// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/probe"
	"github.com/spf13/cobra"
)

var probeInstallOptions probe.Options

// probeCmd represents the get command
var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Checks for supported KubeArmor features in the current environment",
	Long: `Checks for supported KubeArmor features in the current environment.

If KubeArmor is not running, it does a precheck to know if kubearmor will be supported in the environment
and what KubeArmor features will be supported e.g: observability, enforcement, etc. 
	 
If KubeArmor is running, It probes which environment KubeArmor is running on (e.g: systemd mode, kubernetes etc.), 
the supported KubeArmor features in the environment, the pods being handled by KubeArmor and the policies running on each of these pods`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := probe.PrintProbeResultCmd(client, probeInstallOptions)
		return err
	},
}

func init() {
	rootCmd.AddCommand(probeCmd)
	probeCmd.Flags().StringVarP(&probeInstallOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	probeCmd.Flags().BoolVar(&probeInstallOptions.Full, "full", false, `If KubeArmor is not running, it deploys a daemonset to have access to more
information on KubeArmor support in the environment and deletes daemonset after probing`)
	probeCmd.Flags().StringVarP(&probeInstallOptions.Output, "format", "f", "text", "Format: json or text or no-color")
	probeCmd.Flags().StringVar(&probeInstallOptions.GRPC, "gRPC", "", "GRPC port ")
}
