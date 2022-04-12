// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/log"
	"github.com/spf13/cobra"
)

var logOptions log.Options

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Observe Logs from KubeArmor",
	Long:  `Observe Logs from KubeArmor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.StopChan = make(chan struct{})
		if err := log.StartObserver(logOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)

	logCmd.Flags().StringVar(&logOptions.GRPC, "gRPC", "", "gRPC server information")
	logCmd.Flags().StringVar(&logOptions.MsgPath, "msgPath", "none", "Output location for messages, {path|stdout|none}")
	logCmd.Flags().StringVar(&logOptions.LogPath, "logPath", "stdout", "Output location for alerts and logs, {path|stdout|none}")
	logCmd.Flags().StringVar(&logOptions.LogFilter, "logFilter", "policy", "Filter for what kinds of alerts and logs to receive, {policy|system|all}")
	logCmd.Flags().BoolVar(&logOptions.JSON, "json", false, "Flag to print alerts and logs in the JSON format")
	logCmd.Flags().StringVar(&logOptions.Namespace, "namespace", "", "Specify the namespace")
	logCmd.Flags().StringVar(&logOptions.Operation, "operation", "", "Give the type of the operation (Eg:Process/File/Network)")
	logCmd.Flags().StringVar(&logOptions.LogType, "logType", "", "Log type you want (Eg:ContainerLog/HostLog) ")
	logCmd.Flags().StringVar(&logOptions.ContainerName, "container", "", "name of the container ")
	logCmd.Flags().StringVar(&logOptions.PodName, "pod", "", "name of the pod ")
	logCmd.Flags().StringVar(&logOptions.Resource, "resource", "", "command used by the user")
	logCmd.Flags().StringVar(&logOptions.Source, "source", "", "binary used by the system ")
	logCmd.Flags().Uint32Var(&logOptions.Limit, "limit", 0, "number of logs you want to see")
}
