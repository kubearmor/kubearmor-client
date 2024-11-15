// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"net"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var labelOptions vm.LabelOptions

// vmLabelCmd represents the vm command for label management
var vmLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "label handling for kvms control plane vm",
	Long:  `label handling for kvms control plane vm`,
}

// vmLabelAddCmd represents the vm add label command for label management
var vmLabelAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add label for kvms control plane vm",
	Long:  `add label for kvms control plane vm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create http address
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)

		if err := vm.LabelHandling("ADD", labelOptions, httpAddress, IsKvmsEnv); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelDeleteCmd represents the vm add label command for label management
var vmLabelDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete label for kvms control plane vm",
	Long:  `delete label for kvms control plane vm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create http address
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)

		if err := vm.LabelHandling("DELETE", labelOptions, httpAddress, IsKvmsEnv); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelListCmd represents the vm list label command for label management
var vmLabelListCmd = &cobra.Command{
	Use:   "list",
	Short: "list labels for vm in k8s/nonk8s control plane",
	Long:  `list labels for vm in k8s/nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create http address
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)

		if err := vm.LabelHandling("LIST", labelOptions, httpAddress, IsKvmsEnv); err != nil {
			return err
		}
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmLabelCmd)

	vmLabelCmd.PersistentFlags().StringVar(&labelOptions.VMName, "vm", "", "VM name")
	vmLabelCmd.PersistentFlags().StringVar(&labelOptions.VMLabels, "label", "", "list of labels")

	// Subcommand for policy command
	vmLabelCmd.AddCommand(vmLabelAddCmd)
	vmLabelCmd.AddCommand(vmLabelDeleteCmd)
	vmLabelCmd.AddCommand(vmLabelListCmd)
}
