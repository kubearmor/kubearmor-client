// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/summary"
	"github.com/spf13/cobra"
)

var summaryOptions summary.Options

// summaryCmd represents the summary command
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Policy summary from discovery engine",
	Long:  `Policy summary from discovery engine`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := summary.StartSummary(summaryOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryOptions.Source, "source", "all", "The DB for summary : system|network|all")
	summaryCmd.Flags().StringVar(&summaryOptions.Labels, "labels", "", "Labels for resources")
	summaryCmd.Flags().StringVar(&summaryOptions.Fromsource, "fromsource", "", "Filter according to the source path")
	summaryCmd.Flags().StringVarP(&summaryOptions.Namespace, "namespace", "n", "", "Namespace for resources")
	//summaryCmd.Flags().StringVar(&summaryOptions.Type, "type", "", "NW packet type : ingress|egress")
}
