package cmd

import (
	"github.com/kubearmor/kubearmor-client/simulate"
	"github.com/spf13/cobra"
)

var SimulateOptions simulate.Options

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "simulate a kubearmor policy",
	Long:  `simulate a kubearmor policy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// todo
		return nil
	},
}

func init() {
	rootCmd.AddCommand(simulateCmd)
	simulateCmd.Flags().StringVarP(&SimulateOptions.Policy, "policy", "p", "", "Policy file you would like to simulate")
	simulateCmd.Flags().StringVarP(&SimulateOptions.Action, "action", "a", "", "action to be run after simulation : | exec:/bin/bash->/bin/sleep")
	simulateCmd.Flags().StringVarP(&SimulateOptions.Config, "config", "c", "", "kubermor config to be use for simulation")
}
