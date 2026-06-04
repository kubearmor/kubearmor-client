// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"fmt"
	"time"

	"github.com/kubearmor/kubearmor-client/k8s"
)

// Options holds CLI flags for karmor simulate.
type Options struct {
	PolicyFile       string
	EventsFile       string
	Namespace        string
	Pod              string
	Last             string
	CollectTimeout   string
	Limit            uint32
	Output           string
	ShowAllowed      bool
	GRPC             string
	Secure           bool
	TlsCertPath      string
	TlsCertProvider  string
	ReadCAFromSecret bool
}

// Run executes policy simulation.
func Run(client *k8s.Client, opts Options) error {
	policy, err := LoadPolicy(opts.PolicyFile)
	if err != nil {
		return err
	}

	last, err := ParseDurationFlag(opts.Last)
	if err != nil {
		return fmt.Errorf("invalid --last: %w", err)
	}

	collectTimeout := 30 * time.Second
	if opts.CollectTimeout != "" {
		collectTimeout, err = ParseDurationFlag(opts.CollectTimeout)
		if err != nil {
			return fmt.Errorf("invalid --collect-timeout: %w", err)
		}
	}

	events, err := CollectEvents(client, CollectOptions{
		EventsFile:       opts.EventsFile,
		Namespace:        opts.Namespace,
		Pod:              opts.Pod,
		Last:             last,
		CollectTimeout:   collectTimeout,
		Limit:            opts.Limit,
		GRPC:             opts.GRPC,
		Secure:           opts.Secure,
		TlsCertPath:      opts.TlsCertPath,
		TlsCertProvider:  opts.TlsCertProvider,
		ReadCAFromSecret: opts.ReadCAFromSecret,
	})
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("no events matched filters in the last %s (use --events-file from karmor logs export or longer --collect-timeout)", opts.Last)
	}

	rules := BuildRules(policy.Spec)
	results := make([]EventResult, 0, len(events))
	for _, evt := range events {
		results = append(results, EvaluateEvent(policy, rules, evt))
	}

	report := BuildReport(policy.Metadata.Name, opts.Namespace, opts.Last, results)
	report.EventsFile = opts.EventsFile
	report.LiveCollect = opts.EventsFile == ""

	return PrintReport(report, opts.Output, opts.ShowAllowed)
}
