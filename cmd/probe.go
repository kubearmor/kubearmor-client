// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/probe"
	"github.com/spf13/cobra"
)

var probeInstallOptions probe.ProbeOptions

// probeCmd represents the get command
var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Display probe information",
	Long:  `Display probe information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := probe.PrintProbeResult(client, probeInstallOptions); err != nil {
			return err
		}
		return nil
		
	},
}

func init() {
	rootCmd.AddCommand(probeCmd)
	probeCmd.Flags().StringVarP(&probeInstallOptions.Namespace, "namespace", "n", "default", "Namespace for resources")
	probeCmd.Flags().BoolVar(&probeInstallOptions.Full, "full", false, "Full performs full probing")
}
