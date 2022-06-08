// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/discover"
	"github.com/spf13/cobra"
)

var discoverOptions discover.Options

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover applicable policies",
	Long:  `Discover applicable policies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := discover.Policy(discoverOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().StringVar(&discoverOptions.GRPC, "grpc", "", "gRPC server information")
	discoverCmd.Flags().StringVarP(&discoverOptions.Format, "format", "f", "json", "Format: json or yaml")
	discoverCmd.Flags().StringVar(&discoverOptions.Class, "class", "application", "Type of policies to be discovered: application or network ")
	discoverCmd.Flags().StringVarP(&discoverOptions.Namespace, "namespace", "n", "", "Filter by Namespace")
	discoverCmd.Flags().StringVarP(&discoverOptions.Clustername, "clustername", "c", "", "Filter by Clustername")
	discoverCmd.Flags().StringVarP(&discoverOptions.Labels, "labels", "l", "", "Filter by policy Label")
	discoverCmd.Flags().StringVarP(&discoverOptions.Fromsource, "fromsource", "s", "", "Filter by policy FromSource")
}
