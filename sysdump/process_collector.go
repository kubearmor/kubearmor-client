// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"fmt"
	"io"
	"os"
	"path"

	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"
)

type ProcessCollector struct {
	k8sClient *k8s.Client
	options   Options
}

func NewProcessCollector(c *k8s.Client, o Options) *ProcessCollector {
	return &ProcessCollector{
		k8sClient: c,
		options:   o,
	}
}

func (pc *ProcessCollector) Collect(d string) error {
	fmt.Println("KubeArmor running in process mode or not detected")

	if err := writeCommandOutput(path.Join(d, "system-info.txt"), "uname", "-a"); err != nil {
		kg.Warnf("Failed to get system info: %v\n", err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}

	err = probe.PrintProbeResultCmd(pc.k8sClient, probe.Options{
		Namespace: "",
		Full:      false,
		Output:    "no-color",
		GRPC:      "",
		Writer:    writer,
	})
	if err != nil {
		writer.Close()
		kg.Warnf("Failed to get probe data: %v\n", err)
		return nil
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	out, _ := io.ReadAll(reader)
	if len(out) > 0 {
		err = writeToFile(path.Join(d, "karmor-probe.txt"), string(out))
		if err != nil {
			return err
		}
	}

	return nil
}
