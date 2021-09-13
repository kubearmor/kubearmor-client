package cmd

import (
	"github.com/kubearmor/kubearmor-client/get"
	"github.com/spf13/cobra"
)

var options get.Options

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Display specified resources",
	Long:  `Display specified resources`,
	Run: func(cmd *cobra.Command, args []string) {
		get.Resources(client, options)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&options.Namespace, "all-namespaces", "A", "all", "Namespace for resources")
	getCmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "default", "Namespace for resources")
}
