// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package install is responsible for installation and uninstallation of KubeArmor while autogenerating the configuration
package install

import (
	"context"
	// "path/filepath"

	"errors"
	"fmt"
	"os"
	// "path"
	// "slices"
	"strings"
	"time"
	"log"

	"github.com/clarketm/json"
	"github.com/fatih/color"
	"sigs.k8s.io/yaml"

	// deployments "github.com/kubearmor/KubeArmor/deployments/get"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"

	v1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"

	// apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	
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
	Verify         bool
	Env            envOption
}

type envOption struct {
	Auto        bool
	Environment string
}

var verify bool
var progress int
var cursorcount int
var validEnvironments = []string{"k0s", "k3s", "microK8s", "minikube", "gke", "bottlerocket", "eks", "docker", "oke", "generic"}

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

func checkPods(c *k8s.Client, o Options) {
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

// K8sInstaller for karmor
func K8sInstaller(c *k8s.Client) error {
	namespace := "kubearmor"

	settings := cli.New()
	settings.Debug = true
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return fmt.Errorf("failed to initialize Helm configuration: %w", err)
	}

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
	if err := repoFile.WriteFile(settings.RepositoryConfig, 0644); err != nil {
		return fmt.Errorf("failed to write repository file: %w", err)
	}

	values := map[string]interface{}{
		"autoDeploy": true,
	}

	client := action.NewUpgrade(actionConfig)
	client.Namespace = namespace
	client.Timeout = 5 * time.Minute
	client.Wait = true
	client.Install = true

	chartPath, err := client.ChartPathOptions.LocateChart("kubearmor/kubearmor-operator", settings)
	if err != nil {
		return fmt.Errorf("failed to locate Helm chart: %w", err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load Helm chart: %w", err)
	}

	_, err = client.Run("kubearmor-operator", chartRequested, values)
	if err != nil {
		client.Install = true
		if client.Install {
			histClient := action.NewHistory(actionConfig)
			histClient.Max = 1
	
			clientInstall := action.NewInstall(actionConfig) 
			clientInstall.ReleaseName = "kubearmor-operator"
			clientInstall.Namespace = namespace
			clientInstall.Timeout = 5 * time.Minute
			clientInstall.Wait = true
			clientInstall.CreateNamespace = true

	
			chartPath, err := clientInstall.ChartPathOptions.LocateChart("kubearmor/kubearmor-operator", settings)
			if err != nil {
				return fmt.Errorf("failed to locate Helm chart path: %w", err)
			}
			
			chartRequested, err := loader.Load(chartPath)
			if err != nil {
				return fmt.Errorf("failed to load Helm chart: %w", err)
			}
			
			_, err = clientInstall.Run(chartRequested, values)
			if err != nil {
				return fmt.Errorf("failed to install Helm chart: %w", err)
			}
			
		}
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

func removeAnnotations(c *k8s.Client, ns string) {
	deps, err := c.K8sClientset.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
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
	namespace := "kubearmor"

	settings := cli.New()
	settings.Debug = true
	settings.SetNamespace(namespace)
	settings.RESTClientGetter()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	client := action.NewUninstall(actionConfig)
	client.Timeout = 5 * time.Minute
	client.DeletionPropagation = "background"

	_, err := client.Run("kubearmor-operator")
	if err != nil {
		fmt.Println("failed to uninstall kubearmor-operator")
		return err
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
