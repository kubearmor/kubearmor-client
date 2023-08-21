// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package probe helps check compatibility of KubeArmor in a given environment
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"github.com/kubearmor/kubearmor-client/deployment"
	"github.com/kubearmor/kubearmor-client/k8s"

	"golang.org/x/exp/slices"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"errors"

	"golang.org/x/sys/unix"
)

var white = color.New(color.FgWhite)
var boldWhite = white.Add(color.Bold)
var green = color.New(color.FgGreen).SprintFunc()
var itwhite = color.New(color.Italic).Add(color.Italic).SprintFunc()

// K8sInstaller for karmor install
func probeDaemonInstaller(c *k8s.Client, o Options, krnhdr bool) error {
	daemonset := deployment.GenerateDaemonSet(o.Namespace, krnhdr)
	if _, err := c.K8sClientset.AppsV1().DaemonSets("").Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

func probeDaemonUninstaller(c *k8s.Client, o Options) error {
	if err := c.K8sClientset.AppsV1().DaemonSets("").Delete(context.Background(), deployment.Karmorprobe, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Karmor probe DaemonSet not found ...\n")
	}

	return nil
}

// PrintProbeResult prints the result for the  host and k8s probing kArmor does to check compatibility with KubeArmor
func PrintProbeResult(c *k8s.Client, o Options) error {
	if runtime.GOOS != "linux" {
		env := k8s.AutoDetectEnvironment(c)
		if env == "none" {
			return errors.New("unsupported environment or cluster not configured correctly")
		}
	}
	if isSystemdMode() {
		err := probeSystemdMode()
		if err != nil {
			return err
		}
		return nil
	}
	isRunning, daemonsetStatus := isKubeArmorRunning(c, o)
	if isRunning {
		deploymentData := getKubeArmorDeployments(c, o)
		containerData := getKubeArmorContainers(c, o)
		probeData, nodeData, err := ProbeRunningKubeArmorNodes(c, o)
		if err != nil {
			log.Println("error occured when probing kubearmor nodes", err)
		}
		postureData := getPostureData(probeData)
		armoredPodData, podData, err := getAnnotatedPods(c, o, postureData)
		if err != nil {
			log.Println("error occured when getting annotated pods", err)
		}
		if o.Output == "json" {
			ProbeData := map[string]interface{}{"Probe Data": map[string]interface{}{
				"DaemonsetStatus": daemonsetStatus,
				"Deployments":     deploymentData,
				"Containers":      containerData,
				"Nodes":           nodeData,
				"ArmoredPods":     armoredPodData,
			},
			}
			out, err := json.Marshal(ProbeData)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
		} else {
			printDaemonsetData(daemonsetStatus)
			printKubearmorDeployments(deploymentData)
			printKubeArmorContainers(containerData)
			printKubeArmorprobe(probeData)
			printAnnotatedPods(podData)
		}

		return nil
	}

	/*** if kubearmor is not running: ***/

	if o.Full {
		checkHostAuditSupport()
		checkLsmSupport(getHostSupportedLSM())

		if err := probeDaemonInstaller(c, o, true); err != nil {
			return err
		}
		color.Yellow("\nCreating probe daemonset ...")

	checkprobe:
		for timeout := time.After(10 * time.Second); ; {
			select {
			case <-timeout:
				color.Red("Failed to deploy probe daemonset ...")
				color.Yellow("Cleaning Up Karmor Probe DaemonSet ...\n")
				if err := probeDaemonUninstaller(c, o); err != nil {
					return err
				}
				return nil
			default:
				time.Sleep(500 * time.Millisecond)
				pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "kubearmor-app=" + deployment.Karmorprobe, FieldSelector: "status.phase!=Running"})
				if len(pods.Items) == 0 {
					break checkprobe
				}
				if pods.Items[0].Status.ContainerStatuses[0].State.Waiting != nil {
					state := pods.Items[0].Status.ContainerStatuses[0].State.Waiting
					// We redeploy without kernel header mounts
					if state.Reason == "CreateContainerError" {
						if strings.Contains(state.Message, "/usr/src") || strings.Contains(state.Message, "/lib/modules") {
							color.Yellow("Recreating Probe Daemonset ...")
							if err := probeDaemonUninstaller(c, o); err != nil {
								return err
							}
							if err := probeDaemonInstaller(c, o, false); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		probeNode(c, o)
		color.Yellow("\nDeleting Karmor Probe DaemonSet ...\n")
		if err := probeDaemonUninstaller(c, o); err != nil {
			return err
		}
	} else {
		checkHostAuditSupport()
		checkLsmSupport(getHostSupportedLSM())
		color.Blue("To get full probe, a daemonset will be deployed in your cluster - This daemonset will be deleted after probing")
		color.Blue("Use --full tag to get full probing")
	}
	return nil
}

func checkLsmSupport(supportedLSM string) {
	fmt.Printf("\t Enforcement:")
	if strings.Contains(supportedLSM, "bpf") {
		color.Green(" Full (Supported LSMs: " + supportedLSM + ")")
	} else if strings.Contains(supportedLSM, "selinux") {
		color.Yellow(" Partial (Supported LSMs: " + supportedLSM + ") \n\t To have full enforcement support, apparmor must be supported")
	} else if strings.Contains(supportedLSM, "apparmor") || strings.Contains(supportedLSM, "bpf") {
		color.Green(" Full (Supported LSMs: " + supportedLSM + ")")
	} else {
		color.Red(" None (Supported LSMs: " + supportedLSM + ") \n\t To have full enforcement support, AppArmor or BPFLSM must be supported")
	}
}

func getHostSupportedLSM() string {
	b, err := os.ReadFile("/sys/kernel/security/lsm")
	if err != nil {
		log.Printf("an error occured when reading file")
		return "none"
	}
	s := string(b)
	return s
}

func kernelVersionSupported(kernelVersion string) bool {
	return semver.Compare(kernelVersion, "4.14") >= 0
}

func checkAuditSupport(kernelVersion string, kernelHeaderPresent bool) {
	if kernelVersionSupported(kernelVersion) && kernelHeaderPresent {
		color.Green(" Supported (Kernel Version " + kernelVersion + ")")
	} else if kernelVersionSupported(kernelVersion) {
		color.Red(" Not Supported : BTF Information/Kernel Headers must be available")
	} else {
		color.Red(" Not Supported (Kernel Version " + kernelVersion + " \n\t Kernel version must be greater than 4.14) and BTF Information/Kernel Headers must be available")
	}
}

func checkBTFSupport() bool {
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); !os.IsNotExist(err) {
		return true
	}
	return false
}

func checkKernelHeaderPresent() bool {
	//check if there's any directory /usr/src/$(uname -r)
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		var path = ""
		if _, err := os.Stat("/etc/redhat-release"); !os.IsNotExist(err) {
			path = "/usr/src/" + string(uname.Release[:])
		} else if _, err := os.Stat("/lib/modules/" + string(uname.Release[:]) + "/build/Kconfig"); !os.IsNotExist(err) {
			path = "/lib/modules/" + string(uname.Release[:]) + "/build"
		} else if _, err := os.Stat("/lib/modules/" + string(uname.Release[:]) + "/source/Kconfig"); !os.IsNotExist(err) {
			path = "/lib/modules/" + string(uname.Release[:]) + "/source"
		} else {
			path = "/usr/src/linux-headers-" + string(uname.Release[:])
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func execIntoPod(c *k8s.Client, podname, namespace, cmd string) (string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := c.K8sClientset.CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podname).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", cmd},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", request.URL())
	if err != nil {
		return "none", err
	}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})

	if err != nil {
		return "none", err
	}

	return buf.String(), nil
}

func findFileInDir(c *k8s.Client, podname, namespace, cmd string) bool {
	s, err := execIntoPod(c, podname, namespace, cmd)
	if err != nil {
		return false
	}
	if strings.Contains(s, "No such file or directory") || len(s) == 0 {
		return false
	}

	return true
}

// Check for BTF Information or Kernel Headers Availability
func checkNodeKernelHeaderPresent(c *k8s.Client, o Options, nodeName string) bool {
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=" + deployment.Karmorprobe,
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return false
	}

	if findFileInDir(c, pods.Items[0].Name, o.Namespace, "find /sys/kernel/btf/ -name vmlinux") ||
		findFileInDir(c, pods.Items[0].Name, o.Namespace, "find -L /usr/src -maxdepth 2 -path \"*$(uname -r)*\" -name \"Kconfig\"") ||
		findFileInDir(c, pods.Items[0].Name, o.Namespace, "find -L /lib/modules/ -maxdepth 3 -path \"*$(uname -r)*\" -name \"Kconfig\"") {
		return true
	}

	return false
}

func checkHostAuditSupport() {
	color.Yellow("\nDidn't find KubeArmor in systemd or Kubernetes, probing for support for KubeArmor\n\n")
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		kerVersion := string(uname.Release[:])
		s := strings.Split(kerVersion, "-")
		kernelVersion := s[0]

		_, err := boldWhite.Println("Host:")
		if err != nil {
			color.Red(" Error")
		}
		fmt.Printf("\t Observability/Audit:")
		checkAuditSupport(kernelVersion, checkBTFSupport() || checkKernelHeaderPresent())
	} else {
		color.Red(" Error")
	}
}

func getNodeLsmSupport(c *k8s.Client, o Options, nodeName string) (string, error) {
	srcPath := "/sys/kernel/security/lsm"
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=karmor-probe",
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return "none", err
	}

	s, err := execIntoPod(c, pods.Items[0].Name, o.Namespace, "cat "+srcPath)
	if err != nil {
		return "none", err
	}
	return s, nil
}

func probeNode(c *k8s.Client, o Options) {
	nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if len(nodes.Items) > 0 {
		for i, item := range nodes.Items {
			_, err := boldWhite.Printf("Node %d : \n", i+1)
			if err != nil {
				color.Red(" Error")
			}
			fmt.Printf("\t Observability/Audit:")
			kernelVersion := item.Status.NodeInfo.KernelVersion
			check2 := checkNodeKernelHeaderPresent(c, o, item.Name)
			checkAuditSupport(kernelVersion, check2)
			lsm, err := getNodeLsmSupport(c, o, item.Name)
			if err != nil {
				color.Red(err.Error())
			}
			checkLsmSupport(lsm)
		}
	} else {
		fmt.Println("No kubernetes environment found")
	}
}

func isKubeArmorRunning(c *k8s.Client, o Options) (bool, *Status) {
	isRunning, DaemonsetStatus := getKubeArmorDaemonset(c, o)
	return isRunning, DaemonsetStatus

}

func getKubeArmorDaemonset(c *k8s.Client, o Options) (bool, *Status) {
	// KubeArmor DaemonSet
	w, err := c.K8sClientset.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err != nil {
		log.Println("error when getting kubearmor daemonset", err)
		return false, nil
	}
	if len(w.Items) == 0 {
		return false, nil
	}
	desired, ready, available := w.Items[0].Status.DesiredNumberScheduled, w.Items[0].Status.NumberReady, w.Items[0].Status.NumberAvailable
	if desired != ready && desired != available {
		return false, nil
	}
	DaemonSetStatus := Status{
		Desired:   strconv.Itoa(int(desired)),
		Ready:     strconv.Itoa(int(ready)),
		Available: strconv.Itoa(int(available)),
	}
	return true, &DaemonSetStatus
}

func getKubeArmorDeployments(c *k8s.Client, o Options) map[string]*Status {
	kubearmorDeployments, err := c.K8sClientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app",
	})
	if err != nil {
		log.Println("error while getting kubearmor deployments", err)
		return nil
	}
	if len(kubearmorDeployments.Items) > 0 {
		DeploymentsData := make(map[string]*Status)
		for _, kubearmorDeploymentItem := range kubearmorDeployments.Items {
			desired, ready, available := kubearmorDeploymentItem.Status.UpdatedReplicas, kubearmorDeploymentItem.Status.ReadyReplicas, kubearmorDeploymentItem.Status.AvailableReplicas
			if desired == ready && desired == available {
				DeploymentsData[kubearmorDeploymentItem.Name] = &Status{
					Desired:   strconv.Itoa(int(desired)),
					Ready:     strconv.Itoa(int(ready)),
					Available: strconv.Itoa(int(available)),
				}
			}
		}

		return DeploymentsData
	}
	return nil
}

func getKubeArmorContainers(c *k8s.Client, o Options) map[string]*KubeArmorPodSpec {

	kubearmorPods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app",
	})

	if err != nil {
		log.Println("error occured when getting kubearmor pods", err)
		return nil
	}
	KAContainerData := make(map[string]*KubeArmorPodSpec)
	if len(kubearmorPods.Items) > 0 {
		for _, kubearmorPodItem := range kubearmorPods.Items {

			KAContainerData[kubearmorPodItem.Name] = &KubeArmorPodSpec{
				Running:       strconv.Itoa(len(kubearmorPodItem.Spec.Containers)),
				Image_Version: kubearmorPodItem.Spec.Containers[0].Image,
			}
		}

		return KAContainerData
	}
	return nil
}

// ProbeRunningKubeArmorNodes extracts data from running KubeArmor daemonset  by executing into the container and reading /tmp/kubearmor.cfg
func ProbeRunningKubeArmorNodes(c *k8s.Client, o Options) ([]KubeArmorProbeData, map[string]KubeArmorProbeData, error) {
	// KubeArmor Nodes
	nodes, err := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return []KubeArmorProbeData{}, nil, fmt.Errorf("error occured when getting nodes %s", err.Error())
	}

	if len(nodes.Items) == 0 {
		return []KubeArmorProbeData{}, nil, fmt.Errorf("no nodes found")
	}
	nodeData := make(map[string]KubeArmorProbeData)

	var dataList []KubeArmorProbeData
	for i, item := range nodes.Items {
		data, err := readDataFromKubeArmor(c, o, item.Name)
		if err != nil {
			return []KubeArmorProbeData{}, nil, err
		}
		dataList = append(dataList, data)
		nodeData["Node"+strconv.Itoa(i+1)] = data
	}

	return dataList, nodeData, nil
}

func readDataFromKubeArmor(c *k8s.Client, o Options, nodeName string) (KubeArmorProbeData, error) {
	srcPath := "/tmp/karmorProbeData.cfg"
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil || pods == nil || len(pods.Items) == 0 {
		return KubeArmorProbeData{}, fmt.Errorf("error occured while getting KubeArmor pods %s", err.Error())
	}
	reader, outStream := io.Pipe()
	cmdArr := []string{"cat", srcPath}
	req := c.K8sClientset.CoreV1().RESTClient().
		Get().
		Namespace("").
		Resource("pods").
		Name(pods.Items[0].Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pods.Items[0].Spec.Containers[0].Name,
			Command:   cmdArr,
			Stdin:     false,
			Stdout:    true,
			Stderr:    false,
			TTY:       false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return KubeArmorProbeData{}, err
	}
	go func() {
		defer outStream.Close()
		err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
			Stdout: outStream,
			Tty:    false,
		})
	}()
	buf, err := io.ReadAll(reader)
	if err != nil {
		return KubeArmorProbeData{}, fmt.Errorf("error occured while reading data from kubeArmor pod %s", err.Error())
	}
	var kd KubeArmorProbeData
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(buf, &kd)
	if err != nil {
		return KubeArmorProbeData{}, fmt.Errorf("error occured while parsing data from kubeArmor pod %s", err.Error())
	}
	return kd, nil
}
func getPostureData(probeData []KubeArmorProbeData) map[string]string {
	postureData := make(map[string]string)
	if len(probeData) > 0 {
		postureData["filePosture"] = probeData[0].ContainerDefaultPosture.FileAction
		postureData["capabilitiesPosture"] = probeData[0].ContainerDefaultPosture.CapabilitiesAction
		postureData["networkPosture"] = probeData[0].ContainerDefaultPosture.NetworkAction
		postureData["visibility"] = probeData[0].HostVisibility
	}

	return postureData
}

// sudo systemctl status kubearmor
func isSystemdMode() bool {
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	color.Green("\nFound KubeArmor running in Systemd mode \n\n")
	return true
}

func probeSystemdMode() error {
	jsonFile, err := os.Open("/tmp/karmorProbeData.cfg")
	if err != nil {
		log.Println(err)
		return err
	}

	buf, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Println("an error occured when reading file", err)
		return err
	}
	_, err = boldWhite.Printf("Host : \n")
	if err != nil {
		color.Red(" Error")
	}
	var kd KubeArmorProbeData
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(buf, &kd)
	if err != nil {
		return err
	}
	printKubeArmorProbeOutput(kd)
	return nil
}

func getAnnotatedPodLabels(m map[string]string) mapset.Set[string] {
	var a []string
	for key, value := range m {
		a = append(a, key+":"+value)
	}
	b := sliceToSet(a)
	return b
}

func getNsSecurityPostureAndVisibility(c *k8s.Client, postureData map[string]string) (map[string]*NamespaceData, error) {
	// Namespace/host security posture and visibility setting

	mp := make(map[string]*NamespaceData)
	namespaces, err := c.K8sClientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})

	if err != nil {
		return mp, err
	}
	for _, ns := range namespaces.Items {

		filePosture := postureData["filePosture"]
		capabilityPosture := postureData["capabilitiesPosture"]
		networkPosture := postureData["networkPosture"]
		visibility := postureData["visibility"]

		if len(ns.Annotations["kubearmor-file-posture"]) > 0 {
			filePosture = ns.Annotations["kubearmor-file-posture"]
		}

		if len(ns.Annotations["kubearmor-capabilities-posture"]) > 0 {
			capabilityPosture = ns.Annotations["kubearmor-capabilities-posture"]
		}

		if len(ns.Annotations["kubearmor-network-posture"]) > 0 {
			networkPosture = ns.Annotations["kubearmor-network-posture"]
		}

		if len(ns.Annotations["kubearmor-visibility"]) > 0 {
			visibility = ns.Annotations["kubearmor-visibility"]
		}

		mp[ns.Name] = &NamespaceData{
			NsDefaultPosture:   tp.DefaultPosture{FileAction: filePosture, CapabilitiesAction: capabilityPosture, NetworkAction: networkPosture},
			NsVisibilityString: visibility,
			NsVisibility: Visibility{
				Process:      strings.Contains(visibility, "process"),
				File:         strings.Contains(visibility, "file"),
				Network:      strings.Contains(visibility, "network"),
				Capabilities: strings.Contains(visibility, "capabilities"),
			},
			NsPostureString: "File(" + filePosture + "), Capabilities(" + capabilityPosture + "), Network (" + networkPosture + ")",
		}
	}
	return mp, err
}

func getAnnotatedPods(c *k8s.Client, o Options, postureData map[string]string) (map[string]interface{}, [][]string, error) {

	// Annotated Pods Description
	var data [][]string
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, [][]string{}, err
	}

	armoredPodData := make(map[string]*NamespaceData)
	mp, err := getNsSecurityPostureAndVisibility(c, postureData)
	if err != nil {
		return nil, [][]string{}, err
	}

	policyMap, err := getPoliciesOnAnnotatedPods(c)
	if err != nil {
		color.Red(" Error getting policies on annotated pods")
	}

	for _, p := range pods.Items {

		if p.Annotations["kubearmor-policy"] == "enabled" {
			armoredPod, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return nil, [][]string{}, err
			}
			data = append(data, []string{armoredPod.Namespace, mp[armoredPod.Namespace].NsPostureString, mp[armoredPod.Namespace].NsVisibilityString, armoredPod.Name, ""})
			labels := getAnnotatedPodLabels(armoredPod.Labels)

			for policyKey, policyValue := range policyMap {
				s2 := sliceToSet(policyValue)
				if s2.IsSubset(labels) {
					if !checkIfDataAlreadyContainsPodName(data, armoredPod.Name, policyKey) {

						data = append(data, []string{armoredPod.Namespace, mp[armoredPod.Namespace].NsPostureString, mp[armoredPod.Namespace].NsVisibilityString, armoredPod.Name, policyKey})

					}
				}
			}
		}
	}
	// sorting according to namespaces, for merging of cells with same namespaces
	sort.SliceStable(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	for _, v := range data {

		if _, exists := armoredPodData[v[0]]; !exists {
			armoredPodData[v[0]] = &NamespaceData{
				NsDefaultPosture: mp[v[0]].NsDefaultPosture,
				NsVisibility:     mp[v[0]].NsVisibility,
			}
		}
		armoredPodData[v[0]].NsPodList = append(armoredPodData[v[0]].NsPodList, PodInfo{PodName: v[3], Policy: v[4]})
	}

	return map[string]interface{}{"Namespaces": armoredPodData}, data, nil
}

func getPoliciesOnAnnotatedPods(c *k8s.Client) (map[string][]string, error) {
	maps := make(map[string][]string)
	kspInterface := c.KSPClientset.KubeArmorPolicies("")
	policies, err := kspInterface.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(policies.Items) > 0 {
		for _, policy := range policies.Items {
			selectLabels := policy.Spec.Selector.MatchLabels
			for key, value := range selectLabels {
				maps[policy.Name] = append(maps[policy.Name], key+":"+value)
			}
		}
	}
	return maps, nil
}
func checkIfDataAlreadyContainsPodName(input [][]string, name string, policy string) bool {
	for _, slice := range input {
		//if slice contains podname, then append the policy to the existing policies
		if slices.Contains(slice, name) {
			if slice[4] == "" {
				slice[4] = policy
			} else {
				slice[4] = slice[4] + "\n" + policy
			}
			return true
		}

	}
	return false
}

func sliceToSet(mySlice []string) mapset.Set[string] {
	mySet := mapset.NewSet[string]()
	for _, ele := range mySlice {
		mySet.Add(ele)
	}
	return mySet
}
