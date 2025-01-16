// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"errors"
	"net"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

// vmOnboardAddCmd represents the command for vm onboarding
var vmOnboardAddCmd = &cobra.Command{
	Use:   "add",
	Short: "onboard new VM onto kvms control plane vm",
	Long:  `onboard new VM onto kvms control plane vm`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid vm YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)
		if err := vm.Onboarding("ADDED", args[0], httpAddress); err != nil {
			return err
		}
		return nil
	},
}

// vmOnboardDeleteCmd represents the command for vm offboarding
var vmOnboardDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "offboard existing VM from kvms control plane vm",
	Long:  `offboard existing VM from kvms control plane vm`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a path to valid vm YAML as argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)
		if err := vm.Onboarding("DELETED", args[0], httpAddress); err != nil {
			return err
		}
		return nil
	},
}

// vmListCmd represents the command for vm listing
var vmListCmd = &cobra.Command{
	Use:   "list",
	Short: "list configured VMs",
	Long:  `list configured VMs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddress := "http://" + net.JoinHostPort(HTTPIP, HTTPPort)
		if err := vm.List(httpAddress); err != nil {
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
	vmCmd.AddCommand(vmListCmd)
}
