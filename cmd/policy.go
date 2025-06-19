// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
//go:build darwin || (linux && !windows)

package cmd

import (
	"errors"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var policyOptions vm.PolicyOptions

// vmPolicyCmd represents the vm command for policy enforcement
var vmPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "policy handling for non kubernetes/bare metal KubeArmor",
	Long: `Manage standalone (VM or bare‑metal) KubeArmor security policies.

The “policy” command lets you add, delete, and later list or update enforcement rules 
when running KubeArmor outside a Kubernetes cluster. Policies are defined in YAML and 
sent over gRPC to the local KubeArmor agent.

Subcommands:
  • add     Apply a new policy from a YAML file (kubearmor vm policy add <file>)
  • delete  Remove an existing policy by its YAML file (kubearmor vm policy delete <file>)

Global Flags:
  • --gRPC <address>   Address of the KubeArmor gRPC server (host:port)

Examples:
  # Apply a file‑access policy on a standalone host:
  karmor vm policy add ./file-access.yaml --gRPC 127.0.0.1:50051

  # Remove that policy when finished:
  karmor vm policy delete ./file-access.yaml --gRPC 127.0.0.1:50051

See each subcommand’s help for more details:
  karmor vm policy add --help
  karmor vm policy delete --help
`,
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

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmPolicyCmd)

	// Subcommand for policy command
	vmPolicyCmd.AddCommand(vmPolicyAddCmd)
	vmPolicyCmd.AddCommand(vmPolicyDeleteCmd)

	// gRPC endpoint flag to communicate with KubeArmor. Available across subcommands.
	vmPolicyCmd.PersistentFlags().StringVar(&policyOptions.GRPC, "gRPC", "", "gRPC server information")
}
