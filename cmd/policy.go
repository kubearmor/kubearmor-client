// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

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

var vmPolicyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "get policy for bare-metal vm/kvms control plane vm",
	Long: `Retrieve and inspect standalone (VM or bare‑metal) KubeArmor security policies.

The "get" command lets you list all enforced policies or view the full YAML content of a specific policy, 
for both container and host types, when running KubeArmor outside a Kubernetes cluster.

By default, container policies are shown. Use the --type flag to select host policies.

Examples:
  # List all container policies:
  karmor vm policy get

  # List all host policies:
  karmor vm policy get --type=hsp

  # Show the YAML content of a specific container policy:
  karmor vm policy get <policy>

  # Show the YAML content of a specific host policy:
  karmor vm policy get --type=hsp <policy>

See --help for more details on the --type flag and usage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return policyOptions.HandleGet(args)
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmPolicyCmd)

	// Add flags to vmPolicyGetCmd
	vmPolicyGetCmd.Flags().StringVarP(&policyOptions.Type, "type", "t", "ksp", "Specify the type of policy to get.\n ksp/container/Container for Container policy\n hsp/host/Host for Host policy")

	// Subcommand for policy command
	vmPolicyCmd.AddCommand(vmPolicyAddCmd)
	vmPolicyCmd.AddCommand(vmPolicyDeleteCmd)
	vmPolicyCmd.AddCommand(vmPolicyGetCmd)

	// gRPC endpoint flag to communicate with KubeArmor. Available across subcommands.
	vmPolicyCmd.PersistentFlags().StringVar(&policyOptions.GRPC, "gRPC", "", "gRPC server information")
}
