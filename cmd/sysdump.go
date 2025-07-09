// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/sysdump"
	"github.com/spf13/cobra"
)

var dumpOptions sysdump.Options

// sysdumpCmd represents the get command
var sysdumpCmd = &cobra.Command{
	Use:   "sysdump",
	Short: "Collect system dump information for troubleshooting and error report",
	Long:  `Collect system dump information for troubleshooting and error reports`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sysdump.Collect(client, dumpOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sysdumpCmd)
	sysdumpCmd.Flags().StringVarP(&dumpOptions.Filename, "file", "f", "", "output file to use")
	sysdumpCmd.Flags().BoolVar(&dumpOptions.NonK8s, "nonk8s", false, "Collect sysdump for non-Kubernetes (process/VM) environments")
}
