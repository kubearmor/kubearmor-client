// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package install is responsible for installation and uninstallation of KubeArmor while autogenerating the configuration
package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/clarketm/json"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"sigs.k8s.io/yaml"

	deployments "github.com/kubearmor/KubeArmor/deployments/get"
	operatorClient "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/client/clientset/versioned"
	"github.com/kubearmor/kubearmor-client/hacks"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"
	"github.com/kubearmor/kubearmor-client/utils"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"

	Operatorv1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/api/operator.kubearmor.com/v1"
)

// Options for karmor install
type Options struct {
	Namespace              string
	InitImage              string
	KubearmorImage         string
	ControllerImage        string
	OperatorImage          string
	RelayImage             string
	ImageRegistry          string
	Audit                  string
	Block                  string
	HostAudit              string
	HostBlock              string
	Visibility             string
	HostVisibility         string
	Force                  bool
	Local                  bool
	Save                   bool
	Verify                 bool
	Legacy                 bool
	SkipDeploy             bool
	KubeArmorTag           string
	KubeArmorRelayTag      string
	KubeArmorControllerTag string
	KubeArmorOperatorTag   string
	PreserveUpstream       bool
	Env                    envOption
	AlertThrottling        bool
	MaxAlertPerSec         int
	ThrottleSec            int
	AnnotateExisting       bool
	NonK8s                 bool
	VmMode                 utils.VMMode
	ComposeCmd             string
	ComposeVersion         string
	ImagePullPolicy        string
	SecureContainers       bool
}

type envOption struct {
	Auto        bool
	Environment string
}

var (
	verify            bool
	progress          int
	cursorcount       int
	validEnvironments = []string{"k0s", "k3s", "microK8s", "minikube", "gke", "bottlerocket", "eks", "docker", "oke", "generic"}
)

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

func setPosture(option string, value string, posture *Operatorv1.PostureType, optionName string) {
	if option == "all" || strings.Contains(option, value) {
		*posture = Operatorv1.PostureType(optionName)
	}
}

func getOperatorCR(o Options) (*Operatorv1.KubeArmorConfig, string) {
	ns := o.Namespace
	var defaultFilePosture Operatorv1.PostureType
	var defaultCapabilitiesPosture Operatorv1.PostureType
	var defaultNetworkPosture Operatorv1.PostureType

	// Setting default images to stable version
	if len(o.KubearmorImage) == 0 {
		o.KubearmorImage = utils.DefaultKubeArmorImage + ":" + utils.DefaultDockerTag
	}
	o.KubearmorImage = updateImageTag(o.KubearmorImage, o.KubeArmorTag)
	if len(o.InitImage) == 0 {
		o.InitImage = utils.DefaultKubeArmorInitImage + ":" + utils.DefaultDockerTag
	}
	o.InitImage = updateImageTag(o.InitImage, o.KubeArmorTag)
	o.ControllerImage = updateImageTag(o.ControllerImage, o.KubeArmorControllerTag)
	o.RelayImage = updateImageTag(o.RelayImage, o.KubeArmorRelayTag)

	if o.ImageRegistry != "" {
		o.KubearmorImage = UpdateImageRegistry(o.ImageRegistry, o.KubearmorImage, o.PreserveUpstream)
		o.InitImage = UpdateImageRegistry(o.ImageRegistry, o.InitImage, o.PreserveUpstream)
		o.ControllerImage = UpdateImageRegistry(o.ImageRegistry, o.ControllerImage, o.PreserveUpstream)
		o.RelayImage = UpdateImageRegistry(o.ImageRegistry, o.RelayImage, o.PreserveUpstream)
	}

	var imagePullPolicy string = "Always"
	if o.Local {
		imagePullPolicy = "IfNotPresent"
	}

	setPosture(o.Audit, "file", &defaultFilePosture, "audit")
	setPosture(o.Audit, "network", &defaultNetworkPosture, "audit")
	setPosture(o.Audit, "capabilities", &defaultCapabilitiesPosture, "audit")
	setPosture(o.Block, "file", &defaultFilePosture, "block")
	setPosture(o.Block, "network", &defaultNetworkPosture, "block")
	setPosture(o.Block, "capabilities", &defaultCapabilitiesPosture, "block")

	postureSettings := ""
	if defaultCapabilitiesPosture != "" {
		postureSettings += " -defaultCapabilitiesPosture: " + string(defaultCapabilitiesPosture)
	}
	if defaultFilePosture != "" {
		postureSettings += " -defaultFilePosture: " + string(defaultFilePosture)
	}
	if defaultNetworkPosture != "" {
		postureSettings += " -defaultNetworkPosture: " + string(defaultNetworkPosture)
	}

	return &Operatorv1.KubeArmorConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeArmorConfig",
			APIVersion: "operator.kubearmor.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubearmorconfig-default",
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubearmorconfig",
				"app.kubernetes.io/instance":   "kubearmorconfig-default",
				"app.kubernetes.io/part-of":    "kubearmoroperator",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/created-by": "kubearmoroperator",
			},
		},
		Spec: Operatorv1.KubeArmorConfigSpec{
			DefaultFilePosture:         defaultFilePosture,
			DefaultCapabilitiesPosture: defaultCapabilitiesPosture,
			DefaultNetworkPosture:      defaultNetworkPosture,
			DefaultVisibility:          o.Visibility,
			KubeArmorImage: Operatorv1.ImageSpec{
				Image:           o.KubearmorImage,
				ImagePullPolicy: imagePullPolicy,
			},
			KubeArmorInitImage: Operatorv1.ImageSpec{
				Image:           o.InitImage,
				ImagePullPolicy: imagePullPolicy,
			},
			KubeArmorRelayImage: Operatorv1.ImageSpec{
				Image:           o.RelayImage,
				ImagePullPolicy: imagePullPolicy,
			},
			KubeArmorControllerImage: Operatorv1.ImageSpec{
				Image:           o.ControllerImage,
				ImagePullPolicy: imagePullPolicy,
			},
			EnableStdOutLogs:   false,
			EnableStdOutAlerts: false,
			EnableStdOutMsgs:   false,
			AlertThrottling:    o.AlertThrottling,
			MaxAlertPerSec:     int(o.MaxAlertPerSec),
			ThrottleSec:        int(o.ThrottleSec),
		},
	}, postureSettings
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
	if verify {
		if flag {
			progress++
		}
		printBar("\tKubeArmor Installing ", 17)
	}
	return 0
}

func printMessage(msg string, flag bool) int {
	printAnimation(msg, flag)
	return 0
}

func checkPods(c *k8s.Client, o Options, i bool) {
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("⌚️\tThis may take a couple of minutes                     \n")
	// Check snitch completion only for install
	if i {
		for {
			pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app=kubearmor-snitch", FieldSelector: "status.phase==Succeeded"})
			podno := len(pods.Items)
			fmt.Printf("\rℹ️\tWaiting for KubeArmor Snitch to run: %s", cursor[cursorcount])
			cursorcount++
			if cursorcount == len(cursor) {
				cursorcount = 0
			}
			if podno > 0 {
				fmt.Printf("\r🥳\tKubeArmor Snitch Deployed!             \n")
				break
			}
			if !otime.After(time.Now()) {
				fmt.Printf("\r⌚️\tCheck Incomplete due to Time-Out!                     \n")
				break
			}
		}
	}
	for {
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app=kubearmor", FieldSelector: "status.phase==Running"})
		podno := len(pods.Items)
		fmt.Printf("\rℹ️\tWaiting for Daemonset to start: %s", cursor[cursorcount])
		cursorcount++
		if cursorcount == len(cursor) {
			cursorcount = 0
		}
		if podno > 0 {
			fmt.Printf("\r🥳\tKubeArmor Daemonset Deployed!             \n")
			fmt.Printf("\r🥳\tDone Checking , ALL Services are running!             \n")
			fmt.Printf("⌚️\tExecution Time : %s \n", time.Since(stime))
			break
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️\tCheck Incomplete due to Time-Out!                     \n")
			break
		}
	}
	fmt.Print("\n🔧\tVerifying KubeArmor functionality (this may take upto a minute)...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	defer cancel()

	for {
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			fmt.Print("⚠️\tFailed verifying KubeArmor functionality")
			return
		}
		probeData, _, err := probe.ProbeRunningKubeArmorNodes(c, probe.Options{
			Namespace: o.Namespace,
		})
		if err != nil || len(probeData) == 0 {
			fmt.Printf("\r🔧\tVerifying KubeArmor functionality (this may take upto a minute) %s", cursor[cursorcount])
			cursorcount++
			if cursorcount == len(cursor) {
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
	// add annotation for apparmor
	if !o.AnnotateExisting {
		nodeList, err := getApparmorNodes(c)
		if err != nil {
			fmt.Printf("\n⚠️\tError fetching apparmor nodes %s", err.Error())
		} else if len(nodeList) > 0 {
			fmt.Printf("⚠️\tWARNING: Pre-existing pods will not be annotated. Policy enforcement for pre-existing pods on the following AppArmor nodes will not work:\n")
			for i, node := range nodeList {
				fmt.Printf("\t	➤ Node %d: %s", i+1, node)
			}
			fmt.Printf("\n\t•To annotate existing pods using controller, run:")
			fmt.Printf("\n\t	➤ karmor uninstall followed by karmor install --annotateExisting=true")
			fmt.Printf("\n\t• Alternatively, if you prefer manual control, you can restart your deployments yourself using:")
			fmt.Printf("\n\t	➤ kubectl rollout restart deployment <deployment> -n <namespace>\n")
		}
	}
}

func checkPodsLegacy(c *k8s.Client, o Options) {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("😋\tChecking if KubeArmor pods are running...\n")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)

	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		fmt.Printf("\r\tKubeArmor pods left to run : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == len(cursor) {
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
	fmt.Print("\n🔧\tVerifying KubeArmor functionality (this may take upto a minute)...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	defer cancel()

	for {
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			fmt.Print("⚠️\tFailed verifying KubeArmor functionality")
			return
		}
		probeData, _, err := probe.ProbeRunningKubeArmorNodes(c, probe.Options{
			Namespace: o.Namespace,
		})
		if err != nil || len(probeData) == 0 {
			fmt.Printf("\r🔧\tVerifying KubeArmor functionality (this may take upto a minute) %s", cursor[cursorcount])
			cursorcount++
			if cursorcount == len(cursor) {
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

func checkTerminatingPods(c *k8s.Client, ns string) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("🔄  Checking if KubeArmor pods are stopped...\n")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app", FieldSelector: "status.phase=Running"})
		podno := len(pods.Items)
		fmt.Printf("\rKubeArmor pods left to stop : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == len(cursor) {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️  Check incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🔴  Done Checking; all services are stopped!             \n")
			fmt.Printf("⌚️  Termination Time: %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

// UpdateImageRegistry will update the registry address of the image
func UpdateImageRegistry(registry, image string, preserveUpstream bool) string {
	registry = strings.Trim(registry, "/")
	if preserveUpstream {
		return strings.Join([]string{
			registry,
			image,
		}, "/")
	}
	_, name, tag, hash := hacks.GetImageDetails(image)
	if hash != "" {
		return registry + "/" + name + ":" + hash
	}
	return registry + "/" + name + ":" + tag
}

func updateImageTag(image, tag string) string {
	// check if the image constains a tag
	// if not, set it to latest
	if !strings.Contains(image, ":") {
		image = image + ":latest"
	}

	if tag == "" {
		return image
	}

	i := strings.Split(image, ":")
	i[len(i)-1] = tag
	return strings.Join(i, ":")
}

// K8sInstaller for karmor install
func K8sLegacyInstaller(c *k8s.Client, o Options) error {

	// Setting default images to stable version
	if len(o.KubearmorImage) == 0 {
		o.KubearmorImage = utils.DefaultKubeArmorImage + ":" + utils.DefaultDockerTag
	}
	o.KubearmorImage = updateImageTag(o.KubearmorImage, o.KubeArmorTag)
	if len(o.InitImage) == 0 {
		o.InitImage = utils.DefaultKubeArmorInitImage + ":" + utils.DefaultDockerTag
	}
	o.InitImage = updateImageTag(o.InitImage, o.KubeArmorTag)
	o.ControllerImage = updateImageTag(o.ControllerImage, o.KubeArmorControllerTag)
	o.RelayImage = updateImageTag(o.RelayImage, o.KubeArmorRelayTag)

	if o.ImageRegistry != "" {
		o.KubearmorImage = UpdateImageRegistry(o.ImageRegistry, o.KubearmorImage, o.PreserveUpstream)
		o.InitImage = UpdateImageRegistry(o.ImageRegistry, o.InitImage, o.PreserveUpstream)
		o.ControllerImage = UpdateImageRegistry(o.ImageRegistry, o.ControllerImage, o.PreserveUpstream)
		o.RelayImage = UpdateImageRegistry(o.ImageRegistry, o.RelayImage, o.PreserveUpstream)
	}

	verify = o.Verify
	var env string
	if o.Env.Auto {
		env = k8s.AutoDetectEnvironment(c)
		if env == "none" {
			if o.Save {
				printMessage("⚠️\tNo env provided with \"--save\", setting env to  \"generic\"", true)
				env = "generic"
			} else {
				return errors.New("unsupported environment or cluster not configured correctly")
			}
		}
		printMessage("😄\tAuto Detected Environment : "+env, true)
	} else {
		env = o.Env.Environment
		printMessage("😄\tEnvironment : "+env, true)
	}

	// Check if the namespace already exists
	ns := o.Namespace
	if !o.Save {
		if _, err := c.K8sClientset.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{}); err != nil {
			// Create namespace when doesn't exist
			printMessage("🚀\tCreating namespace "+ns+"  ", true)
			newns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}
			if _, err := c.K8sClientset.CoreV1().Namespaces().Create(context.Background(), &newns, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create namespace %s: %+v", ns, err)
			}
		}
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

	relayServiceAccount := deployments.GetRelayServiceAccount(o.Namespace)
	if !o.Save {
		printMessage("💫\tService Account  ", true)
		if _, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Create(context.Background(), relayServiceAccount, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
			printMessage("ℹ️\tRelay Service Account already exists  ", false)
		}
	} else {
		printYAML = append(printYAML, relayServiceAccount)
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

	relayClusterRole := deployments.GetRelayClusterRole()
	RelayClusterRoleBinding := deployments.GetRelayClusterRoleBinding(o.Namespace)
	if !o.Save {
		printMessage("⚙️\tKubeArmor Relay Roles  ", true)
		if _, err := c.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), relayClusterRole, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Relay ClusterRole")
			}
		}
		if _, err := c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), RelayClusterRoleBinding, metav1.CreateOptions{}); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Print("Error while installing KubeArmor Relay ClusterRoleBinding")
			}
		}
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
	relayDeployment.Spec.Template.Spec.Containers[0].Image = o.RelayImage
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

	args := strings.Join(daemonset.Spec.Template.Spec.Containers[0].Args, " ")
	printMessage("🛡\tKubeArmor DaemonSet - Init "+daemonset.Spec.Template.Spec.InitContainers[0].Image+", Container "+daemonset.Spec.Template.Spec.Containers[0].Image+"  "+args+"  ", true)

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
		printMessage("Couldn't generate TLS secret  ", false)
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
	} else {
		printYAML = append(printYAML, controllerClusterRole)
		printYAML = append(printYAML, controllerClusterRoleBinding)
		printYAML = append(printYAML, controllerRole)
		printYAML = append(printYAML, controllerRoleBinding)
	}

	kubearmorControllerDeployment := deployments.GetKubeArmorControllerDeployment(o.Namespace)
	// This deployment contains two containers, we should probably get rid of the kube-proxy pod
	for i := range kubearmorControllerDeployment.Spec.Template.Spec.Containers {
		if kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].Name == "manager" {
			kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].Image = o.ControllerImage
		} else {
			if o.ImageRegistry != "" {
				kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].Image = UpdateImageRegistry(o.ImageRegistry, kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].Image, o.PreserveUpstream)
			}
		}
		kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].ImagePullPolicy = "IfNotPresent"
	}
	if o.Local {
		for i := range kubearmorControllerDeployment.Spec.Template.Spec.Containers {
			kubearmorControllerDeployment.Spec.Template.Spec.Containers[i].ImagePullPolicy = "IfNotPresent"
		}
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
	if verify && !o.Save {
		checkPodsLegacy(c, o)
	}
	return nil
}

func actionConfigInit(ns string, settings *cli.EnvSettings) *action.Configuration {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), ns, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		fmt.Println("failed to initialize Helm configuration: " + err.Error())
		return nil
	}
	return actionConfig
}

func writeHelmManifests(manifests string, filename string, printYAML []interface{}, kubearmorConfig *Operatorv1.KubeArmorConfig) error {
	currDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	printYAML = append(printYAML, kubearmorConfig)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Clean(path.Join(currDir, filename)))
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

	file, _ := os.OpenFile("kubearmor.yaml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	// Write the string to the file
	_, err = file.WriteString(manifests + "\n")
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		log.Fatal(err)
	}
	s3 := f.Name()
	fmt.Println("🤩\tKubeArmor manifest file saved to \033[1m" + s3 + "\033[0m")
	return nil
}

func getOperatorConfig(o Options) map[string]interface{} {
	var operatorImagePullPolicy string = "Always"
	if o.Local {
		operatorImagePullPolicy = "IfNotPresent"
	}

	updateImg := updateImageTag(o.OperatorImage, o.KubeArmorOperatorTag)
	i := strings.Split(updateImg, ":")
	operatorImage := i[0]
	operatorImageTag := i[len(i)-1]
	if o.ImageRegistry != "" {
		operatorImage = o.ImageRegistry + "/" + operatorImage
	}
	return map[string]interface{}{
		"kubearmorOperator": map[string]interface{}{
			"image": map[string]interface{}{
				"repository": operatorImage,
				"tag":        operatorImageTag,
			},
			"imagePullPolicy":  operatorImagePullPolicy,
			"annotateExisting": o.AnnotateExisting,
		},
	}
}

// K8sInstaller using operator for karmor
func K8sInstaller(c *k8s.Client, o Options) error {
	var printYAML []interface{}
	ns := o.Namespace
	releaseName := "kubearmor-operator"
	kubearmorConfig, postureSettings := getOperatorCR(o)
	values := getOperatorConfig(o)
	settings := cli.New()

	actionConfig := actionConfigInit(ns, settings)

	entry := &repo.Entry{
		Name: "kubearmor",
		URL:  "https://kubearmor.github.io/charts",
	}

	r, err := repo.NewChartRepository(entry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create ChartRepository: %w", err)
	}

	r.CachePath = settings.RepositoryCache

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index file: %w", err)
	}

	var repoFile repo.File
	repoFile.Update(entry)
	if err := repoFile.WriteFile(settings.RepositoryConfig, 0o644); err != nil {
		return fmt.Errorf("failed to write repository file: %w", err)
	}

	client := action.NewUpgrade(actionConfig)
	client.Namespace = ns
	client.Timeout = 5 * time.Minute
	client.Install = false
	if o.Save {
		client.DryRun = true
	}

	chartPath, err := client.ChartPathOptions.LocateChart("kubearmor/kubearmor-operator", settings)
	if err != nil {
		return fmt.Errorf("failed to locate Helm chart: %w", err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load Helm chart: %w", err)
	}
	log.SetOutput(io.Discard)
	upgradeInstaller, err := client.Run(releaseName, chartRequested, values)
	log.SetOutput(os.Stdout)
	if err != nil {
		client.Install = true
		if client.Install {
			clientInstall := action.NewInstall(actionConfig)
			clientInstall.ReleaseName = releaseName
			clientInstall.Namespace = ns
			clientInstall.Timeout = 5 * time.Minute
			clientInstall.CreateNamespace = true
			if o.Save {
				clientInstall.DryRun = true
				clientInstall.ClientOnly = true
			}

			chartPath, err := clientInstall.ChartPathOptions.LocateChart("kubearmor/kubearmor-operator", settings)
			if err != nil {
				return fmt.Errorf("failed to locate Helm chart path: %w", err)
			}

			chartRequested, err := loader.Load(chartPath)
			if err != nil {
				return fmt.Errorf("failed to load Helm chart: %w", err)
			}

			log.SetOutput(io.Discard)
			installRunner, err := clientInstall.Run(chartRequested, values)
			log.SetOutput(os.Stdout)
			if o.Save {
				return writeHelmManifests(installRunner.Manifest, "kubearmor.yaml", printYAML, kubearmorConfig)
			}
			if err != nil {
				return fmt.Errorf("failed to install Helm chart: %w", err)
			}
			fmt.Println("🛡\tInstalled helm release : " + releaseName)
		}
	} else {
		if o.Save {
			return writeHelmManifests(upgradeInstaller.Manifest, "kubearmor.yaml", printYAML, kubearmorConfig)
		}
		fmt.Println("🛡\tUpgraded Kubearmor helm release : " + releaseName)
	}

	operatorClientSet, err := operatorClient.NewForConfig(c.Config)
	if err != nil {
		return fmt.Errorf("failed to create operator client: %w", err)
	}

	if o.SkipDeploy {
		yamlData, err := yaml.Marshal(kubearmorConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal kubearmorConfig: %w", err)
		}
		fmt.Println("ℹ️\tSkipping KubeArmorConfig deployment")
		fmt.Printf("--- kubearmorConfig dump:\n%s\n\n", string(yamlData))
	} else {
		if _, err := operatorClientSet.OperatorV1().KubeArmorConfigs(ns).Create(context.Background(), kubearmorConfig, metav1.CreateOptions{}); apierrors.IsAlreadyExists(err) {
			existingConfig, err := operatorClientSet.OperatorV1().KubeArmorConfigs(ns).Get(context.Background(), kubearmorConfig.Name, metav1.GetOptions{})
			if err != nil {
				fmt.Println("Failed to get existing KubeArmorConfig: %w", err)
			}
			kubearmorConfig.ResourceVersion = existingConfig.ResourceVersion
			if _, err := operatorClientSet.OperatorV1().KubeArmorConfigs(ns).Update(context.Background(), kubearmorConfig, metav1.UpdateOptions{}); err != nil {
				fmt.Println("Failed to update KubeArmorConfig: %w", err)
			} else {
				fmt.Println("😄\tKubeArmorConfig updated" + postureSettings)
			}
		} else {
			fmt.Println("😄\tKubeArmorConfig created" + postureSettings)
		}
	}

	if o.Verify && !o.Save && !o.SkipDeploy {
		checkPods(c, o, client.Install)
	}

	return nil
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func removeAnnotationsFromResource(c *k8s.Client, resource interface{}, resourceType, namespace, name string) error {
	cnt := 0
	patchPayload := []patchStringValue{}
	var annotations map[string]string
	var labels map[string]string

	switch r := resource.(type) {
	case *appsv1.Deployment:
		annotations = r.Spec.Template.Annotations
		labels = r.Labels
	case *appsv1.ReplicaSet:
		annotations = r.Spec.Template.Annotations
		labels = r.Labels
	case *appsv1.StatefulSet:
		annotations = r.Spec.Template.Annotations
		labels = r.Labels
	case *appsv1.DaemonSet:
		annotations = r.Spec.Template.Annotations
		labels = r.Labels
	case *batchv1.Job:
		annotations = r.Spec.Template.Annotations
		labels = r.Labels
	case *batchv1.CronJob:
		annotations = r.Spec.JobTemplate.Spec.Template.Annotations
		labels = r.Labels
	default:
		return fmt.Errorf("unsupported resource type: %T", r)
	}

	// returns if it's kubearmor related resources like relay, controller
	if _, exists := labels["kubearmor-app"]; exists {
		return nil
	}

	for k, v := range annotations {
		if strings.Contains(k, "kubearmor") || strings.Contains(v, "kubearmor") {
			k = strings.Replace(k, "/", "~1", -1)
			path := "/spec/template/metadata/annotations/" + k
			if resourceType == "cronjob" {
				path = "/spec/jobTemplate/spec/template/metadata/annotations/" + k
			}
			payload := patchStringValue{
				Op:   "remove",
				Path: path,
			}
			patchPayload = append(patchPayload, payload)
			cnt++
		}
	}

	if cnt > 0 {
		fmt.Printf("Removing kubearmor annotations from %s=%s namespace=%s\n", resourceType, name, namespace)
		payloadBytes, _ := json.Marshal(patchPayload)
		var err error
		switch resourceType {
		case "deployment":
			_, err = c.K8sClientset.AppsV1().Deployments(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		case "replicaset":
			rs := resource.(*appsv1.ReplicaSet)
			replicas := *rs.Spec.Replicas

			_, err = c.K8sClientset.AppsV1().ReplicaSets(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("err : %v", err)
			}

			// To update the annotations we need to restart the replicaset,we scale it down and scale it back up
			patchData := []byte(fmt.Sprintf(`{"spec": {"replicas": 0}}`))
			_, err = c.K8sClientset.AppsV1().ReplicaSets(namespace).Patch(context.Background(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				return err
			}
			time.Sleep(2 * time.Second)
			patchData2 := []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, replicas))
			_, err = c.K8sClientset.AppsV1().ReplicaSets(namespace).Patch(context.Background(), name, types.StrategicMergePatchType, patchData2, metav1.PatchOptions{})
			if err != nil {
				return err
			}
		case "statefulset":
			_, err = c.K8sClientset.AppsV1().StatefulSets(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		case "daemonset":
			_, err = c.K8sClientset.AppsV1().DaemonSets(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		case "job":
			_, err = c.K8sClientset.BatchV1().Jobs(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		case "cronjob":
			_, err = c.K8sClientset.BatchV1().CronJobs(namespace).Patch(context.Background(), name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		default:
			return fmt.Errorf("unsupported resource type: %s", resourceType)
		}
		if err != nil {
			fmt.Printf("failed to remove annotation ns:%s, %s:%s, err:%s\n", namespace, resourceType, name, err.Error())
			return err
		}
	}
	return nil
}

func removeAnnotations(c *k8s.Client) {
	fmt.Println("Force removing the annotations. Deployments might be restarted.")
	deployments, err := c.K8sClientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing deployments: %v\n", err)
	}
	for _, dep := range deployments.Items {
		dep := dep // this is added to handle "Implicit Memory Aliasing..."
		if err := removeAnnotationsFromResource(c, &dep, "deployment", dep.Namespace, dep.Name); err != nil {
			fmt.Printf("Error removing annotations from deployment %s in namespace %s: %v\n", dep.Name, dep.Namespace, err)
		}
	}

	replicaSets, err := c.K8sClientset.AppsV1().ReplicaSets("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing replica sets: %v\n", err)
	}
	for _, rs := range replicaSets.Items {
		rs := rs
		if err := removeAnnotationsFromResource(c, &rs, "replicaset", rs.Namespace, rs.Name); err != nil {
			fmt.Printf("Error removing annotations from replicaset %s in namespace %s: %v\n", rs.Name, rs.Namespace, err)
		}
	}

	statefulSets, err := c.K8sClientset.AppsV1().StatefulSets("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing stateful sets: %v\n", err)
	}
	for _, sts := range statefulSets.Items {
		sts := sts
		if err := removeAnnotationsFromResource(c, &sts, "statefulset", sts.Namespace, sts.Name); err != nil {
			fmt.Printf("Error removing annotations from statefulset %s in namespace %s: %v\n", sts.Name, sts.Namespace, err)
		}
	}

	daemonSets, err := c.K8sClientset.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing daemon sets: %v\n", err)
	}
	for _, ds := range daemonSets.Items {
		ds := ds
		if err := removeAnnotationsFromResource(c, &ds, "daemonset", ds.Namespace, ds.Name); err != nil {
			fmt.Printf("Error removing annotations from daemonset %s in namespace %s: %v\n", ds.Name, ds.Namespace, err)
		}
	}

	jobs, err := c.K8sClientset.BatchV1().Jobs("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing jobs: %v\n", err)
	}
	for _, job := range jobs.Items {
		job := job
		if err := removeAnnotationsFromResource(c, &job, "job", job.Namespace, job.Name); err != nil {
			fmt.Printf("Error removing annotations from job %s in namespace %s: %v\n", job.Name, job.Namespace, err)
		}
	}

	cronJobs, err := c.K8sClientset.BatchV1().CronJobs("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing cron jobs: %v\n", err)
	}
	for _, cronJob := range cronJobs.Items {
		cronJob := cronJob
		if err := removeAnnotationsFromResource(c, &cronJob, "cronjob", cronJob.Namespace, cronJob.Name); err != nil {
			fmt.Printf("Error removing annotations from cronjob %s in namespace %s: %v\n", cronJob.Name, cronJob.Namespace, err)
		}
	}

	// removing annotations at pod level whose owner's are not being annotated
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
	}

	for _, pod := range pods.Items {
		pod := pod // this is added to handle "Implicit Memory Aliasing..."
		restartingPod := false
		if _, exists := pod.ObjectMeta.Labels["kubearmor-app"]; exists {
			continue
		}

		for k := range pod.ObjectMeta.Annotations {
			if strings.Contains(k, "container.apparmor.security.beta.kubernetes.io/") {
				restartingPod = true
			}
		}

		if !restartingPod {
			continue
		}

		fmt.Printf("Removing kubearmor annotations from pod=%s namespace=%s\n", pod.Name, pod.Namespace)
		if pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 {
			err := c.K8sClientset.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("Error deleting pod: %v\n", err)
			}
		} else {
			gracePeriodSeconds := int64(0)
			if err := c.K8sClientset.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}); err != nil {
				fmt.Printf("Error deleting pod: %v\n", err)
			}

			// clean the pre-polutated attributes
			pod.ResourceVersion = ""

			for k := range pod.ObjectMeta.Annotations {
				if strings.Contains(k, "container.apparmor.security.beta.kubernetes.io/") {
					delete(pod.ObjectMeta.Annotations, k)
				}
			}

			// re-create the pod
			if _, err := c.K8sClientset.CoreV1().Pods(pod.Namespace).Create(context.Background(), &pod, metav1.CreateOptions{}); err != nil {
				fmt.Printf("Error creating pod: %v\n", err)
			}
		}
	}
}

func listPods(c *k8s.Client) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"No.", "Pod Name", "Namespace"})
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
	}
	cnt := 0
	for _, pod := range pods.Items {
		pod := pod // this is added to handle "Implicit Memory Aliasing..."
		if _, exists := pod.ObjectMeta.Labels["kubearmor-app"]; exists {
			continue
		}
		for k := range pod.ObjectMeta.Annotations {
			if strings.Contains(k, "container.apparmor.security.beta.kubernetes.io/") {
				cnt++
				table.Append([]string{fmt.Sprintf("%d", cnt), pod.Name, pod.Namespace})
				break
			}
		}
	}
	if cnt != 0 {
		fmt.Println("ℹ️   Following pods will get restarted with karmor uninstall --force: \n")
		table.Render()
	}
}

func K8sLegacyUninstaller(c *k8s.Client, o Options) error {
	verify = o.Verify
	fmt.Print("🗑️  KubeArmor Services\n")
	servicesList, err := c.K8sClientset.CoreV1().Services(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	if len(servicesList.Items) == 0 {
		fmt.Printf("    ℹ️  KubeArmor Services not found\n")
	} else {
		for _, ms := range servicesList.Items {
			fmt.Printf("    ❌  Service: %s removed\n", ms.Name)
			if err := c.K8sClientset.CoreV1().Services(ms.Namespace).Delete(context.Background(), ms.Name, metav1.DeleteOptions{}); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					fmt.Print(err)
					continue
				}
				fmt.Printf("ℹ️  %s service not found\n", ms.Name)
			}
		}
	}

	fmt.Print("💨  Service Accounts\n")
	serviceAccountList, err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	serviceAccountNames := []string{serviceAccountName, deployments.RelayServiceAccountName, deployments.KubeArmorControllerServiceAccountName, operatorServiceAccountName}

	// for backward-compatibility - where ServiceAccounts are not KubeArmor labelled
	if len(serviceAccountList.Items) == 0 {
		serviceAccountList, err = c.K8sClientset.CoreV1().ServiceAccounts("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Print(err)
		}
		if len(serviceAccountList.Items) == 0 {
			fmt.Print("    ℹ️  ServiceAccount not found\n")
		} else {
			for _, sa := range serviceAccountList.Items {
				// check for the services by serviceaccount names
				// once we have labels in all the objects this can be removed
				if slices.Contains(serviceAccountNames, sa.Name) {
					if err := c.K8sClientset.CoreV1().ServiceAccounts(sa.Namespace).Delete(context.Background(), sa.Name, metav1.DeleteOptions{}); err != nil {
						if !strings.Contains(err.Error(), "not found") {
							fmt.Print(err)
							continue
						}
						fmt.Printf("ℹ️  ServiceAccount %s can't be removed\n", sa.Name)
						continue
					}
					fmt.Printf("    ❌  ServiceAccount %s removed\n", sa.Name)

				}
			}
		}
	} else {
		for _, sa := range serviceAccountList.Items {
			if err := c.K8sClientset.CoreV1().ServiceAccounts(sa.Namespace).Delete(context.Background(), sa.Name, metav1.DeleteOptions{}); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					fmt.Print(err)
					continue
				}
				fmt.Printf("ℹ️  ServiceAccount %s not found\n", sa.Name)
			}
		}
	}

	commonUninstall(c, o)

	fmt.Print("👻  KubeArmor Controller TLS certificates\n")
	tlsCertificatesList, err := c.K8sClientset.CoreV1().Secrets(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	for _, tlsCert := range tlsCertificatesList.Items {
		if err := c.K8sClientset.CoreV1().Secrets(tlsCert.Namespace).Delete(context.Background(), tlsCert.Name, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				fmt.Print(err)
				continue
			}
			fmt.Print("ℹ️  KubeArmor Controller TLS certificates not found\n")
			continue
		}
		fmt.Printf("    ❌  KubeArmor Controller TLS certificate %s removed\n", tlsCert.Name)
	}

	fmt.Print("👻  KubeArmor ConfigMap\n")
	configmapList, err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	for _, cm := range configmapList.Items {
		if err := c.K8sClientset.CoreV1().ConfigMaps(cm.Namespace).Delete(context.Background(), cm.Name, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				fmt.Print(err)
				continue
			}
			fmt.Print("ℹ️  KubeArmor ConfigMap not found\n")
			continue
		}
		fmt.Printf("    ❌  ConfigMap %s removed\n", cm.Name)
	}

	fmt.Print("👻  KubeArmor DaemonSet\n")
	daemonsetList, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	if len(daemonsetList.Items) == 0 {
		fmt.Print("    ℹ️  KubeArmor Daemonset not found\n")
	} else {
		for _, ds := range daemonsetList.Items {
			if err := c.K8sClientset.AppsV1().DaemonSets(ds.Namespace).Delete(context.Background(), ds.Name, metav1.DeleteOptions{}); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					fmt.Print(err)
					continue
				}
				fmt.Print("ℹ️  KubeArmor DaemonSet not found\n")
				continue
			}
			fmt.Printf("    ❌  KubeArmor DaemonSet %s removed\n", ds.Name)
		}
	}

	fmt.Print("👻  KubeArmor Deployments\n")
	kaDeploymentsList, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	if len(kaDeploymentsList.Items) == 0 {
		fmt.Print("    ℹ️  KubeArmor Deployments not found\n")
	} else {
		for _, d := range kaDeploymentsList.Items {
			if err := c.K8sClientset.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{}); err != nil {
				fmt.Printf("    ℹ️  Error while uninstalling KubeArmor Deployment %s : %s\n", d.Name, err.Error())
				continue
			}
			fmt.Printf("    ❌  KubeArmor Deployment %s removed\n", d.Name)
		}
	}

	if !o.Force {
		fmt.Println("ℹ️   Please use karmor uninstall --force in order to clean up kubearmor completely including it's annotations and CRDs")
		listPods(c)
	} else {
		operatorClientSet, err := operatorClient.NewForConfig(c.Config)
		if err != nil {
			return fmt.Errorf("failed to create operator clientset: %w", err)
		}

		fmt.Printf("CR kubearmorconfig-default\n")
		if err := operatorClientSet.OperatorV1().KubeArmorConfigs(o.Namespace).Delete(context.Background(), "kubearmorconfig-default", metav1.DeleteOptions{}); apierrors.IsNotFound(err) {
			fmt.Printf("CR %s not found\n", kocName)
		}

		fmt.Printf("CRD %s\n", kocName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), kocName, metav1.DeleteOptions{}); apierrors.IsNotFound(err) {
			fmt.Printf("CRD %s not found\n", kocName)
		}

		fmt.Printf("CRD %s\n", kspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), kspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", kspName)
		}

		fmt.Printf("CRD %s\n", cspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), cspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", cspName)
		}

		fmt.Printf("CRD %s\n", hspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), hspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", hspName)
		}

		removeAnnotations(c)
	}

	if verify {
		checkTerminatingPods(c, o.Namespace)
	}
	return nil
}

// K8sUninstaller for karmor uninstall
func K8sUninstaller(c *k8s.Client, o Options) error {
	var ns string
	settings := cli.New()

	actionConfig := actionConfigInit("", settings)
	statusClient := action.NewStatus(actionConfig)
	res, err := statusClient.Run("kubearmor-operator")
	if err != nil {
		fmt.Println("ℹ️   Helm release not found. Switching to legacy uninstaller.")
		return err
	}
	ns = res.Namespace

	fmt.Printf("ℹ️   Uninstalling KubeArmor\n")
	actionConfig = actionConfigInit(ns, settings)
	client := action.NewUninstall(actionConfig)
	client.Timeout = 5 * time.Minute
	client.DeletionPropagation = "background"

	log.SetOutput(io.Discard)
	_, err = client.Run("kubearmor-operator")
	log.SetOutput(os.Stdout)
	if err != nil {
		fmt.Println("ℹ️   Error uninstalling through Helm. Switching to legacy uninstaller.")
		return err
	}
	commonUninstall(c, o)

	if !o.Force {
		fmt.Println("ℹ️   Resources not managed by helm/Global Resources are not cleaned up. Please use karmor uninstall --force if you want complete cleanup.")
		listPods(c)
	} else {
		operatorClientSet, err := operatorClient.NewForConfig(c.Config)
		if err != nil {
			return fmt.Errorf("failed to create operator clientset: %w", err)
		}

		fmt.Printf("❌  Removing CR kubearmorconfig-default\n")
		if err := operatorClientSet.OperatorV1().KubeArmorConfigs(ns).Delete(context.Background(), "kubearmorconfig-default", metav1.DeleteOptions{}); apierrors.IsNotFound(err) {
			fmt.Printf("CR %s not found\n", kocName)
		}

		fmt.Printf("❌  Removing CRD %s\n", kocName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), kocName, metav1.DeleteOptions{}); apierrors.IsNotFound(err) {
			fmt.Printf("CRD %s not found\n", kocName)
		}

		fmt.Printf("❌  Removing CRD %s\n", kspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), kspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", kspName)
		}

		fmt.Printf("❌  Removing CRD %s\n", cspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), cspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", cspName)
		}

		fmt.Printf("❌  Removing CRD %s\n", hspName)
		if err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), hspName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return err
			}
			fmt.Printf("CRD %s not found\n", hspName)
		}

		removeAnnotations(c)
	}

	fmt.Println("❌  KubeArmor resources removed")

	if o.Verify {
		checkTerminatingPods(c, ns)
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

// this function stores the common elements for legacy and helm-based uninstallation
func commonUninstall(c *k8s.Client, o Options) {

	fmt.Print("🗑️  Mutation Admission Registration\n")
	if err := c.K8sClientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), deployments.KubeArmorControllerMutatingWebhookConfiguration, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print(err)
		}
		fmt.Print("    ℹ️  Mutation Admission Registration not found\n")
	}

	fmt.Print("💨  Cluster Roles\n")
	clusterRoleList, err := c.K8sClientset.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	clusterRoleNames := []string{
		KubeArmorClusterRoleName,
		RelayClusterRoleName,
		KubeArmorOperatorManageControllerClusterRoleName,
		KubeArmorOperatorManageClusterRoleName,
		KubeArmorSnitchClusterRoleName,
		KubeArmorOperatorClusterRoleName,
		KubeArmorControllerClusterRoleName,
		KubeArmorControllerProxyClusterRoleName,
	}
	// for backward-compatibility - where ClusterRoles are not KubeArmor labelled
	if len(clusterRoleList.Items) == 0 {
		clusterRoleList, err = c.K8sClientset.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Print(err)
		}
		for _, cr := range clusterRoleList.Items {
			// check for clusterroles by names
			// once we have labels in all the objects this can be removed
			if slices.Contains(clusterRoleNames, cr.Name) {
				if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), cr.Name, metav1.DeleteOptions{}); err != nil {
					if !strings.Contains(err.Error(), "not found") {
						fmt.Print(err)
						continue
					}
					fmt.Printf("ℹ️  ClusterRole %s cant' be removed\n", cr.Name)
					continue
				}
				fmt.Printf("    ❌  ClusterRole %s removed\n", cr.Name)

			}
		}
	} else {
		for _, cr := range clusterRoleList.Items {
			if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), cr.Name, metav1.DeleteOptions{}); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					fmt.Print(err)
					continue
				}
				fmt.Printf("ℹ️  ClusterRole %s not found\n", cr.Name)
			}
		}
	}

	fmt.Print("💨  Cluster Role Bindings\n")
	clusterRoleBindingsList, err := c.K8sClientset.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	clusterRoleBindingNames := []string{
		KubeArmorSnitchClusterroleBindingName,
		KubeArmorControllerProxyClusterRoleBindingName,
		KubeArmorControllerClusterRoleBindingName,
		KubeArmorClusterRoleBindingName,
		RelayClusterRoleBindingName,
		KubeArmorOperatorManageControllerClusterRoleBindingName,
		KubeArmorOperatorManageClusterRoleBindingName,
		KubeArmorOperatorClusterRoleBindingName,
	}
	// for backward-compatibility - where ClusterRoles are not KubeArmor labelled
	if len(clusterRoleBindingsList.Items) == 0 {
		clusterRoleBindingsList, err := c.K8sClientset.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Print(err)
		}
		for _, crb := range clusterRoleBindingsList.Items {
			// check for clusterroles by names
			// once we have labels in all the objects this can be removed
			if slices.Contains(clusterRoleBindingNames, crb.Name) {
				if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), crb.Name, metav1.DeleteOptions{}); err != nil {
					if !strings.Contains(err.Error(), "not found") {
						fmt.Print(err)
						continue
					}
					fmt.Printf("ℹ️  ClusterRoleBinding %s cant' be removed\n", crb.Name)
					continue
				}
				fmt.Printf("    ❌  ClusterRoleBinding %s removed\n", crb.Name)
			}
		}

	} else {
		for _, crb := range clusterRoleBindingsList.Items {
			// Older CLuster Role Binding Name, keeping it to clean up older kubearmor installations
			if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), crb.Name, metav1.DeleteOptions{}); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					fmt.Print(err)
					continue
				}
				fmt.Print("ℹ️  ClusterRoleBindings not found\n")
				continue
			}
			fmt.Printf("    ❌  ClusterRoleBinding %s removed\n", crb.Name)
		}
	}

	fmt.Print("🧹  Roles\n")
	rolesList, err := c.K8sClientset.RbacV1().Roles(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	if len(rolesList.Items) == 0 {
		rolesList, err := c.K8sClientset.RbacV1().Roles(o.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Print(err)
		}
		for _, r := range rolesList.Items {
			if r.Name == deployments.KubeArmorControllerLeaderElectionRoleName {
				if err := c.K8sClientset.RbacV1().Roles(r.Namespace).Delete(context.Background(), r.Name, metav1.DeleteOptions{}); err != nil {
					if !strings.Contains(err.Error(), "not found") {
						fmt.Print(err)
						continue
					}
					fmt.Printf("ℹ️  Error while uninstalling %s Role\n", r.Name)
					continue
				}
				fmt.Printf("    ❌  Role %s removed\n", r.Name)
			}
		}
	} else {
		if err := c.K8sClientset.RbacV1().Roles(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerLeaderElectionRoleName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				fmt.Print("Error while uninstalling KubeArmor Controller Role\n")
			}
		} else {
			fmt.Printf("    ❌  Role %s removed\n", deployments.KubeArmorControllerLeaderElectionRoleName)
		}
	}

	fmt.Print("🧹  RoleBindings\n")
	roleBindingsList, err := c.K8sClientset.RbacV1().RoleBindings(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app"})
	if err != nil {
		fmt.Print(err)
	}
	if len(roleBindingsList.Items) == 0 {
		rolesBindingsList, err := c.K8sClientset.RbacV1().RoleBindings(o.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Print(err)
		}
		for _, rb := range rolesBindingsList.Items {
			if rb.Name == deployments.KubeArmorControllerLeaderElectionRoleBindingName {
				if err := c.K8sClientset.RbacV1().RoleBindings(rb.Namespace).Delete(context.Background(), rb.Name, metav1.DeleteOptions{}); err != nil {
					if !strings.Contains(err.Error(), "not found") {
						fmt.Printf("ℹ️  Error while uninstalling %s RoleBinding\n", rb.Name)
						continue
					}
				}
				fmt.Printf("    ❌  RoleBinding %s removed\n", rb.Name)
			}
		}
	} else {
		if err := c.K8sClientset.RbacV1().RoleBindings(o.Namespace).Delete(context.Background(), deployments.KubeArmorControllerLeaderElectionRoleBindingName, metav1.DeleteOptions{}); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				fmt.Print("Error while uninstalling KubeArmor Controller Role Bindings\n")
			}
		} else {
			fmt.Printf("    ❌  RoleBinding %s removed\n", deployments.KubeArmorControllerLeaderElectionRoleBindingName)
		}
	}
}

func getApparmorNodes(c *k8s.Client) ([]string, error) {
	nodes, err := c.K8sClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing nodes: %v", err)
	}
	var appArmorNodes []string
	for _, node := range nodes.Items {
		if enforcer, exists := node.Labels["kubearmor.io/enforcer"]; exists && enforcer == "apparmor" {
			appArmorNodes = append(appArmorNodes, node.Name)
		}
	}
	return appArmorNodes, nil
}
