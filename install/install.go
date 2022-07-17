// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	deployments "github.com/kubearmor/KubeArmor/deployments/get"
	"github.com/kubearmor/kubearmor-client/k8s"

	"golang.org/x/mod/semver"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Options for karmor install
type Options struct {
	Namespace      string
	KubearmorImage string
	Audit          string
	Force          bool
}

// K8sInstaller for karmor install
func K8sInstaller(c *k8s.Client, o Options) error {
	env := autoDetectEnvironment(c)
	if env == "none" {
		return errors.New("unsupported environment or cluster not configured correctly")
	}
	fmt.Printf("Auto Detected Environment : %s\n", env)

	fmt.Printf("CRD %s ...\n", kspName)
	if _, err := CreateCustomResourceDefinition(c, kspName); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Printf("CRD %s already exists ...\n", kspName)
	}

	fmt.Printf("CRD %s ...\n", hspName)
	if _, err := CreateCustomResourceDefinition(c, hspName); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Printf("CRD %s already exists ...\n", hspName)
	}

	fmt.Print("Service Account ...\n")
	if _, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Create(context.Background(), deployments.GetServiceAccount(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("Service Account already exists ...\n")
	}

	fmt.Print("Cluster Role Bindings ...\n")
	if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), deployments.GetClusterRoleBinding(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("Cluster Role Bindings already exists ...\n")
	}

	fmt.Print("KubeArmor Relay Service ...\n")
	if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), deployments.GetRelayService(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Relay Service already exists ...\n")
	}

	fmt.Print("KubeArmor Relay Deployment ...\n")
	if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployments.GetRelayDeployment(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Relay Deployment already exists ...\n")
	}

	daemonset := deployments.GenerateDaemonSet(env, o.Namespace)
	daemonset.Spec.Template.Spec.Containers[0].Image = o.KubearmorImage
	if o.Audit == "all" || strings.Contains(o.Audit, "file") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultFilePosture=audit")
	}
	if o.Audit == "all" || strings.Contains(o.Audit, "network") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultNetworkPosture=audit")
	}
	if o.Audit == "all" || strings.Contains(o.Audit, "capabilities") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultCapabilitiesPosture=audit")
	}
	fmt.Printf("KubeArmor DaemonSet %s %v...\n", daemonset.Spec.Template.Spec.Containers[0].Image, daemonset.Spec.Template.Spec.Containers[0].Args)

	if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor DaemonSet already exists ...\n")
	}

	fmt.Print("KubeArmor Policy Manager Service ...\n")
	if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), deployments.GetPolicyManagerService(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Policy Manager Service already exists ...\n")
	}

	fmt.Print("KubeArmor Policy Manager Deployment ...\n")
	if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployments.GetPolicyManagerDeployment(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Policy Manager Deployment already exists ...\n")
	}

	fmt.Print("KubeArmor Host Policy Manager Service ...\n")
	if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), deployments.GetHostPolicyManagerService(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Host Policy Manager Service already exists ...\n")
	}

	fmt.Print("KubeArmor Host Policy Manager Deployment ...\n")
	if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployments.GetHostPolicyManagerDeployment(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Host Policy Manager Deployment already exists ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller TLS certificates ...\n")
	caCert, tlsCrt, tlsKey, err := GeneratePki(o.Namespace, deployments.AnnotationsControllerServiceName)
	if err != nil {
		fmt.Print("Could'nt generate TLS secret ...\n")
		return err
	}
	if _, err := c.K8sClientset.CoreV1().Secrets(o.Namespace).Create(context.Background(), deployments.GetAnnotationsControllerTLSSecret(o.Namespace, caCert.String(), tlsCrt.String(), tlsKey.String()), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller TLS certificates already exists ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Deployment ...\n")
	if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployments.GetAnnotationsControllerDeployment(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Deployment already exists ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Service ...\n")
	if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), deployments.GetAnnotationsControllerService(o.Namespace), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Service already exists ...\n")
	}
	fmt.Print("KubeArmor Annotation Controller Mutation Admission Registration ...\n")
	if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), deployments.GetAnnotationsControllerMutationAdmissionConfiguration(o.Namespace, caCert.Bytes()), metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Mutation Admission Registration already exists ...\n")
	}
	return nil
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func removeDeployAnnotations(c *k8s.Client, dep *v1.Deployment) {
	cnt := 0
	patchPayload := []patchStringValue{}
	for k, v := range dep.Spec.Template.ObjectMeta.Annotations {
		if strings.Contains(k, "kubearmor") || strings.Contains(v, "kubearmor") {
			k = strings.Replace(k, "/", "~1", -1)
			payload := patchStringValue{
				Op:   "remove",
				Path: "/spec/template/metadata/annotations/" + k,
			}
			patchPayload = append(patchPayload, payload)
			cnt++
		}
	}

	if cnt > 0 {
		fmt.Printf("\tRemoving kubearmor annotations from deployment=%s namespace=%s\n",
			dep.ObjectMeta.Name, dep.ObjectMeta.Namespace)
		payloadBytes, _ := json.Marshal(patchPayload)
		_, err := c.K8sClientset.AppsV1().Deployments(dep.ObjectMeta.Namespace).Patch(context.Background(), dep.ObjectMeta.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			fmt.Printf("failed to remove annotation ns:%s, deployment:%s, err:%s\n",
				dep.ObjectMeta.Namespace, dep.ObjectMeta.Name, err.Error())
			return
		}
	}
}

func removeAnnotations(c *k8s.Client) {
	deps, err := c.K8sClientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("could not get deployments")
		return
	}
	fmt.Println("Force removing the annotations. Deployments might be restarted.")
	for _, dep := range deps.Items {
		dep := dep // this is added to handle "Implicit Memory Aliasing..."
		removeDeployAnnotations(c, &dep)
	}
}

// K8sUninstaller for karmor uninstall
func K8sUninstaller(c *k8s.Client, o Options) error {
	fmt.Print("Mutation Admission Registration ...\n")
	if err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), deployments.AnnotationsControllerServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Mutation Admission Registration not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), deployments.AnnotationsControllerServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Service not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), deployments.AnnotationsControllerDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller Deployment not found ...\n")
	}

	fmt.Print("KubeArmor Annotation Controller TLS certificates ...\n")
	if err := c.K8sClientset.CoreV1().Secrets(o.Namespace).Delete(context.Background(), deployments.AnnotationsControllerSecretName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Annotation Controller TLS certificates not found ...\n")
	}
	fmt.Print("Service Account ...\n")
	if err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Delete(context.Background(), serviceAccountName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Service Account not found ...\n")
	}

	fmt.Print("Cluster Role Bindings ...\n")
	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), clusterRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Cluster Role Bindings not found ...\n")
	}

	fmt.Print("KubeArmor Relay Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), relayServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Relay Service not found ...\n")
	}

	fmt.Print("KubeArmor Relay Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), relayDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Relay Deployment not found ...\n")
	}

	fmt.Print("KubeArmor DaemonSet ...\n")
	if err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Delete(context.Background(), kubearmor, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor DaemonSet not found ...\n")
	}

	fmt.Print("KubeArmor Policy Manager Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), policyManagerServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Policy Manager Service not found ...\n")
	}

	fmt.Print("KubeArmor Policy Manager Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), policyManagerDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Policy Manager Deployment not found ...\n")
	}

	fmt.Print("KubeArmor Host Policy Manager Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), hostPolicyManagerServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Host Policy Manager Service not found ...\n")
	}

	fmt.Print("KubeArmor Host Policy Manager Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), hostPolicyManagerDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("KubeArmor Host Policy Manager Deployment not found ...\n")
	}

	fmt.Printf("CRD %s ...\n", kspName)
	if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), kspName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Printf("CRD %s not found ...\n", kspName)
	}

	fmt.Printf("CRD %s ...\n", hspName)
	if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), hspName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Printf("CRD %s not found ...\n", hspName)
	}

	if o.Force {
		removeAnnotations(c)
	}

	return nil
}

func autoDetectEnvironment(c *k8s.Client) (name string) {
	env := "none"

	contextName := c.RawConfig.CurrentContext
	clusterContext, exists := c.RawConfig.Contexts[contextName]
	if !exists {
		return env
	}

	clusterName := clusterContext.Cluster
	cluster := c.RawConfig.Clusters[clusterName]
	nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	containerRuntime := nodes.Items[0].Status.NodeInfo.ContainerRuntimeVersion
	nodeImage := nodes.Items[0].Status.NodeInfo.OSImage

	// Detecting Environment based on cluster name and context or OSImage
	if clusterName == "minikube" || contextName == "minikube" {
		env = "minikube"
		return env
	}

	if strings.HasPrefix(clusterName, "microk8s-") || contextName == "microk8s" {
		env = "microk8s"
		return env
	}

	if strings.HasPrefix(clusterName, "gke_") {
		env = "gke"
		return env
	}

	if strings.Contains(nodeImage, "Bottlerocket") {
		env = "bottlerocket"
		return env
	}

	if strings.HasSuffix(clusterName, ".eksctl.io") || strings.HasSuffix(cluster.Server, "eks.amazonaws.com") {
		env = "eks"
		return env
	}

	// Environment is Self Managed K8s, checking container runtime and it's version

	if strings.Contains(containerRuntime, "k3s") {
		env = "k3s"
		return env
	}

	s := strings.Split(containerRuntime, "://")
	runtime := s[0]
	version := "v" + s[1]

	if runtime == "docker" && semver.Compare(version, "v18.9") >= 0 {
		env = "docker"
		return env
	}

	if runtime == "cri-o" {
		env = "oke"
		return env
	}

	if (runtime == "docker" && semver.Compare(version, "v19.3") >= 0) || runtime == "containerd" {
		env = "generic"
		return env
	}

	return env
}
