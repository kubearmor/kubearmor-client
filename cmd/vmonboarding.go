// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"errors"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

// vmPolicyAddCmd represents the command for vm onboarding
var vmOnboardAddCmd = &cobra.Command{
	Use:   "add",
	Short: "onboard new vm onto nonk8s control plane",
	Long:  `onboard new vm onto nonk8s control plane`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid vm YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.Onboarding("ADDED", args[0]); err != nil {
			return err
		}
		return nil
	},
}

// vmOnboardDeleteCmd represents the command for vm offboarding
var vmOnboardDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "offboard existing vm from nonk8s control plane",
	Long:  `offboard existing vm from nonk8s control plane`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid vm YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.Onboarding("DELETED", args[0]); err != nil {
			return err
		}
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmOnboardAddCmd)
	vmCmd.AddCommand(vmOnboardDeleteCmd)
}
