// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package rotatetls rotates webhook controller tls certificates
package rotatetls

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli/install"
	"github.com/accuknox/accuknox-cli/k8s"
	deployments "github.com/kubearmor/KubeArmor/deployments/get"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// RotateTLS - rotate TLS certs
func RotateTLS(c *k8s.Client, namespace string) error {
	// verify if all needed component are present in the cluster
	fmt.Print("Checking if all needed component are present ...\n")
	if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Background(), deployments.AnnotationsControllerServiceName, metav1.GetOptions{}); err != nil {
		return err
	}

	if _, err := c.K8sClientset.CoreV1().Services(namespace).Get(context.Background(), deployments.AnnotationsControllerServiceName, metav1.GetOptions{}); err != nil {
		return err
	}

	origdeploy, err := c.K8sClientset.AppsV1().Deployments(namespace).Get(context.Background(), deployments.AnnotationsControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if _, err := c.K8sClientset.CoreV1().Secrets(namespace).Get(context.Background(), deployments.KubeArmorControllerSecretName, metav1.GetOptions{}); err != nil {
		return nil
	}

	fmt.Print("All needed component are present ...\n")

	fmt.Print("Generating temporary certificates ...\n")
	suffix, err := getFreeRandSuffix(c, namespace)
	if err != nil {
		fmt.Print("Error generating random suffix ...\n")
		return err
	}
	fmt.Print("Using suffix " + suffix + " for all new temorary resources ...\n")

	serviceName := deployments.AnnotationsControllerServiceName + "-" + suffix
	caCert, tlsCrt, tlsKey, err := install.GeneratePki(namespace, serviceName)
	if err != nil {
		fmt.Print("Could'nt generate TLS secret ...\n")
		return err
	}

	fmt.Print("Installing temporary resources ...\n")
	fmt.Print("KubeArmor Annotation Controller temporary TLS certificates ...\n")
	secret := deployments.GetAnnotationsControllerTLSSecret(namespace, caCert.String(), tlsCrt.String(), tlsKey.String())
	secret.Name = secret.GetName() + "-" + suffix
	if _, err := c.K8sClientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		fmt.Print("KubeArmor Annotation Controller TLS certificates with the same suffix exists ...\n")
		return err
	}

	fmt.Print("KubeArmor Annotation Controller temporary Deployment ...\n")
	deploy := deployments.GetAnnotationsControllerDeployment(namespace)
	deploy.Name = deploy.GetName() + "-" + suffix
	for i, s := range deploy.Spec.Template.Spec.Volumes {
		if s.Name == "cert" {
			s.Secret.SecretName = secret.GetName()
			deploy.Spec.Template.Spec.Volumes[i] = s
			break
		}
	}
	selectLabels := deploy.Spec.Selector.MatchLabels
	selectLabels["kubearmor-app"] = suffix
	deploy.Spec.Selector.MatchLabels = selectLabels
	deploy.Spec.Replicas = origdeploy.Spec.Replicas
	if _, err := c.K8sClientset.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{}); err != nil {
		fmt.Print("KubeArmor Annotation Controller Deployment with the same suffix exists ...\n")
		return err
	}

	fmt.Print("Waiting for the deployment to start, sleeping 15 seconds ...\n")
	time.Sleep(15 * time.Second)

	fmt.Print("KubeArmor Annotation Controller temporary Service ...\n")
	service := deployments.GetAnnotationsControllerService(namespace)
	service.Name = serviceName
	service.Spec.Selector = selectLabels
	if _, err := c.K8sClientset.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		fmt.Print("KubeArmor Annotation Controller Service with the same suffix exists ...\n")
		return err
	}

	fmt.Print("KubeArmor Annotation Controller temporary Mutation Admission Registration ...\n")
	mutation := deployments.GetAnnotationsControllerMutationAdmissionConfiguration(namespace, caCert.Bytes())
	mutation.Name = mutation.Name + "-" + suffix
	mutation.Webhooks[0].ClientConfig.Service.Name = service.GetName()
	if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), mutation, metav1.CreateOptions{}); err != nil {
		fmt.Print("KubeArmor Annotation Controller Mutation Admission Registration with the same suffix exists ...\n")
		return err
	}

	fmt.Print("Temporarily removing the main mutation registation ...\n")
	if err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), deployments.AnnotationsControllerServiceName, metav1.DeleteOptions{}); err != nil {
		return err
	}

	fmt.Print("Generating new certificates ...\n")
	caCert, tlsCrt, tlsKey, err = install.GeneratePki(namespace, deployments.AnnotationsControllerServiceName)
	if err != nil {
		fmt.Print("Could'nt generate TLS secret ...\n")
		return err
	}

	fmt.Print("Updating the main TLS secret ...\n")
	if _, err := c.K8sClientset.CoreV1().Secrets(namespace).Update(context.Background(), deployments.GetAnnotationsControllerTLSSecret(namespace, caCert.String(), tlsCrt.String(), tlsKey.String()), metav1.UpdateOptions{}); err != nil {
		return err
	}

	fmt.Print("Refreshing controller deployment ...\n")
	replicas := int32(0)
	origdeploy.Spec.Replicas = &replicas
	if _, err := c.K8sClientset.AppsV1().Deployments(namespace).Update(context.Background(), origdeploy, metav1.UpdateOptions{}); err != nil {
		return err
	}
	time.Sleep(10 * time.Second)

	origdeploy, err = c.K8sClientset.AppsV1().Deployments(namespace).Get(context.Background(), deployments.AnnotationsControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	origdeploy.Spec.Replicas = deploy.Spec.Replicas
	if _, err := c.K8sClientset.AppsV1().Deployments(namespace).Update(context.Background(), origdeploy, metav1.UpdateOptions{}); err != nil {
		return err
	}
	time.Sleep(10 * time.Second)

	fmt.Print("Restoring main mutation registation ... \n")
	if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), deployments.GetAnnotationsControllerMutationAdmissionConfiguration(namespace, caCert.Bytes()), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Mutation Admission Registration already exists ...\n")
	}

	fmt.Print("Deleting temprary ressources ...\n")
	fmt.Print("Mutation Admission Registration ...\n")
	if err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), mutation.Name, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Mutation Admission Registration not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Service not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Deployment not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller TLS certificates ...\n")
	if err := c.K8sClientset.CoreV1().Secrets(namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller TLS certificates not found ...\n")
	}

	fmt.Print("Certificates were rotated ...\n")
	return nil
}

func getFreeRandSuffix(c *k8s.Client, namespace string) (suffix string, err error) {
	var found bool
	for {
		suffix = rand.String(5)
		found = false
		if _, err = c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Background(), deployments.AnnotationsControllerServiceName+"-"+suffix, metav1.GetOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return "", err
			}
		} else {
			found = true
		}

		if _, err = c.K8sClientset.CoreV1().Services(namespace).Get(context.Background(), deployments.AnnotationsControllerServiceName+"-"+suffix, metav1.GetOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return "", err
			}
		} else {
			found = true
		}

		if _, err = c.K8sClientset.AppsV1().Deployments(namespace).Get(context.Background(), deployments.AnnotationsControllerDeploymentName+"-"+suffix, metav1.GetOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return "", err
			}
		} else {
			found = true
		}

		if _, err = c.K8sClientset.CoreV1().Secrets(namespace).Get(context.Background(), deployments.KubeArmorControllerSecretName+"-"+suffix, metav1.GetOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return "", err
			}
		} else {
			found = true
		}

		if !found {
			break
		}
	}
	return suffix, nil
}
