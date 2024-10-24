// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
//go:build darwin || (linux && !windows)

package cmd

import (
	"errors"

	"github.com/kubearmor/kubearmor-client/utils"
	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var policyOptions vm.PolicyOptions

// vmPolicyCmd represents the vm command for policy enforcement
var vmPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "policy handling for non kubernetes/bare metal KubeArmor",
	Long:  `policy handling for non kubernetes/bare metal KubeArmor`,
}

// vmPolicyAddCmd represents the vm add policy command for policy enforcement
var vmPolicyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add policy for non kubernetes/bare metal KubeArmor",
	Long:  `add policy for non kubernetes/bare metal KubeArmor`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.PolicyHandling("ADDED", args[0], policyOptions); err != nil {
			return err
		}
		return nil
	},
}

// vmPolicyDeleteCmd represents the vm delete policy command for policy enforcement
var vmPolicyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete policy for non kubernetes/bare metal KubeArmor",
	Long:  `delete policy for non kubernetes/bare metal KubeArmor`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.PolicyHandling("DELETED", args[0], policyOptions); err != nil {
			return err
		}
		return nil
	},
}

var vmPolicyListCmd = &cobra.Command{
	Use:   "list",
	Short: "list policy for bare-metal vm/kvms control plane vm",
	Long:  `list policy for bare-metal vm/kvms control plane vm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		PolicyData, err := vm.GetPolicy(policyOptions)
		if err != nil {
			return err
		}
		armoredContainer, _ := utils.GetArmoredContainerData(PolicyData.ContainerList, PolicyData.ContainerMap)
		policyOptions.PrintContainersSystemd(armoredContainer)
		return nil
	},
}

var vmPolicyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "get policy for bare-metal vm/kvms control plane vm",
	Long:  `get policy for bare-metal vm/kvms control plane vm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmPolicyCmd)

	// Subcommand for policy command
	vmPolicyCmd.AddCommand(vmPolicyAddCmd)
	vmPolicyCmd.AddCommand(vmPolicyDeleteCmd)
	vmPolicyCmd.AddCommand(vmPolicyListCmd)
	vmPolicyCmd.AddCommand(vmPolicyGetCmd)

	// gRPC endpoint flag to communicate with KubeArmor. Available across subcommands.
	vmPolicyCmd.PersistentFlags().StringVar(&policyOptions.GRPC, "gRPC", "", "gRPC server information")
}
