// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"context"
	"github.com/kubearmor/kubearmor-client/recommend"
	"github.com/kubearmor/kubearmor-client/recommend/common"
	genericpolicies "github.com/kubearmor/kubearmor-client/recommend/engines/generic_policies"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var recommendOptions common.Options

// recommendCmd represents the recommend command
var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend Policies",
	Long:  `Recommend policies based on container image, k8s manifest or the actual runtime env`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if recommendOptions.K8s {
			// Check if k8sClient can connect to the server by listing namespaces
			_, err := k8sClient.K8sClientset.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})
			if err != nil {
				if len(recommendOptions.Images) == 0 { // only log the client if no images are provided
					log.Error("K8s client is not initialized, using docker client instead")
				}
				return recommend.Recommend(dockerClient, recommendOptions, genericpolicies.GenericPolicy{})
			}
			return recommend.Recommend(k8sClient, recommendOptions, genericpolicies.GenericPolicy{})
		} else {
			return recommend.Recommend(dockerClient, recommendOptions, genericpolicies.GenericPolicy{})
		}
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
	recommendCmd.Flags().BoolVarP(&recommendOptions.K8s, "k8s", "k", true, "Use k8s client instead of docker client")
}
