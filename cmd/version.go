// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the get command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display version information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := version.PrintVersion(k8sClient); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
