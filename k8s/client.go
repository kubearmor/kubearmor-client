// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package k8s contains helper functions to establlish connection and communicate with k8s apis
package k8s

import (
	"context"

	"github.com/rs/zerolog/log"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	kspAPI "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/client/clientset/versioned/typed/security.kubearmor.com/v1"
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
	// kubearmor-ca secret label
	KubeArmorCALabels = map[string]string{
		"kubearmor-app": "kubearmor-ca",
	}
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

func GetKubeArmorCaSecret(client kubernetes.Interface) (string, string) {
	secret, err := client.CoreV1().Secrets("").List(context.Background(), v1.ListOptions{
		LabelSelector: v1.FormatLabelSelector(&v1.LabelSelector{MatchLabels: KubeArmorCALabels}),
	})
	if err != nil {
		log.Error().Msgf("error getting kubearmor ca secret: %v", err)
		return "", ""
	}
	if len(secret.Items) < 1 {
		log.Error().Msgf("no kubearmor ca secret found in the cluster: %v", err)
		return "", ""
	}
	return secret.Items[0].Name, secret.Items[0].Namespace
}
