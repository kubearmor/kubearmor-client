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
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/deployment"
	"github.com/kubearmor/kubearmor-client/k8s"
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

	if o.Full {
		checkHostAuditSupport()
		checkLsmSupport(getHostSupportedLSM())

		if err := probeDaemonInstaller(c, o); err != nil {
			return err
		}
		color.Yellow("\t Creating probe daemonset ...")
		time.Sleep(2 * time.Second)
		probeNode(c)
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
func checkNodeKernelHeaderPresent(c *k8s.Client, nodeName string, kernelVersion string) bool {
	pods, err := c.K8sClientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
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
		Namespace("default").
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

func getNodeLsmSupport(c *k8s.Client, nodeName string) (string, error) {
	srcPath := "/sys/kernel/security/lsm"
	pods, err := c.K8sClientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
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
		Namespace("default").
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

func probeNode(c *k8s.Client) {
	nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if len(nodes.Items) > 0 {
		for i, item := range nodes.Items {
			_, err := boldWhite.Printf("Node %d : \n", i+1)
			if err != nil {
				color.Red(" Error")
			}
			fmt.Printf("\t Observability/Audit:")
			kernelVersion := item.Status.NodeInfo.KernelVersion
			check2 := checkNodeKernelHeaderPresent(c, item.Name, kernelVersion)
			checkAuditSupport(kernelVersion, check2)
			lsm, _ := getNodeLsmSupport(c, item.Name)
			checkLsmSupport(lsm)

		}
	} else {
		fmt.Println("No kubernetes environment found")
	}
}
