// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"errors"

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
		//Condition to check if at least one Container image name is passes as an argument
		if len(recommendOptions.Images) < 1 {
			return errors.New("at least one container image is required as an argument")
		}
		if err := recommend.Recommend(client, recommendOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(recommendCmd)

	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Images, "image", "i", []string{}, "Container image list (comma separated)")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.UseLabels, "use-labels", "", []string{}, "User defined labels for policy (comma separated)")
	recommendCmd.Flags().StringVarP(&recommendOptions.UseNamespace, "use-namespace", "", "", "User defined namespace value for policies")
	recommendCmd.Flags().StringVarP(&recommendOptions.OutDir, "outdir", "o", "out", "output folder to write policies")
	recommendCmd.Flags().StringVarP(&recommendOptions.ReportFile, "report", "r", "report.txt", "report file")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Tags, "tag", "t", []string{}, "tags (comma-separated) to apply. Eg. PCI-DSS, MITRE")
}
