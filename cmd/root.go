package cmd

import (
	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

var k8sClient *kubernetes.Clientset
var crdClient *ksp.SecurityV1Client

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error

		//Initialise k8sClient for all child commands to inherit
		k8sClient, crdClient, err = k8s.ConnectK8sClient()
		if err != nil {
			log.Error().Msgf("unable to create Kubernetes clients: %w", err.Error())
		}
	},
	Use:   "kubearmor",
	Short: "A CLI Utility to help manage KubeArmor",
	Long: `CLI Utility to help manage KubeArmor
	
KubeArmor is a container-aware runtime security enforcement system that
restricts the behavior (such as process execution, file access, and networking
operation) of containers at the system level.
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
