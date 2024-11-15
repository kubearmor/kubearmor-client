//go:build darwin || (linux && !windows)

package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/deployment"
	"github.com/kubearmor/kubearmor-client/k8s"
	"golang.org/x/mod/semver"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func printProbeResult(c *k8s.Client, o Options) error {
	return printProbeResultForUnix(c, o)
}

func printProbeResultForUnix(c *k8s.Client, o Options) error {
	if runtime.GOOS != "linux" {
		env := k8s.AutoDetectEnvironment(c)
		if env == "none" {
			return errors.New("unsupported environment or cluster not configured correctly")
		}
	}
	if isSystemdMode() {
		return printWhenKubeArmorIsRunningInSystemmd(o)
	}
	isRunning, daemonsetStatus := isKubeArmorRunning(c)
	if isRunning {
		return printWhenKubeArmorIsRunningInK8s(c, o, daemonsetStatus)
	}
	/*** if kubearmor is not running: ***/

	if o.Full {
		return printWhenKubeArmorIsNotRunning(c, o)
	} else {
		checkHostAuditSupport(o)
		checkLsmSupport(getHostSupportedLSM(), o)
		o.printToOutput(blue, "To get full probe, a daemonset will be deployed in your cluster - This daemonset will be deleted after probing")
		o.printToOutput(blue, "Use --full tag to get full probing")
	}
	return nil
}

// sudo systemctl status kubearmor
func isSystemdMode() bool {
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	return err == nil
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

func printWhenKubeArmorIsNotRunning(c *k8s.Client, o Options) error {
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
	return nil
}

// K8sInstaller for karmor install
func probeDaemonInstaller(c *k8s.Client, o Options, krnhdr bool) error {
	daemonset := deployment.GenerateDaemonSet(o.Namespace, krnhdr)
	if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

func probeDaemonUninstaller(c *k8s.Client, o Options) error {
	if err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Delete(context.Background(), deployment.Karmorprobe, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Karmor probe DaemonSet not found ...\n")
	}

	return nil
}

func printWhenKubeArmorIsRunningInSystemmd(o Options) error {
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

func getHostPolicyData(policyData *pb.ProbeResponse) [][]string {
	var data [][]string
	for k, v := range policyData.HostMap {
		for _, policy := range v.PolicyList {
			data = append(data, []string{k, policy})
		}
	}
	return data
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
