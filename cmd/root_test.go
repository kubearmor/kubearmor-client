// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor
//go:build darwin || (linux && !windows)

package cmd

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/spf13/cobra"
)

func TestStandaloneCommandsWithoutClusterConfiguration(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing-kubeconfig")
	t.Setenv("KUBECONFIG", missingConfig)
	t.Setenv("DOCKER_HOST", "://invalid")
	originalConfig := k8s.KubeConfig
	k8s.KubeConfig = missingConfig
	t.Cleanup(func() { k8s.KubeConfig = originalConfig })

	for _, tc := range []struct {
		name       string
		args       []string
		standalone bool
	}{
		{"policy add", []string{"vm", "policy", "add", "policy.yaml"}, true},
		{"policy delete", []string{"vm", "policy", "delete", "policy.yaml"}, true},
		{"standalone logs", []string{"logs", "--nonk8s"}, true},
		{"standalone probe", []string{"probe", "--nonk8s"}, true},
		{"standalone install", []string{"install", "--nonk8s"}, true},
		{"standalone uninstall", []string{"uninstall", "--nonk8s"}, true},
		{"cluster logs", []string{"logs"}, false},
		{"cluster probe", []string{"probe"}, false},
		{"cluster install", []string{"install"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, _, err := rootCmd.Find(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if flag := command.Flags().Lookup("nonk8s"); flag != nil {
				value, changed := flag.Value.String(), flag.Changed
				t.Cleanup(func() {
					_ = flag.Value.Set(value)
					flag.Changed = changed
				})
			}
			// Stop before performing an install or contacting an agent. The real
			// Cobra parsing and inherited pre-run still execute for each command.
			reachedRun := errors.New("command reached RunE")
			originalRun := command.RunE
			command.RunE = func(*cobra.Command, []string) error { return reachedRun }
			t.Cleanup(func() {
				command.RunE = originalRun
				rootCmd.SetArgs(nil)
			})
			rootCmd.SetArgs(tc.args)
			err = rootCmd.Execute()
			if tc.standalone {
				if !errors.Is(err, reachedRun) {
					t.Fatalf("standalone command required cluster configuration: %v", err)
				}
			} else if err == nil || errors.Is(err, reachedRun) {
				t.Fatalf("cluster command skipped Kubernetes initialization: %v", err)
			}
		})
	}
}
