// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package k8s contains helper functions to establlish connection and communicate with k8s apis
package k8s

import (
	"context"
	"github.com/kubearmor/kubearmor-client/recommend/common"
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

func (k *Client) ListObjects(o common.Options) ([]common.Object, error) {
	labelSelector := v1.FormatLabelSelector(&v1.LabelSelector{MatchLabels: common.LabelArrayToLabelMap(o.Labels)})
	if labelSelector == "<none>" {
		labelSelector = ""
	}
	// CronJobs
	cronJobs, err := k.K8sClientset.BatchV1().CronJobs(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing cronjobs: %v", err)
		return nil, err
	}

	// DaemonSets
	daemonSets, err := k.K8sClientset.AppsV1().DaemonSets(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing daemonsets: %v", err)
		return nil, err
	}

	// Deployments
	deployments, err := k.K8sClientset.AppsV1().Deployments(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing deployments: %v", err)
		return nil, err
	}

	// Jobs
	jobs, err := k.K8sClientset.BatchV1().Jobs(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing jobs: %v", err)
		return nil, err
	}

	// ReplicaSets
	replicaSets, err := k.K8sClientset.AppsV1().ReplicaSets(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing replicasets: %v", err)
		return nil, err
	}

	// StatefulSets
	statefulSets, err := k.K8sClientset.AppsV1().StatefulSets(o.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Msgf("error listing statefulsets: %v", err)
		return nil, err
	}

	var result []common.Object

	for _, cj := range cronJobs.Items {
		var images []string
		for _, container := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, common.Object{
			Name:      cj.Name,
			Namespace: cj.Namespace,
			Labels:    cj.Spec.JobTemplate.Spec.Template.Labels,
			Images:    images,
		})
	}

	for _, ds := range daemonSets.Items {
		var images []string
		for _, container := range ds.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, common.Object{
			Name:      ds.Name,
			Namespace: ds.Namespace,
			Labels:    ds.Spec.Template.Labels,
			Images:    images,
		})
	}

	for _, dp := range deployments.Items {
		var images []string
		for _, container := range dp.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, common.Object{
			Name:      dp.Name,
			Namespace: dp.Namespace,
			Labels:    dp.Spec.Template.Labels,
			Images:    images,
		})
	}

	for _, j := range jobs.Items {
		var images []string
		for _, container := range j.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, common.Object{
			Name:      j.Name,
			Namespace: j.Namespace,
			Labels:    j.Spec.Template.Labels,
			Images:    images,
		})
	}

	for _, rs := range replicaSets.Items {
		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" || owner.Kind == "StatefulSet" || owner.Kind == "DaemonSet" || owner.Kind == "ReplicaSet" {
				isOwned = true
				break
			}
		}
		if isOwned {
			continue
		}

		var images []string
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
		result = append(result, common.Object{
			Name:      rs.Name,
			Namespace: rs.Namespace,
			Labels:    rs.Spec.Template.Labels,
			Images:    images,
		})
	}

	for _, sts := range statefulSets.Items {
		var images []string
		for _, container := range sts.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, common.Object{
			Name:      sts.Name,
			Namespace: sts.Namespace,
			Labels:    sts.Spec.Template.Labels,
			Images:    images,
		})
	}

	log.Printf("+%v", result)
	return result, nil
}
