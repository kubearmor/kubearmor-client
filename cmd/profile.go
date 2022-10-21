// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/profile"
	"github.com/spf13/cobra"
)

// logCmd represents the log command
var profilecmd = &cobra.Command{
	Use:   "profile",
	Short: "Profiling of logs",
	Long:  `Profiling of logs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profile.GetLogs()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profilecmd)
}
