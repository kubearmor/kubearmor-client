// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	profileclient "github.com/kubearmor/kubearmor-client/profile/Client"
	"github.com/spf13/cobra"
)

// profileCmd represents the profile command
var profilecmd = &cobra.Command{
	Use:   "profile",
	Short: "Profiling of logs",
	Long: `Launch an interactive terminal UI to explore KubeArmor logs by operation type.

The TUI presents separate tabs for File, Network, and Syscall events. Use the Tab key (or click)
to switch between each view. Within any tab, press "e" to export the currently displayed data
to a JSON file (saved in the current directory as ProfileSummary/{operation}.json).

Filtering Options:
  • --gRPC <port>             port of the KubeArmor gRPC server (port)
  • --namespace, -n <namespace>  only show logs from this Kubernetes namespace
  • --pod <pod-name>             only show logs from this pod
  • --container, -c <name>       only show logs from this container

Usage Examples:
  # Start the TUI connecting to a local agent:
  karmor profile --gRPC 32737

  # Filter to namespace "prod" and container "nginx":
  karmor profile -n prod -c nginx 

Controls:
  • Tab / Shift+Tab   switch between File, Network, and Syscall views  
  • e                 export current tab’s data as JSON  
  • Ctrl+C        quit the TUI  
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileclient.Start()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profilecmd)
	profilecmd.Flags().StringVar(&profileclient.ProfileOpts.GRPC, "gRPC", "", "use gRPC")
	profilecmd.Flags().StringVarP(&profileclient.ProfileOpts.Namespace, "namespace", "n", "", "Filter using namespace")
	profilecmd.Flags().StringVar(&profileclient.ProfileOpts.Pod, "pod", "", "Filter using Pod name")
	profilecmd.Flags().StringVarP(&profileclient.ProfileOpts.Container, "container", "c", "", "name of the container ")
	profilecmd.Flags().BoolVar(&profileclient.ProfileOpts.Save, "save", false, "Save Profile data in json format")
}
