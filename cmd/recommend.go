// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/recommend"
	"github.com/spf13/cobra"
)

var recommendOptions recommend.Options

// recommendCmd represents the recommend command
var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend Policies",
	Long:  `Recommend policies based on container image, k8s manifest or the actual runtime env`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if err := recommend.Recommend(client, recommendOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(recommendCmd)

	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Images, "image", "i", []string{}, "Container image list (comma separated)")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Labels, "labels", "l", []string{}, "User defined labels for policy (comma separated)")
	recommendCmd.Flags().StringVarP(&recommendOptions.Namespace, "namespace", "n", "", "User defined namespace value for policies")
	recommendCmd.Flags().BoolVar(&recommendOptions.Update, "update", false, "Updates the local copy of policy-templates to latest release")
	recommendCmd.Flags().StringVarP(&recommendOptions.OutDir, "outdir", "o", "out", "output folder to write policies")
	recommendCmd.Flags().StringVarP(&recommendOptions.ReportFile, "report", "r", "report.txt", "report file")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Tags, "tag", "t", []string{}, "tags (comma-separated) to apply. Eg. PCI-DSS, MITRE")
}
