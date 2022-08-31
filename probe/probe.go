// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package probe helps check compatibility of KubeArmor in a given environment
package probe

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"github.com/kubearmor/kubearmor-client/deployment"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/exp/slices"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"errors"

	"github.com/kubearmor/kubearmor-client/install"
	"golang.org/x/sys/unix"
)

var white = color.New(color.FgWhite)
var boldWhite = white.Add(color.Bold)
var green = color.New(color.FgGreen).SprintFunc()
var itwhite = color.New(color.Italic).Add(color.Italic).SprintFunc()

// Options provides probe daemonset options install
type Options struct {
	Namespace string
	Full      bool
}

// K8sInstaller for karmor install
func probeDaemonInstaller(c *k8s.Client, o Options) error {
	daemonset := deployment.GenerateDaemonSet(o.Namespace)
	if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

func probeDaemonUninstaller(c *k8s.Client, o Options) error {
	color.Yellow("\t Deleting Karmor Probe DaemonSet ...\n")
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
		env := install.AutoDetectEnvironment(c)
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
	if isKubeArmorRunning(c, o) {
		getKubeArmorDeployments(c, o)
		getKubeArmorContainers(c, o)
		err := probeRunningKubeArmorNodes(c, o)
		if err != nil {
			log.Println("error occured when probing kubearmor nodes", err)
		}
		err = getAnnotatedPods(c)
		if err != nil {
			log.Println("error occured when getting annotated pods", err)
		}
		return nil
	}

	/*** if kubearmor is not running: ***/

	if o.Full {
		checkHostAuditSupport()
		checkLsmSupport(getHostSupportedLSM())

		if err := probeDaemonInstaller(c, o); err != nil {
			return err
		}
		color.Yellow("\t Creating probe daemonset ...")
		time.Sleep(2 * time.Second)
		probeNode(c, o)
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
	if strings.Contains(supportedLSM, "selinux") {
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
		color.Red(" Not Supported : Kernel header must be present")
	} else {
		color.Red(" Not Supported (Kernel Version " + kernelVersion + " \n\t Kernel version must be greater than 4.14) and kernel header must be present")
	}
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

/** check if there's any file like $(uname -r) in directory /usr/src/  **/
func checkNodeKernelHeaderPresent(c *k8s.Client, o Options, nodeName string, kernelVersion string) bool {
	pods, err := c.K8sClientset.CoreV1().Pods(o.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=" + deployment.Karmorprobe,
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return false
	}

	srcPath := "/usr/src/"
	fileName := "*" + kernelVersion + "*"
	reader, outStream := io.Pipe()

	cmdArr := []string{"find", srcPath, "-name", fileName}

	req := c.K8sClientset.CoreV1().RESTClient().
		Get().
		Namespace(o.Namespace).
		Resource("pods").
		Name(pods.Items[0].Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pods.Items[0].Spec.Containers[0].Name,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return false
	}

	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
	}()

	buf, err := io.ReadAll(reader)
	if err != nil {
		return false
	}

	s := string(buf)
	if strings.Contains(s, "No such file or directory") || len(s) == 0 {
		return false
	}

	return true
}

func checkHostAuditSupport() {
	color.Yellow("\nDidn't find KubeArmor in systemd or Kubernetes, probing for support for KubeArmor\n\n")
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		kVersion := string(uname.Release[:])
		s := strings.Split(kVersion, "-")
		kernelVersion := s[0]

		_, err := boldWhite.Println("Host:")
		if err != nil {
			color.Red(" Error")
		}
		fmt.Printf("\t Observability/Audit:")
		checkAuditSupport(kernelVersion, checkKernelHeaderPresent())
	} else {
		color.Red(" Error")
	}
}

func getNodeLsmSupport(c *k8s.Client, o Options, nodeName string) (string, error) {
	srcPath := "/sys/kernel/security/lsm"
	pods, err := c.K8sClientset.CoreV1().Pods(o.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=karmor-probe",
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return "none", err
	}

	reader, outStream := io.Pipe()
	cmdArr := []string{"cat", srcPath}
	req := c.K8sClientset.CoreV1().RESTClient().
		Get().
		Namespace(o.Namespace).
		Resource("pods").
		Name(pods.Items[0].Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pods.Items[0].Spec.Containers[0].Name,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return "none", err
	}

	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
	}()

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("an error occured when reading file")
		return "none", err
	}

	s := string(buf)
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
			check2 := checkNodeKernelHeaderPresent(c, o, item.Name, kernelVersion)
			checkAuditSupport(kernelVersion, check2)
			lsm, _ := getNodeLsmSupport(c, o, item.Name)
			checkLsmSupport(lsm)

		}
	} else {
		fmt.Println("No kubernetes environment found")
	}
}

type KubeArmorProbeData struct {
	OSImage                 string
	KernelVersion           string
	KubeletVersion          string
	ContainerRuntime        string
	ActiveLSM               string
	KernelHeaderPresent     bool
	HostSecurity            bool
	ContainerSecurity       bool
	ContainerDefaultPosture tp.DefaultPosture
	HostDefaultPosture      tp.DefaultPosture
}

func isKubeArmorRunning(c *k8s.Client, o Options) bool {
	_, err := getKubeArmorDaemonset(c, o)
	return err == nil

}

func getKubeArmorDaemonset(c *k8s.Client, o Options) (bool, error) {
	var data [][]string
	// KubeArmor DaemonSet
	w, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Get(context.Background(), "kubearmor", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	color.Green("\nFound KubeArmor running in Kubernetes\n\n")
	_, err = boldWhite.Printf("Daemonset :\n")
	if err != nil {
		color.Red(" Error while printing")
	}
	desired, ready, available := w.Status.DesiredNumberScheduled, w.Status.NumberReady, w.Status.NumberAvailable
	if desired != ready && desired != available {
		data = append(data, []string{" ", "kubearmor ", "Desired: " + strconv.Itoa(int(desired)), "Ready: " + strconv.Itoa(int(ready)), "Available: " + strconv.Itoa(int(available))})
		return false, nil
	}
	data = append(data, []string{" ", "kubearmor ", "Desired: " + strconv.Itoa(int(desired)), "Ready: " + strconv.Itoa(int(ready)), "Available: " + strconv.Itoa(int(available))})
	renderOutputInTableWithNoBorders(data)
	return true, nil
}
func getKubeArmorDeployments(c *k8s.Client, o Options) {
	var data [][]string
	_, err := boldWhite.Printf("Deployments : \n")
	if err != nil {
		color.Red(" Error while printing")
	}
	kubearmorDeployments, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app",
	})

	if err != nil {
		return
	}
	if len(kubearmorDeployments.Items) > 0 {
		for _, kubearmorDeploymentItem := range kubearmorDeployments.Items {
			desired, ready, available := kubearmorDeploymentItem.Status.UpdatedReplicas, kubearmorDeploymentItem.Status.ReadyReplicas, kubearmorDeploymentItem.Status.AvailableReplicas
			if desired != ready && desired != available {
				continue
			} else {
				data = append(data, []string{" ", kubearmorDeploymentItem.Name, "Desired: " + strconv.Itoa(int(desired)), "Ready: " + strconv.Itoa(int(ready)), "Available: " + strconv.Itoa(int(available))})
			}
		}
	}
	renderOutputInTableWithNoBorders(data)
}

func getKubeArmorContainers(c *k8s.Client, o Options) {
	var data [][]string
	_, err := boldWhite.Printf("Containers : \n")
	if err != nil {
		color.Red(" Error while printing")
	}
	kubearmorPods, err := c.K8sClientset.CoreV1().Pods(o.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app",
	})
	if err != nil {
		log.Println("error occured when getting kubearmor pods", err)
		return
	}

	if len(kubearmorPods.Items) > 0 {
		for _, kubearmorPodItem := range kubearmorPods.Items {
			data = append(data, []string{" ", kubearmorPodItem.Name, "Running: " + strconv.Itoa(len(kubearmorPodItem.Spec.Containers)), "Image Version: " + kubearmorPodItem.Spec.Containers[0].Image})
		}
	}

	renderOutputInTableWithNoBorders(data)

}

func probeRunningKubeArmorNodes(c *k8s.Client, o Options) error {

	// // KubeArmor Nodes
	nodes, err := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Println("error getting nodes", err)
		return err
	}

	if len(nodes.Items) > 0 {

		for i, item := range nodes.Items {
			_, err := boldWhite.Printf("Node %d : \n", i+1)
			if err != nil {
				color.Red(" Error while printing")
			}
			err = readDataFromKubeArmor(c, o, item.Name)
			if err != nil {
				return err
			}
		}
	} else {
		fmt.Println("No kubernetes environment found")
	}
	return nil
}

func readDataFromKubeArmor(c *k8s.Client, o Options, nodeName string) error {
	srcPath := "/tmp/karmorProbeData.cfg"
	pods, err := c.K8sClientset.CoreV1().Pods(o.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		log.Println("error occured while getting kubeArmor pods", err)
		return err
	}
	reader, outStream := io.Pipe()
	cmdArr := []string{"cat", srcPath}
	req := c.K8sClientset.CoreV1().RESTClient().
		Get().
		Namespace(o.Namespace).
		Resource("pods").
		Name(pods.Items[0].Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pods.Items[0].Spec.Containers[0].Name,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())

	if err != nil {
		return err
	}
	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
	}()
	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Println("an error occured when reading file", err)
		return err
	}
	err = printKubeArmorProbeOutput(buf)
	if err != nil {
		log.Println("an error occured when printing output", err)
		return err
	}
	return nil
}

func printKubeArmorProbeOutput(buf []byte) error {
	var kd *KubeArmorProbeData
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var data [][]string
	err := json.Unmarshal(buf, &kd)
	if err != nil {
		log.Println("an error occured when parsing file", err)
		return err
	}
	data = append(data, []string{" ", "OS Image:", green(kd.OSImage)})
	data = append(data, []string{" ", "Kernel Version:", green(kd.KernelVersion)})
	data = append(data, []string{" ", "Kubelet Version:", green(kd.KubeletVersion)})
	data = append(data, []string{" ", "Container Runtime:", green(kd.ContainerRuntime)})
	data = append(data, []string{" ", "Active LSM:", green(kd.ActiveLSM)})
	data = append(data, []string{" ", "Host Security:", green(strconv.FormatBool(kd.HostSecurity))})
	data = append(data, []string{" ", "Container Security:", green(strconv.FormatBool(kd.ContainerSecurity))})
	data = append(data, []string{" ", "Container Default Posture:", green(kd.ContainerDefaultPosture.FileAction) + itwhite("(File)"), green(kd.ContainerDefaultPosture.FileAction) + itwhite("(Capabilities)"), green(kd.ContainerDefaultPosture.NetworkAction) + itwhite("(Network)")})
	data = append(data, []string{" ", "Host Default Posture:", green(kd.HostDefaultPosture.FileAction) + itwhite("(File)"), green(kd.HostDefaultPosture.CapabilitiesAction) + itwhite("(Capabilities)"), green(kd.HostDefaultPosture.NetworkAction) + itwhite("(Network)")})
	renderOutputInTableWithNoBorders(data)
	return nil
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
	err = printKubeArmorProbeOutput(buf)
	if err != nil {
		log.Println("an error occured when printing output", err)
		return err
	}
	return nil
}

func getAnnotatedPods(c *k8s.Client) error {
	// Annotated Pods Description
	var data [][]string
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	policyMap, err := getPoliciesOnAnnotatedPods(c)
	if err != nil {
		color.Red(" Error getting policies on annotated pods")
	}

	for _, p := range pods.Items {
		if p.Annotations["kubearmor-policy"] == "enabled" {
			armoredPod, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			data = append(data, []string{armoredPod.Namespace, armoredPod.Name, ""})
			for key, value := range armoredPod.Labels {
				for policyKey, policyValue := range policyMap {
					if slices.Contains(policyValue, key+":"+value) {
						if checkIfDataAlreadyContainsPodName(data, armoredPod.Name, policyKey) {
							continue
						} else {
							data = append(data, []string{armoredPod.Namespace, armoredPod.Name, policyKey})
						}

					}

				}
			}
		}
	}
	_, err = boldWhite.Printf("Armored Up pods : \n")
	if err != nil {
		color.Red(" Error printing bold text")
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAMESPACE", "NAME", "POLICY"})

	for _, v := range data {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()
	return nil
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

func renderOutputInTableWithNoBorders(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

func checkIfDataAlreadyContainsPodName(input [][]string, name string, policy string) bool {
	for _, slice := range input {
		//if slice contains podname, then append the policy to the existing policies
		if slices.Contains(slice, name) {
			if slice[2] == "" {
				slice[2] = policy
			} else {
				slice[2] = slice[2] + "\n" + policy
			}
			return true
		}

	}
	return false
}
