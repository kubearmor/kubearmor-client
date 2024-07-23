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
	sysdumpCmd.Flags().BoolVar(&dumpOptions.Full, "full", false, `If KubeArmor is not running, it deploys a daemonset to have access to more
information on KubeArmor support in the environment and deletes daemonset after probing`)
	sysdumpCmd.Flags().StringVarP(&dumpOptions.Output, "type", "t", "text", " Output type: json or text ")
	sysdumpCmd.Flags().StringVar(&dumpOptions.GRPC, "gRPC", "", "GRPC port ")
}
