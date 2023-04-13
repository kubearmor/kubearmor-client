// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/accuknox/accuknox-cli/discover"
	"github.com/spf13/cobra"
)

var discoverOptions discover.Options

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover applicable policies",
	Long:  `Discover applicable policies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := discover.Policy(client, discoverOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().StringVar(&discoverOptions.GRPC, "gRPC", "", "gRPC server information")
	discoverCmd.Flags().StringVarP(&discoverOptions.Format, "format", "f", "yaml", "Format: json or yaml")
	discoverCmd.Flags().StringVarP(&discoverOptions.Policy, "policy", "p", "KubearmorSecurityPolicy", "Type of policies to be discovered: KubearmorSecurityPolicy|CiliumNetworkPolicy|NetworkPolicy")
	discoverCmd.Flags().StringVarP(&discoverOptions.Namespace, "namespace", "n", "", "Filter by Namespace")
	discoverCmd.Flags().StringVarP(&discoverOptions.Clustername, "clustername", "c", "", "Filter by Clustername")
	discoverCmd.Flags().StringVarP(&discoverOptions.Labels, "labels", "l", "", "Filter by policy Label")
	discoverCmd.Flags().StringVarP(&discoverOptions.Fromsource, "fromsource", "s", "", "Filter by policy FromSource")
	discoverCmd.Flags().BoolVar(&discoverOptions.IncludeNetwork, "network", false, "Include network rules in system policies")
}
