// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var (
	scriptOptions vm.ScriptOptions
)

// vmCmd represents the vm command
var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "VM commands for kvmservice",
	Long:  `VM commands for kvmservice`,
}

// vmAddCmd represents the vm command for vm onboarding
var vmAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add/onboard a new vm for nonk8s control plane",
	Long:  `add/onboard a new vm for nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// vmDelCmd represents the vm command for vm onboarding
var vmDelCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete/offboard a vm from nonk8s control plane",
	Long:  `delete/offboard a vm from nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// vmScriptCmd represents the vm command for script download
var vmScriptCmd = &cobra.Command{
	Use:   "getscript",
	Short: "download vm installation script for nonk8s control plane",
	Long:  `download vm installation script for nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.GetScript(client, scriptOptions); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelCmd represents the vm command for script download
var vmLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "manage vm labels for nonk8s control plane",
	Long:  `manage vm labels for nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	rootCmd.AddCommand(vmCmd)

	// All subcommands
	vmCmd.AddCommand(vmAddCmd)
	vmCmd.AddCommand(vmDelCmd)
	vmCmd.AddCommand(vmPolicyCmd)
	vmCmd.AddCommand(vmScriptCmd)
	vmCmd.AddCommand(vmLabelCmd)

	// Options for vm script download
	vmScriptCmd.Flags().StringVarP(&scriptOptions.Port, "port", "p", "32770", "Port of kvmservice")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.VMName, "kvm", "v", "", "Name of configured vm")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.File, "file", "f", "none", "Filename with path to store the configured vm installation script")

	// Marking this flag as markedFlag and mandatory
	err := vmScriptCmd.MarkFlagRequired("kvm")
	if err != nil {
		_ = fmt.Errorf("kvm option not supplied")
	}

}
