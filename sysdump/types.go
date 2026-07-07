// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"github.com/kubearmor/kubearmor-client/k8s"
)

type DeploymentMode int

const (
	ModeUnknown DeploymentMode = iota
	ModeKubernetes
	ModeSystemd
	ModeProcess
)

type Collector interface {
	Collect(dumpDir string) error
}

type CollectorFactory struct {
	k8sClient *k8s.Client
	options   Options
}

func NewCollectorFactory(c *k8s.Client, o Options) *CollectorFactory {
	return &CollectorFactory{
		k8sClient: c,
		options:   o,
	}
}

func DetectDeploymentMode(c *k8s.Client) DeploymentMode {
	if isSystemdMode() {
		return ModeSystemd
	}
	if isKubernetesMode(c) {
		return ModeKubernetes
	}
	return ModeProcess
}

func (cf *CollectorFactory) NewCollector(mode DeploymentMode) Collector {
	switch mode {
	case ModeKubernetes:
		return NewK8sCollector(cf.k8sClient, cf.options)
	case ModeSystemd:
		return NewSystemdCollector(cf.options)
	default:
		return NewProcessCollector(cf.k8sClient, cf.options)
	}
}
