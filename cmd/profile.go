// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	profileclient "github.com/kubearmor/kubearmor-client/profile/Client"
	"github.com/spf13/cobra"
)

// profileCmd represents the profile command
var profilecmd = &cobra.Command{
	Use:   "profile",
	Short: "Profiling of logs",
	Long:  `Profiling of logs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileclient.Start()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profilecmd)
	profilecmd.Flags().StringVar(&profileclient.ProfileOpts.GRPC, "gRPC", "", "use gRPC")
	profilecmd.Flags().StringVarP(&profileclient.ProfileOpts.Namespace, "namespace", "n", "", "Filter using namespace")
	profilecmd.Flags().StringVar(&profileclient.ProfileOpts.Pod, "pod", "", "Filter using Pod name")
	profilecmd.Flags().StringVarP(&profileclient.ProfileOpts.Container, "container", "c", "", "name of the container ")
	profilecmd.Flags().BoolVar(&profileclient.ProfileOpts.Save, "save", false, "Save Profile data in json format")
}
