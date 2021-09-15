// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package k8s

import (
	"github.com/rs/zerolog/log"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed to auth with cloud providers
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Client struct {
	K8sClientset kubernetes.Interface
	KSPClientset ksp.SecurityV1Interface
	RawConfig    clientcmdapi.Config
}

func ConnectK8sClient() (*Client, error) {
	var kubeconfig string
	var contextName string

	restClientGetter := genericclioptions.ConfigFlags{
		Context:    &contextName,
		KubeConfig: &kubeconfig,
	}
	rawKubeConfigLoader := restClientGetter.ToRawKubeConfigLoader()

	config, err := rawKubeConfigLoader.ClientConfig()
	if err != nil {
		return nil, err
	}

	rawConfig, err := rawKubeConfigLoader.RawConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, err
	}

	kspClientset, err := ksp.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, err
	}

	return &Client{
		K8sClientset: clientset,
		KSPClientset: kspClientset,
		RawConfig:    rawConfig,
	}, nil
}
