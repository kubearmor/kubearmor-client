// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"errors"
	"fmt"
	"github.com/kubearmor/kubearmor-client/utils"
	"net"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var policyOptions vm.PolicyOptions

// vmPolicyCmd represents the vm command for policy enforcement
var vmPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "policy handling for bare-metal vm/kvms control plane vm",
	Long:  `policy handling for bare-metal vm/kvms control plane vm`,
}

// vmPolicyAddCmd represents the vm add policy command for policy enforcement
var vmPolicyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add policy for bare-metal vm/kvms control plane vm",
	Long:  `add policy for bare-metal vm/kvms control plane vm`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create http address
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)

		if err := vm.PolicyHandling("ADDED", args[0], policyOptions, httpAddress, IsKvmsEnv); err != nil {
			return err
		}
		return nil
	},
}

// vmPolicyDeleteCmd represents the vm delete policy command for policy enforcement
var vmPolicyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete policy for bare-metal vm/kvms control plane vm",
	Long:  `delete policy for bare-metal vm/kvms control plane vm`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)

		if err := vm.PolicyHandling("DELETED", args[0], policyOptions, httpAddress, IsKvmsEnv); err != nil {
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
		if len(args) < 1 {
			return errors.New("requires a container name as argument")
		}
		PolicyData, err := vm.GetPolicy(policyOptions)
		if err != nil {
			return err
		}
		if containerMap, ok := PolicyData.ContainerMap[args[0]]; ok {
			for _, policy := range containerMap.PolicyDataList {
				fmt.Println(string(policy.Policy))
			}
		} else {
			return errors.New("no policy found for container: " + args[0])
		}
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
