// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package install is responsible for installation and uninstallation of KubeArmor while autogenerating the configuration
package install

import (
	"context"
	"path/filepath"

	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	deployments "github.com/kubearmor/KubeArmor/deployments/get"
	"github.com/kubearmor/kubearmor-client/k8s"

	"golang.org/x/mod/semver"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Options for karmor install
type Options struct {
	Namespace      string
	KubearmorImage string
	Audit          string
	Force          bool
	Save           bool
	Animation      bool
}

var animation bool
var progress int
var cursorcount int

func clearLine(size int) int {
	for i := 0; i < size; i++ {
		fmt.Printf(" ")
	}
	fmt.Printf("\r")
	return 0
}

func printBar(msg string, total int) int {
	fill := "▇▇▇"
	blank := "   "
	bar := ""
	percent := float64(progress) / float64(total) * 100
	for i := 0; i < progress; i++ {
		bar = bar + fill
	}
	for i := 0; i < total-progress; i++ {
		bar = bar + blank
	}
	fmt.Printf(msg+"[%s] %0.2f%%\r", bar, percent)
	if progress == total {
		time.Sleep(500 * time.Millisecond)
		clearLine(90)
		fmt.Printf("🥳  Done Installing KubeArmor\n")
	}
	return 0
}

func printAnimation(msg string, flag bool) int {
	clearLine(90)
	fmt.Printf(msg + "\n")
	if flag == true {
		progress++
	}
	printBar("    KubeArmor Installing ", 16)
	return 0
}

func printMessage(msg string, flag bool) int {
	if animation {
		printAnimation(msg, flag)
	}
	return 0
}

func checkPods(c *k8s.Client) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("😋  Checking if KubeArmor pods are running ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		clearLine(90)
		fmt.Printf("\rKUBEARMOR pods left to run : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️  Check Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🥳  Done Checking , ALL Services are running!             \n")
			fmt.Printf("⌚️  Execution Time : %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

// K8sInstaller for karmor install
func K8sInstaller(c *k8s.Client, o Options) error {
	env := AutoDetectEnvironment(c)
	if env == "none" {
		return errors.New("unsupported environment or cluster not configured correctly")
	}
	printMessage("😄  Auto Detected Environment : "+env, true)

	var printYAML []interface{}

	kspCRD := CreateCustomResourceDefinition(kspName)
	if !o.Save {
		printMessage("🔥  CRD "+kspName+"  ", true)
		if _, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &kspCRD, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create CRD %s: %+v", kspName, err)
			}
			printMessage("ℹ️   CRD "+kspName+" already exists", false)
		}
	} else {
		printYAML = append(printYAML, kspCRD)
	}

	hspCRD := CreateCustomResourceDefinition(hspName)
	if !o.Save {
		printMessage("🔥  CRD "+hspName+"  ", true)
		if _, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &hspCRD, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create CRD %s: %+v", hspName, err)
			}
			printMessage("ℹ️   CRD "+hspName+" already exists", false)
		}
	} else {
		printYAML = append(printYAML, hspCRD)
	}

	serviceAccount := deployments.GetServiceAccount(o.Namespace)
	if !o.Save {
		printMessage("💫  Service Account  ", true)
		if _, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Create(context.Background(), serviceAccount, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   Service Account already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, serviceAccount)
	}

	clusterRoleBinding := deployments.GetClusterRoleBinding(o.Namespace)
	if !o.Save {
		printMessage("⚙️   Cluster Role Bindings  ", true)
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), clusterRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   Cluster Role Bindings already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, clusterRoleBinding)
	}

	relayService := deployments.GetRelayService(o.Namespace)
	if !o.Save {
		printMessage("🛡   KubeArmor Relay Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), relayService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Relay Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, relayService)
	}

	relayDeployment := deployments.GetRelayDeployment(o.Namespace)
	if !o.Save {
		printMessage("🛰   KubeArmor Relay Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), relayDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Relay Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, relayDeployment)
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
	s := strings.Join(daemonset.Spec.Template.Spec.Containers[0].Args, " ")
	printMessage("🛡   KubeArmor DaemonSet"+daemonset.Spec.Template.Spec.Containers[0].Image+s+"  ", true)

	if !o.Save {
		if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor DaemonSet already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, daemonset)
	}

	policyManagerService := deployments.GetPolicyManagerService(o.Namespace)
	if !o.Save {
		printMessage("🧐  KubeArmor Policy Manager Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), policyManagerService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Policy Manager Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, policyManagerService)
	}

	policyManagerDeployment := deployments.GetPolicyManagerDeployment(o.Namespace)
	if !o.Save {
		printMessage("🤖  KubeArmor Policy Manager Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), policyManagerDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Policy Manager Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, policyManagerDeployment)
	}

	hostPolicyManagerService := deployments.GetHostPolicyManagerService(o.Namespace)
	if !o.Save {
		printMessage("😃  KubeArmor Host Policy Manager Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), hostPolicyManagerService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Host Policy Manager Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, hostPolicyManagerService)
	}

	hostPolicyManagerDeployment := deployments.GetHostPolicyManagerDeployment(o.Namespace)
	if !o.Save {
		printMessage("🛡   KubeArmor Host Policy Manager Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), hostPolicyManagerDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Host Policy Manager Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, hostPolicyManagerDeployment)
	}

	caCert, tlsCrt, tlsKey, err := GeneratePki(o.Namespace, deployments.AnnotationsControllerServiceName)
	if err != nil {
		printMessage("Couldn't generate TLS secret  ", false)
		return err
	}
	annotationsControllerTLSSecret := deployments.GetAnnotationsControllerTLSSecret(o.Namespace, caCert.String(), tlsCrt.String(), tlsKey.String())
	if !o.Save {
		printMessage("🛡   KubeArmor Annotation Controller TLS certificates  ", true)
		if _, err := c.K8sClientset.CoreV1().Secrets(o.Namespace).Create(context.Background(), annotationsControllerTLSSecret, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Annotation Controller TLS certificates already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, annotationsControllerTLSSecret)
	}

	annotationsControllerDeployment := deployments.GetAnnotationsControllerDeployment(o.Namespace)
	if !o.Save {
		printMessage("🚀  KubeArmor Annotation Controller Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), annotationsControllerDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Annotation Controller Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, annotationsControllerDeployment)
	}

	annotationsControllerService := deployments.GetAnnotationsControllerService(o.Namespace)
	if !o.Save {
		printMessage("🚀  KubeArmor Annotation Controller Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), annotationsControllerService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Annotation Controller Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, annotationsControllerService)
	}

	annotationsControllerMutationAdmissionConfiguration := deployments.GetAnnotationsControllerMutationAdmissionConfiguration(o.Namespace, caCert.Bytes())
	if !o.Save {
		printMessage("🤩  KubeArmor Annotation Controller Mutation Admission Registration  ", true)
		if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), annotationsControllerMutationAdmissionConfiguration, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️   KubeArmor Annotation Controller Mutation Admission Registration already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, annotationsControllerMutationAdmissionConfiguration)
	}

	// Save the Generated YAML to file
	if o.Save {
		currDir, err := os.Getwd()
		if err != nil {
			return err
		}

		f, err := os.Create(filepath.Clean(path.Join(currDir, "kubearmor.yaml")))
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Printf("Error closing file: %s\n", err)
			}
		}()

		for _, o := range printYAML {
			if err := writeToYAML(f, o); err != nil {
				return err
			}
		}

		err = f.Sync()
		if err != nil {
			return err
		}
		s3 := f.Name()
		printMessage("🤩  KubeArmor manifest file saved to \033[1m"+s3+"\033[0m", false)

	}
	if animation {
		checkPods(c)
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

	if o.Force {
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

		removeAnnotations(c)
	}

	return nil
}

// AutoDetectEnvironment detect the environment for a given k8s context
func AutoDetectEnvironment(c *k8s.Client) (name string) {
	env := "none"

	contextName := c.RawConfig.CurrentContext
	clusterContext, exists := c.RawConfig.Contexts[contextName]
	if !exists {
		return env
	}

	clusterName := clusterContext.Cluster
	cluster := c.RawConfig.Clusters[clusterName]
	nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if len(nodes.Items) <= 0 {
		return env
	}
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

func writeToYAML(f *os.File, o interface{}) error {
	// Use "clarketm/json" to marshal so as to support zero values of structs with omitempty
	j, err := json.Marshal(o)
	if err != nil {
		return err
	}

	object, err := yaml.JSONToYAML(j)
	if err != nil {
		return err
	}

	_, err = f.Write(append([]byte("---\n"), object...))
	if err != nil {
		return err
	}

	return nil
}
