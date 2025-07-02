package cmd

import (
	"github.com/kubearmor/kubearmor-client/rotatetls"
	"github.com/spf13/cobra"
)

var (
	namespace string
	rotateCmd = &cobra.Command{
		Use:   "rotate-tls",
		Short: "Rotate webhook controller tls certificates",
		Long:  `Rotate webhook controller tls certificates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rotatetls.RotateTLS(client, namespace); err != nil {
				return err
			}
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(rotateCmd)

	rotateCmd.Flags().StringVarP(&namespace, "namespace", "n", "kubearmor", "Namespace for resources")
}
