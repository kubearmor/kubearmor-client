// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/log"
	"github.com/spf13/cobra"
)

var logOptions log.Options

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "logs",
	Short: "Observe Logs from KubeArmor",
	Long: `Stream and filter KubeArmor alerts, system logs, and policy enforcement events.

This command connects to a running KubeArmor instance (in‑cluster or remote) over gRPC and
continuously prints the events you care about. You can control the connection security,
select which namespaces or containers to watch, and format the output to suit your workflow.

Connection Options:
  • --gRPC <port>          port of the KubeArmor gRPC server (port)
  • --secure                  use mutual TLS for the gRPC connection
  • --tlsCertPath <path>      local directory containing ca.crt, client.crt & client.key
  • --tlsCertProvider <mode>  certificate provisioning: “self” (auto‑generate) or “external”
  • --readCAFromSecret        fetch CA cert from in‑cluster secret (default true)

Output Control:
  • --msgPath <path|stdout|none>   where to write raw event messages
  • --logPath <path|stdout|none>   where to write human‑readable alerts & logs
  • --output, -o <text|json|pretty-json>  choose your output format
  • --json                       shorthand to force JSON output

Filtering:
  • --logFilter <policy|system|all>  type of logs to receive (default “policy” i.e alerts)
  • --namespace, -n <ns>             only show events for this Kubernetes namespace
  • --operation <Process|File|Network> filter by operation type
  • --logType <ContainerLog|HostLog>  filter by log source
  • --container <name>                filter by container name
  • --pod <name>                      filter by pod name
  • --resource <cmd>                  filter by executed command
  • --source <binary>                 filter by system binary path
  • --labels, -l <key=val,…>          label selectors to narrow pods/services
  • --limit <n>                       maximum number of events to print (0 for unlimited)

Examples:
  # Stream all policy‑related events in JSON by connecting to local kubearmor instance:
  karmor logs --gRPC 32767 --json --logFilter policy

  # Watch only file operations in namespace “prod”:
  karmor logs -n prod --operation File --logFilter all

  # Persist alerts to a file in pretty JSON:
  karmor logs --msgPath stdout --logPath /var/log/kubearmor.json --output pretty-json

	Use "karmor logs --help" to see detailed flag descriptions and defaults.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.StopChan = make(chan struct{})
		return log.StartObserver(client, logOptions)
	},
}

func init() {
	rootCmd.AddCommand(logCmd)

	logCmd.Flags().StringVar(&logOptions.GRPC, "gRPC", "", "gRPC server information")
	logCmd.Flags().BoolVar(&logOptions.Secure, "secure", false, "connect to kubearmor on a secure connection")
	logCmd.Flags().StringVar(&logOptions.TlsCertPath, "tlsCertPath", "/var/lib/kubearmor/tls", "path to the ca.crt, client.crt, and client.key if certs are provided locally")
	logCmd.Flags().StringVar(&logOptions.TlsCertProvider, "tlsCertProvider", "self", "{self|external} self: dynamically crete client certificates, external: provide client certificate and key with --tlsCertPath")
	logCmd.Flags().BoolVar(&logOptions.ReadCAFromSecret, "readCAFromSecret", true, "true if ca cert to be read from k8s secret on cluster running kubearmor")
	logCmd.Flags().StringVar(&logOptions.MsgPath, "msgPath", "none", "Output location for messages, {path|stdout|none}")
	logCmd.Flags().StringVar(&logOptions.LogPath, "logPath", "stdout", "Output location for alerts and logs, {path|stdout|none}")
	logCmd.Flags().StringVar(&logOptions.LogFilter, "logFilter", "policy", "Filter for what kinds of alerts and logs to receive, {policy|system|all}")
	logCmd.Flags().BoolVar(&logOptions.JSON, "json", false, "Flag to print alerts and logs in the JSON format")
	logCmd.Flags().StringVarP(&logOptions.Output, "output", "o", "text", "Output format: text, json, or pretty-json")
	logCmd.Flags().StringVarP(&logOptions.Namespace, "namespace", "n", "", "k8s namespace filter")
	logCmd.Flags().StringVar(&logOptions.Operation, "operation", "", "Give the type of the operation (Eg:Process/File/Network)")
	logCmd.Flags().StringVar(&logOptions.LogType, "logType", "", "Log type you want (Eg:ContainerLog/HostLog) ")
	logCmd.Flags().StringVar(&logOptions.ContainerName, "container", "", "name of the container ")
	logCmd.Flags().StringVar(&logOptions.PodName, "pod", "", "name of the pod ")
	logCmd.Flags().StringVar(&logOptions.Resource, "resource", "", "command used by the user")
	logCmd.Flags().StringVar(&logOptions.Source, "source", "", "binary used by the system ")
	logCmd.Flags().Uint32Var(&logOptions.Limit, "limit", 0, "number of logs you want to see")
	logCmd.Flags().StringSliceVarP(&logOptions.Selector, "labels", "l", []string{}, "use the labels to select the endpoints")
}
