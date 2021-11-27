// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/vm"
	"github.com/spf13/cobra"
)

var vmOptions vm.Options

// vmCmd represents the vm command
var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Download VM install script from kvmservice",
	Long:  `Download VM install script from kvmservice`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.FileDownload(client, vmOptions); err != nil {
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
	vmCmd.Flags().StringVarP(&vmOptions.Port, "port", "p", "32770", "Port of kvmservice")
	vmCmd.Flags().StringVarP(&vmOptions.VMName, "kvm", "v", "", "Name of configured vm")
	vmCmd.Flags().StringVarP(&vmOptions.File, "file", "f", "none", "Filename with path to store the configured vm installation script")

	// Marking this flag as markedFlag and mandatory
	err := vmCmd.MarkFlagRequired("kvm")
	if err != nil {
		_ = fmt.Errorf("kvm option not supplied")
	}
}
