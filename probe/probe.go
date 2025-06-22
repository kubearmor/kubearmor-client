// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package probe helps check compatibility of KubeArmor in a given environment

// Don't import any unix or windows specific package in this file. This file is commonly shared by
// both windows and unix platforms and importing any platform specific packages here will lead to compilation
// errors in both platforms.
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"github.com/kubearmor/kubearmor-client/k8s"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

<<<<<<< HEAD
	"errors"
=======
	pb "github.com/kubearmor/KubeArmor/protobuf"
	"golang.org/x/sys/unix"
>>>>>>> 4997334 (fix: probe command)
)

var (
	white     = color.New(color.FgWhite)
	boldWhite = white.Add(color.Bold)
	green     = color.New(color.FgGreen)
	itwhite   = color.New(color.Italic).Add(color.Italic)
	red       = color.New(color.FgRed)
	yellow    = color.New(color.FgYellow)
	blue      = color.New(color.FgBlue)
)

var ErrKubeArmorNotRunningOnK8s = errors.New("kubearmor is not running in k8s")

func PrintProbeResultCmd(c *k8s.Client, o Options) error {
	return printProbeResult(c, o)
}

func printWhenKubeArmorIsRunningInK8s(c *k8s.Client, o Options, daemonsetStatus *Status) error {
	deploymentData := getKubeArmorDeployments(c)
	containerData := getKubeArmorContainers(c)
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
		o.printLn(string(out))
	} else {
		o.printDaemonsetData(daemonsetStatus)
		o.printKubearmorDeployments(deploymentData)
		o.printKubeArmorContainers(containerData)
		o.printKubeArmorprobe(probeData)
		o.printAnnotatedPods(podData)
	}

	return nil
}

<<<<<<< HEAD
=======
func probeDaemonUninstaller(c *k8s.Client, o Options) error {
	if err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Delete(context.Background(), deployment.Karmorprobe, metav1.DeleteOptions{}); err != nil {
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
		kd, err := probeSystemdMode()
		if err != nil {
			return err
		}
		policyData, err := getPolicyData(o)
		if err != nil {
			return err
		}
		armoredContainers, containerMap := getArmoredContainerData(policyData.ContainerList, policyData.ContainerMap)
		hostPolicyData := getHostPolicyData(policyData)
		if o.Output == "json" {
			probeData := map[string]interface{}{
				"Probe Data": map[string]interface{}{
					"Host":              kd,
					"HostPolicies":      policyData.HostMap,
					"ArmoredContainers": containerMap,
				},
			}
			out, err := json.Marshal(probeData)
			if err != nil {
				return err
			}
			o.printLn(string(out))
		} else {

			o.printToOutput(green, "\nFound KubeArmor running in Systemd mode \n\n")

			o.printToOutput(boldWhite, "Host : \n")

			o.printKubeArmorProbeOutput(kd)
			if len(policyData.HostMap) > 0 {
				o.printHostPolicy(hostPolicyData)
			}
			o.printContainersSystemd(armoredContainers)

		}

		return nil
	}
	isRunning, daemonsetStatus := isKubeArmorRunning(c)
	if isRunning {
		deploymentData := getKubeArmorDeployments(c)
		containerData := getKubeArmorContainers(c)
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
			ProbeData := map[string]interface{}{
				"Probe Data": map[string]interface{}{
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
			o.printLn(string(out))
		} else {
			o.printDaemonsetData(daemonsetStatus)
			o.printKubearmorDeployments(deploymentData)
			o.printKubeArmorContainers(containerData)
			o.printKubeArmorprobe(probeData)
			o.printAnnotatedPods(podData)
		}

		return nil
	}

	/*** if kubearmor is not running: ***/

	if o.Full {
		checkHostAuditSupport(o)
		checkLsmSupport(getHostSupportedLSM(), o)

		if err := probeDaemonInstaller(c, o, true); err != nil {
			return err
		}
		o.printToOutput(yellow, "\nCreating probe daemonset ...")

	checkprobe:
		for timeout := time.After(10 * time.Second); ; {
			select {
			case <-timeout:
				o.printToOutput(red, "Failed to deploy probe daemonset ...")
				o.printToOutput(yellow, "Cleaning Up Karmor Probe DaemonSet ...\n")
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
							o.printToOutput(yellow, "Recreating Probe Daemonset ...")
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
		o.printToOutput(yellow, "\nDeleting Karmor Probe DaemonSet ...\n")
		if err := probeDaemonUninstaller(c, o); err != nil {
			return err
		}
	} else {
		checkHostAuditSupport(o)
		checkLsmSupport(getHostSupportedLSM(), o)
		o.printToOutput(blue, "To get full probe, a daemonset will be deployed in your cluster - This daemonset will be deleted after probing")
		o.printToOutput(blue, "Use --full tag to get full probing")
	}
	return nil
}

func checkLsmSupport(supportedLSM string, o Options) {
	o.printLn("\t Enforcement:")
	if strings.Contains(supportedLSM, "bpf") {
		o.printToOutput(green, " Full (Supported LSMs: "+supportedLSM+")")
	} else if strings.Contains(supportedLSM, "selinux") {
		o.printToOutput(yellow, " Partial (Supported LSMs: "+supportedLSM+") \n\t To have full enforcement support, apparmor must be supported")
	} else if strings.Contains(supportedLSM, "apparmor") || strings.Contains(supportedLSM, "bpf") {
		o.printToOutput(green, " Full (Supported LSMs: "+supportedLSM+")")
	} else {
		o.printToOutput(red, " None (Supported LSMs: "+supportedLSM+") \n\t To have full enforcement support, AppArmor or BPFLSM must be supported")
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

func checkAuditSupport(kernelVersion string, kernelHeaderPresent bool, o Options) {
	if kernelVersionSupported(kernelVersion) && kernelHeaderPresent {
		o.printToOutput(green, " Supported (Kernel Version "+kernelVersion+")")
	} else if kernelVersionSupported(kernelVersion) {
		o.printToOutput(red, " Not Supported : BTF Information/Kernel Headers must be available")
	} else {
		o.printToOutput(red, " Not Supported (Kernel Version "+kernelVersion+" \n\t Kernel version must be greater than 4.14) and BTF Information/Kernel Headers must be available")
	}
}

func checkBTFSupport() bool {
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); !os.IsNotExist(err) {
		return true
	}
	return false
}

func checkKernelHeaderPresent() bool {
	// check if there's any directory /usr/src/$(uname -r)
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		path := ""
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

func checkHostAuditSupport(o Options) {
	o.printToOutput(yellow, "\nDidn't find KubeArmor in systemd or Kubernetes, probing for support for KubeArmor\n\n")
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		kerVersion := string(uname.Release[:])
		s := strings.Split(kerVersion, "-")
		kernelVersion := s[0]

		o.printToOutput(boldWhite, "Host:")
		o.printF("\t Observability/Audit:")
		checkAuditSupport(kernelVersion, checkBTFSupport() || checkKernelHeaderPresent(), o)
	} else {
		o.printToOutput(red, " Error")
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
			str := fmt.Sprintf("Node %d : \n", i+1)
			o.printToOutput(boldWhite, str)
			o.printF("\t Observability/Audit:")
			kernelVersion := item.Status.NodeInfo.KernelVersion
			check2 := checkNodeKernelHeaderPresent(c, o, item.Name)
			checkAuditSupport(kernelVersion, check2, o)
			lsm, err := getNodeLsmSupport(c, o, item.Name)
			if err != nil {
				color.Red(err.Error())
			}
			checkLsmSupport(lsm, o)
		}
	} else {
		o.printLn("No kubernetes environment found")
	}
}

>>>>>>> 4997334 (fix: probe command)
func isKubeArmorRunning(c *k8s.Client) (bool, *Status) {
	isRunning, DaemonsetStatus := getKubeArmorDaemonset(c)
	return isRunning, DaemonsetStatus
}

func getKubeArmorDaemonset(c *k8s.Client) (bool, *Status) {
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
	if desired != ready && desired != available && ready == 0 {
		// set kubearmor to not running only if there are 0 ready pods
		return false, nil
	}
	DaemonSetStatus := Status{
		Desired:   strconv.Itoa(int(desired)),
		Ready:     strconv.Itoa(int(ready)),
		Available: strconv.Itoa(int(available)),
	}
	return true, &DaemonSetStatus
}

func getKubeArmorDeployments(c *k8s.Client) map[string]*Status {
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

func getKubeArmorContainers(c *k8s.Client) map[string]*KubeArmorPodSpec {
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
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})

	if err != nil || len(pods.Items) == 0 {
		return []KubeArmorProbeData{}, nil, fmt.Errorf("no nodes found")
	}
	nodeData := make(map[string]KubeArmorProbeData)

	var dataList []KubeArmorProbeData
	for i, item := range pods.Items {
		if item.Status.Phase != corev1.PodRunning {
			continue
		}
		data, err := readDataFromKubeArmor(c, item)
		if err != nil {
			continue
		}
		dataList = append(dataList, data)
		nodeData["Node"+strconv.Itoa(i+1)] = data
	}

	return dataList, nodeData, nil
}

func readDataFromKubeArmor(c *k8s.Client, pod corev1.Pod) (KubeArmorProbeData, error) {
	srcPath := "/tmp/karmorProbeData.cfg"
	reader, outStream := io.Pipe()
	cmdArr := []string{"cat", srcPath}
	req := c.K8sClientset.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
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

	if len(buf) == 0 {
		return KubeArmorProbeData{}, fmt.Errorf("read empty data from kubearmor pod")
	}
	var kd KubeArmorProbeData
	json := jsoniter.ConfigCompatibleWithStandardLibrary
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

<<<<<<< HEAD
=======
// sudo systemctl status kubearmor
func isSystemdMode() bool {
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	return err == nil
}

func probeSystemdMode() (KubeArmorProbeData, error) {
	jsonFile, err := os.Open("/tmp/karmorProbeData.cfg")
	if err != nil {
		log.Println(err)
		return KubeArmorProbeData{}, err
	}

	buf, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Println("an error occured when reading file", err)
		return KubeArmorProbeData{}, err
	}

	var kd KubeArmorProbeData
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(buf, &kd)
	if err != nil {
		return KubeArmorProbeData{}, err
	}
	return kd, nil
}

func getPolicyData(o Options) (*pb.ProbeResponse, error) {
	gRPC := ""

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
			gRPC = val
		} else {
			gRPC = "localhost:32767"
		}
	}
	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := pb.NewProbeServiceClient(conn)

	resp, err := client.GetProbeData(context.Background(), &emptypb.Empty{})
	if err != nil {
		o.printLn(err)
		return nil, err
	}

	return resp, nil
}

func getArmoredContainerData(containerList []string, containerMap map[string]*pb.ContainerData) ([][]string, map[string][]string) {
	var data [][]string
	for _, containerName := range containerList {
		if _, ok := containerMap[containerName]; ok {
			if containerMap[containerName].PolicyEnabled == 1 {
				for _, policyName := range containerMap[containerName].PolicyList {
					data = append(data, []string{containerName, policyName})
				}
			}
		} else {
			data = append(data, []string{containerName, ""})
		}
	}
	mp := make(map[string][]string)

	for _, v := range data {
		if val, exists := mp[v[0]]; exists {

			val = append(val, v[1])
			mp[v[0]] = val

		} else {
			mp[v[0]] = []string{v[1]}
		}
	}

	return data, mp
}

func getHostPolicyData(policyData *pb.ProbeResponse) [][]string {
	var data [][]string
	for k, v := range policyData.HostMap {
		for _, policy := range v.PolicyList {
			data = append(data, []string{k, policy})
		}
	}
	return data
}

>>>>>>> 4997334 (fix: probe command)
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
		o.printToOutput(red, " Error getting policies on annotated pods")
	}

	for _, p := range pods.Items {
		if p.Annotations["kubearmor-policy"] == "enabled" {
			armoredPod, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return nil, [][]string{}, err
			}
			if _, exists := mp[armoredPod.Namespace]; !exists {
				data = append(data, []string{armoredPod.Namespace, "", "", armoredPod.Name, ""})
			} else {
				data = append(data, []string{armoredPod.Namespace, mp[armoredPod.Namespace].NsPostureString, mp[armoredPod.Namespace].NsVisibilityString, armoredPod.Name, ""})
			}
			labels := getAnnotatedPodLabels(armoredPod.Labels)

			for _, policy := range policyMap {
				s2 := sliceToSet(policy["labels"].([]string))
				namespaces := policy["namespaces"].([]string)
				found := false
				for _, namespace := range namespaces {
					if namespace == armoredPod.Namespace {
						found = true
						break
					}
				}
				if found && s2.IsSubset(labels) {
					if !checkIfDataAlreadyContainsPodName(data, armoredPod.Name, policy["name"].(string)) {
						data = append(data, []string{armoredPod.Namespace, mp[armoredPod.Namespace].NsPostureString, mp[armoredPod.Namespace].NsVisibilityString, armoredPod.Name, policy["name"].(string)})
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

func getPoliciesOnAnnotatedPods(c *k8s.Client) ([]map[string]interface{}, error) {
	var maps []map[string]interface{}
	kspInterface := c.KSPClientset.KubeArmorPolicies("")
	policies, err := kspInterface.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(policies.Items) > 0 {
		for _, policy := range policies.Items {
			p := make(map[string]interface{})
			selectLabels := policy.Spec.Selector.MatchLabels
			labels := []string{}
			namespaces := []string{}
			for key, value := range selectLabels {
				labels = append(labels, key+":"+value)
				namespaces = append(namespaces, policy.Namespace)
			}
			p["name"] = policy.Name
			p["labels"] = labels
			p["namespaces"] = namespaces
			maps = append(maps, p)
		}
	}
	return maps, nil
}

func checkIfDataAlreadyContainsPodName(input [][]string, name string, policy string) bool {
	for _, slice := range input {
		// if slice contains podname, then append the policy to the existing policies
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
