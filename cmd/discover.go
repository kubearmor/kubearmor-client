// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	profileclient "github.com/kubearmor/kubearmor-client/profile/Client"
	"github.com/spf13/cobra"
)

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover policies from KubeArmor logs",
	Long: `Discover policies from live KubeArmor telemetry in an interactive terminal UI.

The UI presents separate tabs for Process, File, Network, and Syscall events. Use the Tab key
to switch between each view. The --type flag can narrow discovery to a single policy category:
network, file, or process.

Filtering Options:
  • --gRPC <port>               port of the KubeArmor gRPC server (port)
  • --namespace, -n <namespace>  only show logs from this Kubernetes namespace
  • --pod <pod-name>            only show logs from this pod
  • --container, -c <name>      only show logs from this container
  • --type, -t <type>           filter by policy type: network | file | process

Usage Examples:
  # Start the UI connecting to a local agent:
  karmor discover --gRPC 32737

  # Filter to namespace "prod" and container "nginx":
  karmor discover -n prod -c nginx

  # Show only network policy discoveries:
  karmor discover --type network
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return profileclient.Start()
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().StringVar(&profileclient.ProfileOpts.GRPC, "gRPC", "", "use gRPC")
	discoverCmd.Flags().StringVarP(&profileclient.ProfileOpts.Namespace, "namespace", "n", "", "Filter using namespace")
	discoverCmd.Flags().StringVar(&profileclient.ProfileOpts.Pod, "pod", "", "Filter using Pod name")
	discoverCmd.Flags().StringVarP(&profileclient.ProfileOpts.Container, "container", "c", "", "name of the container ")
	discoverCmd.Flags().StringVarP(&profileclient.ProfileOpts.PolicyType, "type", "t", "", "Filter by policy type: network | file | process")
	discoverCmd.Flags().BoolVar(&profileclient.ProfileOpts.Save, "save", false, "Save Profile data in json format")
}
