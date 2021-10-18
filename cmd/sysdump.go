// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/sysdump"
	"github.com/spf13/cobra"
)

// sysdumpCmd represents the get command
var sysdumpCmd = &cobra.Command{
	Use:   "sysdump",
	Short: "Collect system dump information for troubleshooting and error report",
	Long:  `Collect system dump information for troubleshooting and error reports`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sysdump.Collect(client); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sysdumpCmd)
}
