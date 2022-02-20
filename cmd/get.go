// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/get"
	"github.com/spf13/cobra"
)

var options get.Options

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Display specified resources",
	Long:  `Display specified resources`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := get.Resources(client, options); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Namespace for resources")
}
