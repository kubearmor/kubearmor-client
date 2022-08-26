package cmd

import (
	"github.com/kubearmor/kubearmor-client/list"
	"github.com/spf13/cobra"
)

var listOptions list.Options

var listCmd = &cobra.Command{
	Use:   "list-policy",
	Short: "List applied policies applied on a continer/host",
	Long:  "List applied policies applied on a continer/host",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := list.ListPolicies(client, listOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listOptions.Namespace, "namespace", "n", "kube-system", "Specify the namespace")

}
