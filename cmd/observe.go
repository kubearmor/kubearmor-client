// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"errors"

	"github.com/kubearmor/kubearmor-client/observe"
	"github.com/spf13/cobra"
)

var observeTelemetryOptions observe.TelemetryOptions

var observeCmd = &cobra.Command{
	Use:   "observe",
	Short: "Retrieve observabilities data",
	Long:  "Retrieve observabilities data",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires an operation to observe as argument, valid operations are [file|network|process|syscall|alert]")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := observe.StartObserveTelemetry(args, observeTelemetryOptions); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(observeCmd)

	observeCmd.Flags().StringVarP(&observeTelemetryOptions.Namespace, "namespace", "n", "default", "the desired namespace")
	observeCmd.Flags().BoolVarP(&observeTelemetryOptions.AllNamespace, "all", "A", false, "the desired namespace")
	observeCmd.Flags().StringVarP(&observeTelemetryOptions.Labels, "labels", "l", "", "the labels of the resource")
	observeCmd.Flags().BoolVar(&observeTelemetryOptions.ShowLabels, "show-labels", false, "display the labels")
	observeCmd.Flags().StringVar(&observeTelemetryOptions.Since, "since", "", "duration of observabilities data to be displayed")
	observeCmd.Flags().StringVar(
		&observeTelemetryOptions.CustomColumns,
		"custom-columns",
		"",
		"the custom columns of the output, the valid keys are process_name, type, data, host_name, labels, container_image, ppid, cluster_name, parent_process_name, host_ppid, operation, result, created_at, namespace_name, container_name, host_pid, source, resource, pod_name, container_id, pid",
	)
	observeCmd.Flags().StringVar(&observeTelemetryOptions.GRPC, "gRPC", "", "gRPC server information")
}
