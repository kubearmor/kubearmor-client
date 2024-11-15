// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/recommend"
	"github.com/kubearmor/kubearmor-client/recommend/common"
	genericpolicies "github.com/kubearmor/kubearmor-client/recommend/engines/generic_policies"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var recommendOptions common.Options

// recommendCmd represents the recommend command
var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend Policies",
	Long:  `Recommend policies based on container image, k8s manifest or the actual runtime env`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := recommend.Recommend(client, recommendOptions, genericpolicies.GenericPolicy{})
		return err
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates policy-template cache",
	Long:  "Updates the local cache of policy-templates ($HOME/.cache/karmor)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := genericpolicies.DownloadAndUnzipRelease(); err != nil {
			return err
		}
		log.WithFields(log.Fields{
			"Current Version": genericpolicies.CurrentVersion,
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
	recommendCmd.Flags().StringVarP(&recommendOptions.Config, "config", "c", common.UserHome()+"/.docker/config.json", "absolute path to image registry configuration file")
}
