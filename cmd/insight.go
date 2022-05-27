// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/insight"
	"github.com/spf13/cobra"
)

var insightOptions insight.Options

// insightCmd represents the insight command
var insightCmd = &cobra.Command{
	Use:   "insight",
	Short: "Policy insight from discovery engine",
	Long:  `Policy insight from discovery engine`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := insight.StartInsight(insightOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(insightCmd)

	insightCmd.Flags().StringVar(&insightOptions.GRPC, "grpc", "", "gRPC server information")
	insightCmd.Flags().StringVar(&insightOptions.Source, "class", "all", "The DB for insight : system|network|all")
	insightCmd.Flags().StringVar(&insightOptions.Labels, "labels", "", "Labels for resources")
	insightCmd.Flags().StringVar(&insightOptions.Containername, "containername", "", "Filter according to the Container name")
	insightCmd.Flags().StringVar(&insightOptions.Clustername, "clustername", "", "Filter according to the Cluster name")
	insightCmd.Flags().StringVar(&insightOptions.Fromsource, "fromsource", "", "Filter according to the source path")
	insightCmd.Flags().StringVarP(&insightOptions.Namespace, "namespace", "n", "", "Namespace for resources")
	insightCmd.Flags().StringVar(&insightOptions.Type, "type", "", "NW packet type : ingress|egress")
	insightCmd.Flags().StringVar(&insightOptions.Rule, "rule", "", "NW packet Rule")
}
