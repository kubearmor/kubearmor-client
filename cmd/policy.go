// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"errors"

	"github.com/kubearmor/kubearmor-client/policy"
	"github.com/spf13/cobra"
)

var policyOptions policy.PolicyOptions

// policyCmd represents command for policy enforcement
var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy handling for kubearmor standalone",
	Long:  `Policy handling for kubearmor standalone`,
}

// policyAddCmd represents the add policy command for policy enforcement
var policyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add policy for standlaone kubearmor host policy",
	Long:  `add policy for standlaone kubearmor host policy`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := policy.PolicyHandling("ADDED", args[0], policyOptions); err != nil {
			return err
		}
		return nil
	},
}

// policyDeleteCmd represents the delete policy command for policy enforcement
var policyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete policy for standlaone kubearmor host policy",
	Long:  `delete policy for standlaone kubearmor host policy`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid policy YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := policy.PolicyHandling("DELETED", args[0], policyOptions); err != nil {
			return err
		}
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	rootCmd.AddCommand(policyCmd)

	// Subcommand for policy command
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyDeleteCmd)

	// gRPC endpoint flag to communicate with KubeArmor. Available across subcommands.
	policyCmd.PersistentFlags().StringVar(&policyOptions.GRPC, "gRPC", "", "gRPC server information")
}
