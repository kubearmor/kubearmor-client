// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package k8s contains helper functions to establlish connection and communicate with k8s apis
package k8s

import (
	"github.com/rs/zerolog/log"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	kspAPI "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed to auth with cloud providers
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Client Structure
type Client struct {
	K8sClientset    kubernetes.Interface
	KSPClientset    ksp.SecurityV1Interface
	APIextClientset apiextensionsclientset.Interface
	RawConfig       clientcmdapi.Config
	Config          *rest.Config
}

var (
	// KubeConfig specifies the path of kubeconfig file
	KubeConfig string
	// ContextName specifies the name of kubeconfig context
	ContextName string
)

// ConnectK8sClient Function
func ConnectK8sClient() (*Client, error) {
	_ = kspAPI.AddToScheme(scheme.Scheme)

	restClientGetter := genericclioptions.ConfigFlags{
		Context:    &ContextName,
		KubeConfig: &KubeConfig,
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

	extClientset, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, err
	}

	return &Client{
		K8sClientset:    clientset,
		KSPClientset:    kspClientset,
		APIextClientset: extClientset,
		RawConfig:       rawConfig,
		Config:          config,
	}, nil
}
