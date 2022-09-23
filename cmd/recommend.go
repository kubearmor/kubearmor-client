// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/recommend"
	log "github.com/sirupsen/logrus"
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
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates policy-template cache",
	Long:  "Updates the local cache of policy-templates ($HOME/.cache/karmor)",
	RunE: func(cmd *cobra.Command, args []string) error {

		if d, err := recommend.DownloadAndUnzipRelease(); err != nil {
			fmt.Println(d)
			return err
		}
		log.WithFields(log.Fields{
			"Current Version": recommend.CurrentVersion,
		}).Info("policy-templates updated")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(recommendCmd)
	recommendCmd.AddCommand(updateCmd)

	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Images, "image", "i", []string{}, "Container image list (comma separated)")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Labels, "labels", "l", []string{}, "User defined labels for policy (comma separated)")
	recommendCmd.Flags().StringVarP(&recommendOptions.Namespace, "namespace", "n", "", "User defined namespace value for policies")
	recommendCmd.Flags().StringVarP(&recommendOptions.OutDir, "outdir", "o", "out", "output folder to write policies")
	recommendCmd.Flags().StringVarP(&recommendOptions.ReportFile, "report", "r", "report.txt", "report file")
	recommendCmd.Flags().StringSliceVarP(&recommendOptions.Tags, "tag", "t", []string{}, "tags (comma-separated) to apply. Eg. PCI-DSS, MITRE")
}
