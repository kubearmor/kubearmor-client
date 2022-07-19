// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/observe"
	"github.com/spf13/cobra"
)

var observeOptions observe.Options

var observeCmd = &cobra.Command{
	Use:   "observe",
	Short: "Retrieve observabilities data",
	Long:  "Retrieve observabilities data",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := observe.StartObserve(args, observeOptions); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(observeCmd)

	observeCmd.Flags().StringVarP(&observeOptions.Operation, "operation", "o", "", "the type of the operation (Eg:Process/File/Network)")
	observeCmd.Flags().StringVarP(&observeOptions.Namespace, "namespace", "n", "default", "the desired namespace")
	observeCmd.Flags().BoolVarP(&observeOptions.AllNamespace, "all", "A", false, "the desired namespace")
	observeCmd.Flags().StringVarP(&observeOptions.Labels, "labels", "l", "", "the labels of the resource")
	observeCmd.Flags().BoolVar(&observeOptions.ShowLabels, "show-labels", false, "display the labels")
	observeCmd.Flags().StringVar(&observeOptions.Since, "since", "", "duration of observabilities data to be displayed")
	observeCmd.Flags().StringVar(&observeOptions.CustomColumns, "custom-columns", "", "the custom columns of the output")
}
