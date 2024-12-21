// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	profileclient "github.com/kubearmor/kubearmor-client/profile/Client"
	"github.com/spf13/cobra"
)

var profileOptions profileclient.Options

// profileCmd represents the profile command
var profilecmd = &cobra.Command{
	Use:   "profile",
	Short: "Profiling of logs",
	Long:  `Profiling of logs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileclient.Start(profileOptions)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profilecmd)
	profilecmd.Flags().StringVar(&profileOptions.GRPC, "gRPC", "", "use gRPC")
	profilecmd.Flags().StringVarP(&profileOptions.Namespace, "namespace", "n", "", "Filter using namespace")
	profilecmd.Flags().StringVar(&profileOptions.Pod, "pod", "", "Filter using Pod name")
	profilecmd.Flags().StringVarP(&profileOptions.Container, "container", "c", "", "name of the container ")
	profilecmd.Flags().StringVarP(&profileOptions.Output, "output", "o", "", "Output profile data to the specified directory")
}
