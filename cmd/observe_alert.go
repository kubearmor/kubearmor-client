// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/observe"
	"github.com/spf13/cobra"
)

var observeAlertOptions observe.AlertOptions

var observeAlertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Retrieve alert",
	Long:  "Retrieve alert",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := observe.StartObserveAlert(args, observeAlertOptions); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	observeCmd.AddCommand(observeAlertCmd)

	observeAlertCmd.Flags().StringVarP(&observeAlertOptions.Namespace, "namespace", "n", "", "Specify the namespace")
	observeAlertCmd.Flags().StringVar(&observeAlertOptions.Pod, "pod", "", "name of the pod ")
	observeAlertCmd.Flags().StringVar(&observeAlertOptions.Container, "container", "", "name of the container ")
	observeAlertCmd.Flags().BoolVar(&observeAlertOptions.JSON, "json", false, "Flag to print alerts and logs in the JSON format")
	observeAlertCmd.Flags().StringVar(&observeAlertOptions.GRPC, "gRPC", "", "gRPC server information")
}
