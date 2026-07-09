// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/kubearmor/kubearmor-client/simulate"
	"github.com/spf13/cobra"
)

var simulateOptions simulate.Options

// simulateCmd replays telemetry against a policy YAML without enforcing it.
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Dry-run a KubeArmorPolicy against telemetry (no enforcement)",
	Long: `Replay telemetry events against a policy YAML in pure userspace.

Shows which events would have been blocked, audited, or allowed without
changing cluster enforcement. Safe to run against production clusters.

Telemetry sources:
  • Live: connects to kubearmor-relay (same as karmor logs) for --collect-timeout
  • Offline: --events-file with JSON lines from "karmor logs --logFilter system -o json"

Note: relay streams live events only. --last filters timestamps on events seen during
collection (or in the events file). It does not fetch historical backlog from the relay.

Rules with fromSource are skipped in v1 (not partially evaluated).

Examples:
  karmor simulate --policy my-policy.yaml --namespace default --last 30m
  karmor simulate --policy my-policy.yaml --pod myapp-xxx --last 1h --output json
  karmor simulate --policy my-policy.yaml --events-file ./logs.jsonl --last 24h`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return simulate.Run(k8sClient, simulateOptions)
	},
}

func init() {
	rootCmd.AddCommand(simulateCmd)

	simulateCmd.Flags().StringVar(&simulateOptions.PolicyFile, "policy", "", "Path to KubeArmorPolicy YAML (required)")
	simulateCmd.Flags().StringVarP(&simulateOptions.Namespace, "namespace", "n", "", "Filter events by Kubernetes namespace")
	simulateCmd.Flags().StringVar(&simulateOptions.Pod, "pod", "", "Filter events by pod name")
	simulateCmd.Flags().StringVar(&simulateOptions.Last, "last", "30m", "Time window for events (e.g. 30m, 1h)")
	simulateCmd.Flags().StringVar(&simulateOptions.CollectTimeout, "collect-timeout", "30s", "Max duration for live telemetry collection")
	simulateCmd.Flags().Uint32Var(&simulateOptions.Limit, "limit", 0, "Max events to collect from relay (default 1000 when live)")
	simulateCmd.Flags().StringVar(&simulateOptions.EventsFile, "events-file", "", "JSONL file of system logs (from karmor logs); use - for stdin")
	simulateCmd.Flags().StringVarP(&simulateOptions.Output, "output", "o", "text", "Output format: text or json")
	simulateCmd.Flags().BoolVar(&simulateOptions.ShowAllowed, "show-allowed", false, "Print ALLOWED lines in text output (default: summary only)")
	simulateCmd.Flags().StringVar(&simulateOptions.GRPC, "gRPC", "", "gRPC server address for relay")
	simulateCmd.Flags().BoolVar(&simulateOptions.Secure, "secure", false, "Connect to relay using mutual TLS")
	simulateCmd.Flags().StringVar(&simulateOptions.TlsCertPath, "tlsCertPath", "/var/lib/kubearmor/tls", "Path to ca.crt, client.crt, client.key")
	simulateCmd.Flags().StringVar(&simulateOptions.TlsCertProvider, "tlsCertProvider", "self", "TLS cert provider: self or external")
	simulateCmd.Flags().BoolVar(&simulateOptions.ReadCAFromSecret, "readCAFromSecret", true, "Read CA cert from in-cluster secret")

	_ = simulateCmd.MarkFlagRequired("policy")
}
