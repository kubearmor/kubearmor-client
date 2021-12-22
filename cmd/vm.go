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
	policyOption  vm.PolicyOption
)

// vmCmd represents the root vm command for non-k8s control plane management
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
		if err := vm.VmAdd(policyOption.PolicyFile); err != nil {
			return err
		}
		return nil
	},
}

// vmDelCmd represents the vm command for vm onboarding
var vmDelCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete/offboard a vm from nonk8s control plane",
	Long:  `delete/offboard a vm from nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.VmDelete(policyOption.PolicyFile); err != nil {
			return err
		}
		return nil
	},
}

// vmDelCmd represents the vm command for vm onboarding
var vmListCmd = &cobra.Command{
	Use:   "list",
	Short: "display the list of configured VMs in non-k8s control plane",
	Long:  `display the list of configured VMs in non-k8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.VmList(); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelCmd represents the vm command for policy enforcement
var vmPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "manage policy enforcement for nonk8s control plane",
	Long:  `manage policy enforcement for nonk8s control plane`,
}

// vmLabelCmd represents the vm command for policy enforcement
var vmPolicyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add new policy",
	Long:  `add new policy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.PolicyAdd(policyOption.PolicyFile); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelCmd represents the vm command for policy enforcement
var vmPolicyUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "modify existing policy",
	Long:  `modify existing policy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.PolicyUpdate(policyOption.PolicyFile); err != nil {
			return err
		}
		return nil
	},
}

// vmLabelCmd represents the vm command for policy enforcement
var vmPolicyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete policy",
	Long:  `delete policy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.PolicyDelete(policyOption.PolicyFile); err != nil {
			return err
		}
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

func populateVmPolicySubCommands() {
	vmPolicyCmd.AddCommand(vmPolicyAddCmd)
	vmPolicyCmd.AddCommand(vmPolicyUpdateCmd)
	vmPolicyCmd.AddCommand(vmPolicyDeleteCmd)

	vmPolicyAddCmd.Flags().StringVarP(&policyOption.PolicyFile, "file", "f", "none", "Filename with path for policy yaml")
	vmPolicyUpdateCmd.Flags().StringVarP(&policyOption.PolicyFile, "file", "f", "none", "Filename with path for policy yaml")
	vmPolicyDeleteCmd.Flags().StringVarP(&policyOption.PolicyFile, "file", "f", "none", "Filename with path for policy yaml")

	// Marking this flag as markedFlag and mandatory
	err := vmPolicyAddCmd.MarkFlagRequired("file")
	if err != nil {
		_ = fmt.Errorf("file path not provided")
	}

	// Marking this flag as markedFlag and mandatory
	err = vmPolicyUpdateCmd.MarkFlagRequired("file")
	if err != nil {
		_ = fmt.Errorf("file path not provided")
	}

	// Marking this flag as markedFlag and mandatory
	err = vmPolicyDeleteCmd.MarkFlagRequired("file")
	if err != nil {
		_ = fmt.Errorf("file path not provided")
	}
}

func populateGetScriptCommand() {
	// Options/Sub-commands for vm script download
	vmScriptCmd.Flags().BoolVarP(&scriptOptions.NonK8s, "nonk8s", "n", true, "Non-k8s env")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.Port, "port", "p", "32770", "Port of kvmservice")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.VMName, "kvm", "v", "", "Name of configured vm")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.File, "file", "f", "none", "Filename with path to store the configured vm installation script")

	// Marking this flag as markedFlag and mandatory
	err := vmScriptCmd.MarkFlagRequired("kvm")
	if err != nil {
		_ = fmt.Errorf("kvm option not supplied")
	}
}

func populateVmOnboardingCommands() {
	vmCmd.AddCommand(vmAddCmd)
	vmCmd.AddCommand(vmDelCmd)
	vmCmd.AddCommand(vmListCmd)

	vmAddCmd.Flags().StringVarP(&policyOption.PolicyFile, "file", "f", "none", "Filename with path for policy yaml")
	vmDelCmd.Flags().StringVarP(&policyOption.PolicyFile, "file", "f", "none", "Filename with path for policy yaml")

	// Marking this flag as markedFlag and mandatory
	err := vmAddCmd.MarkFlagRequired("file")
	if err != nil {
		_ = fmt.Errorf("file path not provided")
	}

	// Marking this flag as markedFlag and mandatory
	err = vmDelCmd.MarkFlagRequired("file")
	if err != nil {
		_ = fmt.Errorf("file path not provided")
	}
}

// ========== //
// == Init == //
// ========== //

func init() {
	rootCmd.AddCommand(vmCmd)

	// All subcommands
	vmCmd.AddCommand(vmPolicyCmd)
	vmCmd.AddCommand(vmScriptCmd)
	vmCmd.AddCommand(vmLabelCmd)

	// Options/Sub-commands for vm onboarding/offboarding
	populateVmOnboardingCommands()

	// Options/Sub-commands for vm policy management
	populateVmPolicySubCommands()

	// Options for vm installation script download
	populateGetScriptCommand()

}
