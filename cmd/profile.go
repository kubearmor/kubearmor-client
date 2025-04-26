// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/profile"
	profileclient "github.com/kubearmor/kubearmor-client/profile/Client"
	"github.com/spf13/cobra"
)

var profileOptions profile.Options

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
	profilecmd.Flags().StringVar(&profileOptions.LogFilter, "logFilter", "system", "Filter for what kinds of alerts and logs to receive, {policy|system|all}")
	profilecmd.Flags().StringVar(&profileOptions.LogType, "logType", "ContainerLog", "Log type you want (Eg:ContainerLog/HostLog)")
	profilecmd.Flags().StringVar(&profileOptions.Pod, "pod", "", "Filter using Pod name")
	profilecmd.Flags().StringVarP(&profileOptions.Container, "container", "c", "", "name of the container ")
	profilecmd.Flags().BoolVar(&profileOptions.Save, "save", false, "Save Profile data in json format")
}
