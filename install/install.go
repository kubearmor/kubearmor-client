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
	"github.com/fatih/color"
	"sigs.k8s.io/yaml"

	deployments "github.com/kubearmor/KubeArmor/deployments/get"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"

	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Options for karmor install
type Options struct {
	Namespace      string
	InitImage      string
	KubearmorImage string
	Tag            string
	Audit          string
	Block          string
	Visibility     string
	Force          bool
	Local          bool
	Save           bool
	Animation      bool
	Env            envOption
}

type envOption struct {
	Auto        bool
	Environment string
}

var animation bool
var progress int
var cursorcount int
var validEnvironments = []string{"k3s", "microK8s", "minikube", "gke", "bottlerocket", "eks", "docker", "oke", "generic"}

// Checks if passed string is a valid environment
func (env *envOption) CheckAndSetValidEnvironmentOption(envOption string) error {
	if envOption == "" {
		env.Auto = true
		return nil
	}
	for _, v := range validEnvironments {
		if v == envOption {
			env.Environment = envOption
			env.Auto = false
			return nil
		}
	}
	return errors.New("invalid environment passed")
}

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
		fmt.Printf("🥳\tDone Installing KubeArmor\n")
	}
	return 0
}

func printAnimation(msg string, flag bool) int {
	clearLine(90)
	fmt.Printf(msg + "\n")
	if flag {
		progress++
	}
	printBar("\tKubeArmor Installing ", 17)
	return 0
}

func printMessage(msg string, flag bool) int {
	if animation {
		printAnimation(msg, flag)
	}
	return 0
}

func checkPods(c *k8s.Client, o Options) {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("😋\tChecking if KubeArmor pods are running ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		clearLine(90)
		fmt.Printf("\r\tKUBEARMOR pods left to run : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️\tCheck Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🥳\tDone Checking , ALL Services are running!             \n")
			fmt.Printf("⌚️\tExecution Time : %s \n", time.Since(stime))
			break
		}
	}
	fmt.Print("\n🔧\tVerifying KubeArmor functionality (this may take upto a minute) ...")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	for {
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			fmt.Print("⚠️\tFailed verifying KubeArmor functionality ...")
			return
		}
		probeData, _, err := probe.ProbeRunningKubeArmorNodes(c, probe.Options{
			Namespace: o.Namespace,
		})
		if err != nil || len(probeData) == 0 {
			fmt.Printf("\r🔧\tVerifying KubeArmor functionality (this may take upto a minute) ... %s", cursor[cursorcount])
			cursorcount++
			if cursorcount == 4 {
				cursorcount = 0
			}
			continue
		}
		enforcing := true
		for _, k := range probeData {
			if k.ActiveLSM == "" || !k.ContainerSecurity {
				enforcing = false
				break
			}
		}
		if enforcing {
			fmt.Print(color.New(color.FgWhite, color.Bold).Sprint("\n\n\t🛡️\tYour Cluster is Armored Up! \n"))
		} else {
			color.Yellow("\n\n\t⚠️\tKubeArmor is running in Audit mode, only Observability will be available and Policy Enforcement won't be available. \n")
		}
		break
	}

}

func checkTerminatingPods(c *k8s.Client) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("🔴   Checking if KubeArmor pods are stopped ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app", FieldSelector: "status.phase=Running"})
		podno := len(pods.Items)
		clearLine(90)
		fmt.Printf("\rKUBEARMOR pods left to stop : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️  Check Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🔴   Done Checking , ALL Services are stopped!             \n")
			fmt.Printf("⌚️   Termination Time : %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

// K8sInstaller for karmor install
func K8sInstaller(c *k8s.Client, o Options) error {
	animation = o.Animation
	var env string
	if o.Env.Auto {
		env = k8s.AutoDetectEnvironment(c)
		if env == "none" {
			return errors.New("unsupported environment or cluster not configured correctly")
		}
		printMessage("😄\tAuto Detected Environment : "+env, true)
	} else {
		env = o.Env.Environment
		printMessage("😄\tEnvironment : "+env, true)
	}

	var printYAML []interface{}

	kspCRD := CreateCustomResourceDefinition(kspName)
	if !o.Save {
		printMessage("🔥\tCRD "+kspName+"  ", true)
		if _, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &kspCRD, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create CRD %s: %+v", kspName, err)
			}
			printMessage("ℹ️\tCRD "+kspName+" already exists", false)
		}
	} else {
		printYAML = append(printYAML, kspCRD)
	}

	hspCRD := CreateCustomResourceDefinition(hspName)
	if !o.Save {
		printMessage("🔥\tCRD "+hspName+"  ", true)
		if _, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &hspCRD, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create CRD %s: %+v", hspName, err)
			}
			printMessage("ℹ️\tCRD "+hspName+" already exists", false)
		}
	} else {
		printYAML = append(printYAML, hspCRD)
	}

	serviceAccount := deployments.GetServiceAccount(o.Namespace)
	if !o.Save {
		printMessage("💫\tService Account  ", true)
		if _, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Create(context.Background(), serviceAccount, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tService Account already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, serviceAccount)
	}

	clusterRole := deployments.GetClusterRole()
	if !o.Save {
		printMessage("⚙️\tCluster Role  ", true)
		if _, err := c.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), clusterRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tCluster Role already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, clusterRole)
	}

	clusterRoleBinding := deployments.GetClusterRoleBinding(o.Namespace)
	if !o.Save {
		printMessage("⚙️\tCluster Role Bindings  ", true)
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), clusterRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tCluster Role Bindings already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, clusterRoleBinding)
	}

	relayService := deployments.GetRelayService(o.Namespace)
	if !o.Save {
		printMessage("🛡\tKubeArmor Relay Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), relayService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Relay Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, relayService)
	}

	relayDeployment := deployments.GetRelayDeployment(o.Namespace)
	if o.Local {
		relayDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = "IfNotPresent"
	}
	if !o.Save {
		printMessage("🛰\tKubeArmor Relay Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), relayDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Relay Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, relayDeployment)
	}

	daemonset := deployments.GenerateDaemonSet(env, o.Namespace)
	if o.Tag != "" {
		kimg := strings.Split(o.KubearmorImage, ":")
		kimg[1] = o.Tag
		o.KubearmorImage = strings.Join(kimg, ":")

		iimg := strings.Split(o.InitImage, ":")
		iimg[1] = o.Tag
		o.InitImage = strings.Join(iimg, ":")
	}
	daemonset.Spec.Template.Spec.Containers[0].Image = o.KubearmorImage
	daemonset.Spec.Template.Spec.InitContainers[0].Image = o.InitImage
	if o.Local {
		daemonset.Spec.Template.Spec.Containers[0].ImagePullPolicy = "IfNotPresent"
		daemonset.Spec.Template.Spec.InitContainers[0].ImagePullPolicy = "IfNotPresent"
	}
	if o.Audit == "all" || strings.Contains(o.Audit, "file") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultFilePosture=audit")
	}
	if o.Audit == "all" || strings.Contains(o.Audit, "network") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultNetworkPosture=audit")
	}
	if o.Audit == "all" || strings.Contains(o.Audit, "capabilities") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultCapabilitiesPosture=audit")
	}
	if o.Block == "all" || strings.Contains(o.Block, "file") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultFilePosture=block")
	}
	if o.Block == "all" || strings.Contains(o.Block, "network") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultNetworkPosture=block")
	}
	if o.Block == "all" || strings.Contains(o.Block, "capabilities") {
		daemonset.Spec.Template.Spec.Containers[0].Args = append(daemonset.Spec.Template.Spec.Containers[0].Args, "-defaultCapabilitiesPosture=block")
	}
	s := strings.Join(daemonset.Spec.Template.Spec.Containers[0].Args, " ")
	printMessage("🛡\tKubeArmor DaemonSet - Init "+daemonset.Spec.Template.Spec.InitContainers[0].Image+", Container "+daemonset.Spec.Template.Spec.Containers[0].Image+s+"  ", true)

	if !o.Save {
		if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor DaemonSet already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, daemonset)
	}

	caCert, tlsCrt, tlsKey, err := GeneratePki(o.Namespace, deployments.KubeArmorControllerWebhookServiceName)
	if err != nil {
		printMessage("C\tldn't generate TLS secret  ", false)
		return err
	}
	kubearmorControllerTLSSecret := deployments.GetKubeArmorControllerTLSSecret(o.Namespace, caCert.String(), tlsCrt.String(), tlsKey.String())
	if !o.Save {
		printMessage("🛡\tKubeArmor Controller TLS certificates  ", true)
		if _, err := c.K8sClientset.CoreV1().Secrets(o.Namespace).Create(context.Background(), kubearmorControllerTLSSecret, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller TLS certificates already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorControllerTLSSecret)
	}

	controllerServiceAccount := deployments.GetKubeArmorControllerServiceAccount(o.Namespace)
	if !o.Save {
		printMessage("💫\tKubeArmor Controller Service Account  ", true)
		if _, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Create(context.Background(), controllerServiceAccount, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller Service Account already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, controllerServiceAccount)
	}

	controllerClusterRole := deployments.GetKubeArmorControllerClusterRole()
	controllerClusterRoleBinding := deployments.GetKubeArmorControllerClusterRoleBinding(o.Namespace)
	controllerRole := deployments.GetKubeArmorControllerLeaderElectionRole(o.Namespace)
	controllerRoleBinding := deployments.GetKubeArmorControllerLeaderElectionRoleBinding(o.Namespace)
	controllerProxyRole := deployments.GetKubeArmorControllerProxyRole()
	controllerProxyRoleBinding := deployments.GetKubeArmorControllerProxyRoleBinding(o.Namespace)
	controllerMetricsReaderRole := deployments.GetKubeArmorControllerMetricsReaderRole()
	controllerMetricsReaderRoleBinding := deployments.GetKubeArmorControllerMetricsReaderRoleBinding(o.Namespace)
	if !o.Save {
		printMessage("⚙️\tKubeArmor Controller Roles  ", true)
		if _, err := c.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), controllerClusterRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller ClusterRole")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), controllerClusterRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller ClusterRoleBinding")
			}
		}
		if _, err := c.K8sClientset.RbacV1().Roles(o.Namespace).Create(context.Background(), controllerRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller Role")
			}
		}
		if _, err := c.K8sClientset.RbacV1().RoleBindings(o.Namespace).Create(context.Background(), controllerRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller RoleBinding")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), controllerProxyRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller ProxyRole")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), controllerProxyRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller ProxyRoleBinding")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), controllerMetricsReaderRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller MetricsReaderRole")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), controllerMetricsReaderRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Controller MetricsReaderRoleBinding")
			}
		}
	} else {
		printYAML = append(printYAML, controllerClusterRole)
		printYAML = append(printYAML, controllerClusterRoleBinding)
		printYAML = append(printYAML, controllerRole)
		printYAML = append(printYAML, controllerRoleBinding)
		printYAML = append(printYAML, controllerProxyRole)
		printYAML = append(printYAML, controllerProxyRoleBinding)
		printYAML = append(printYAML, controllerMetricsReaderRole)
		printYAML = append(printYAML, controllerMetricsReaderRoleBinding)
	}

	kubearmorControllerDeployment := deployments.GetKubeArmorControllerDeployment(o.Namespace)
	if o.Local {
		kubearmorControllerDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = "IfNotPresent"
	}
	if !o.Save {
		printMessage("🚀\tKubeArmor Controller Deployment  ", true)
		if _, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Create(context.Background(), kubearmorControllerDeployment, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller Deployment already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorControllerDeployment)
	}

	kubearmorControllerMetricsService := deployments.GetKubeArmorControllerMetricsService(o.Namespace)
	if !o.Save {
		printMessage("🚀\tKubeArmor Controller Metrics Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), kubearmorControllerMetricsService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller Metrics Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorControllerMetricsService)
	}

	kubearmorControllerWebhookService := deployments.GetKubeArmorControllerWebhookService(o.Namespace)
	if !o.Save {
		printMessage("🚀\tKubeArmor Controller Webhook Service  ", true)
		if _, err := c.K8sClientset.CoreV1().Services(o.Namespace).Create(context.Background(), kubearmorControllerWebhookService, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller Webhook Service already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorControllerWebhookService)
	}

	kubearmorControllerMutationAdmissionConfiguration := deployments.GetKubeArmorControllerMutationAdmissionConfiguration(o.Namespace, caCert.Bytes())
	if !o.Save {
		printMessage("🤩\tKubeArmor Controller Mutation Admission Registration  ", true)
		if _, err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), kubearmorControllerMutationAdmissionConfiguration, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor Controller Mutation Admission Registration already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorControllerMutationAdmissionConfiguration)
	}

	kubearmorConfigMap := deployments.GetKubearmorConfigMap(o.Namespace, deployments.KubeArmorConfigMapName)
	if o.Visibility != "" && o.Visibility != kubearmorConfigMap.Data["visibility"] {
		kubearmorConfigMap.Data["visibility"] = o.Visibility
	}
	if !o.Save {
		printMessage("🚀\tKubeArmor ConfigMap Creation  ", true)
		if _, err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Create(context.Background(), kubearmorConfigMap, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tKubeArmor ConfigMap already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, kubearmorConfigMap)
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
		printMessage("🤩\tKubeArmor manifest file saved to \033[1m"+s3+"\033[0m", false)

	}
	if animation && !o.Save {
		checkPods(c, o)
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
	animation = o.Animation
	fmt.Print("❌   Mutation Admission Registration ...\n")
	if err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), deployments.KubeArmorControllerMutatingWebhookConfiguration, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Mutation Admission Registration not found ...\n")
	}

	fmt.Print("❌   KubeArmor Controller Webhook Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerWebhookServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Controller Webhook Service not found ...\n")
	}

	fmt.Print("❌   KubeArmor Controller Metrics Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerMetricsServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Controller Metrics Service not found ...\n")
	}

	fmt.Print("❌   KubeArmor Controller Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Controller Deployment not found ...\n")
	}

	fmt.Print("❌   KubeArmor Controller Service Account ...\n")
	if err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerServiceAccountName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Controller Service Account not found ...\n")
	}

	fmt.Print("❌   KubeArmor Controller Roles ...\n")
	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), deployments.KubeArmorControllerClusterRoleName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Cluster Role\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), deployments.KubeArmorControllerClusterRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Cluster Role Bindings\n")
		}
	}

	if err := c.K8sClientset.RbacV1().Roles(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerLeaderElectionRoleName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not foundists") {
			fmt.Print("Error while uninstalKubeArmor Controller Roleling \n")
		}
	}

	if err := c.K8sClientset.RbacV1().RoleBindings(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerLeaderElectionRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Role Bindings\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), deployments.KubeArmorControllerProxyRoleName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Proxy Role\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), deployments.KubeArmorControllerProxyRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Proxy Role Bindings\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), deployments.KubeArmorControllerMetricsReaderRoleName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Metrics Reader Role\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), deployments.KubeArmorControllerMetricsReaderRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling KubeArmor Controller Metrics Reader Role Bindings\n")
		}
	}

	fmt.Print("❌   KubeArmor Controller TLS certificates ...\n")
	if err := c.K8sClientset.CoreV1().Secrets(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerSecretName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Controller TLS certificates not found ...\n")
	}
	fmt.Print("❌   Service Account ...\n")
	if err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Delete(context.Background(), serviceAccountName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Service Account not found ...\n")
	}

	fmt.Print("❌   Cluster Role Bindings ...\n")
	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), clusterRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		// Older CLuster Role Binding Name, keeping it to clean up older kubearmor installations
		if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), kubearmor, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Print("ℹ️   Cluster Role Bindings not found ...\n")
		}
	}

	fmt.Print("❌   Cluster Role ...\n")
	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), deployments.KubeArmorClusterRoleName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Cluster Role not found ...\n")
	}

	fmt.Print("❌   KubeArmor Relay Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), relayServiceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Relay Service not found ...\n")
	}

	fmt.Print("❌   KubeArmor Relay Deployment ...\n")
	if err := c.K8sClientset.AppsV1().Deployments(o.Namespace).Delete(context.Background(), relayDeploymentName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor Relay Deployment not found ...\n")
	}

	fmt.Print("❌   KubeArmor DaemonSet ...\n")
	if err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Delete(context.Background(), kubearmor, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor DaemonSet not found ...\n")
	}

	fmt.Print("❌   KubeArmor ConfigMap ...\n")
	if err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Delete(context.Background(), deployments.KubeArmorConfigMapName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   KubeArmor ConfigMap not found ...\n")
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
	if animation {
		checkTerminatingPods(c)
	}
	return nil
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
