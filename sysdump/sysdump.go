// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package sysdump collects and dumps information for troubleshooting KubeArmor
package sysdump

import (
	"fmt"
	"os"

	"github.com/kubearmor/kubearmor-client/k8s"
)

type Options struct {
	Filename string
}

func Collect(c *k8s.Client, o Options) error {
	d, err := os.MkdirTemp("", "karmor-sysdump")
	if err != nil {
		return err
	}
	defer os.RemoveAll(d)

	mode := DetectDeploymentMode(c)
	factory := NewCollectorFactory(c, o)
	collector := factory.NewCollector(mode)

	if err := collector.Collect(d); err != nil {
		fmt.Printf("Warning: Some data collection failed: %v\n", err)
	}

	empty, err := IsDirEmpty(d)
	if err != nil {
		return err
	}

	if empty {
		return fmt.Errorf("no data collected")
	}

	sysdumpFile, err := archiveDump(d, o.Filename)
	if err != nil {
		return err
	}

	fmt.Printf("Sysdump at %s\n", sysdumpFile)
	return nil
}
