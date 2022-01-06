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
	HttpIP        string
	HttpPort      string
	IsNonK8sEnv   bool
)

// vmCmd represents the vm command
var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "VM commands for kvmservice",
	Long:  `VM commands for kvmservice`,
}

// vmScriptCmd represents the vm command for script download
var vmScriptCmd = &cobra.Command{
	Use:   "getscript",
	Short: "download vm installation script for nonk8s control plane",
	Long:  `download vm installation script for nonk8s control plane`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := HttpIP
		isNonK8sEnv := IsNonK8sEnv

		if err := vm.GetScript(client, scriptOptions, ip, isNonK8sEnv); err != nil {
			return err
		}
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	rootCmd.AddCommand(vmCmd)

	// Options for vm script download
	vmScriptCmd.Flags().StringVarP(&scriptOptions.Port, "port", "p", "32770", "Port of kvmservice")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.VMName, "kvm", "v", "", "Name of configured vm")
	vmScriptCmd.Flags().StringVarP(&scriptOptions.File, "file", "f", "none", "Filename with path to store the configured vm installation script")

	// Marking this flag as markedFlag and mandatory
	err := vmScriptCmd.MarkFlagRequired("kvm")
	if err != nil {
		_ = fmt.Errorf("kvm option not supplied")
	}

	// options for vm generic commands related to HTTP Request
	vmCmd.PersistentFlags().StringVar(&HttpIP, "http-ip", "127.0.0.1", "IP of non-k8s control plane")
	vmCmd.PersistentFlags().StringVar(&HttpPort, "http-port", "8080", "IP and port of http request")
	vmCmd.PersistentFlags().BoolVar(&IsNonK8sEnv, "non-k8s", false, "Enable if non-k8s environment/control-plane")

	// All subcommands
	vmCmd.AddCommand(vmScriptCmd)
}
