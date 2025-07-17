// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package cmd is the collection of all the subcommands available in kArmor while providing relevant options for the same
package cmd

import (
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var client *k8s.Client

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		// Initialise k8sClient for all child commands to inherit
		client, err = k8s.ConnectK8sClient()
		// fmt.Printf("%v", client.K8sClientset)
		if err != nil {
			log.Error().Msgf("unable to create Kubernetes clients: %s", err.Error())
			return err
		}
		return nil
	},
	Use:   "karmor",
	Short: "A CLI Utility to help manage KubeArmor",
	Long: `CLI Utility to help manage KubeArmor
	
KubeArmor is a container-aware runtime security enforcement system that
restricts the behavior (such as process execution, file access, and networking
operation) of containers at the system level.
	`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&k8s.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVar(&k8s.ContextName, "context", "", "Name of the kubeconfig context to use")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
