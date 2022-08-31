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
	Short: "Observability from discovery engine",
	Long:  `Discovery engine keeps the telemetry information from the policy enforcement engines and the karmor connects to it to provide this as observability data`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := summary.Summary(summaryOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)
	summaryCmd.Flags().StringVarP(&summaryOptions.Labels, "labels", "l", "", "Labels")
	summaryCmd.Flags().StringVarP(&summaryOptions.Namespace, "namespace", "n", "", "Namespace")
	summaryCmd.Flags().StringVarP(&summaryOptions.PodName, "pod", "p", "", "PodName")
	summaryCmd.Flags().StringVar(&summaryOptions.ClusterName, "cluster", "", "Cluster name")
	summaryCmd.Flags().StringVar(&summaryOptions.ContainerName, "container", "", "Container name")
	summaryCmd.Flags().BoolVar(&summaryOptions.RevDNSLookup, "rev-dns-lookup", false, "Reverse DNS Lookup")
}
