// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package probe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/deployment"
	"github.com/kubearmor/kubearmor-client/k8s"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var white = color.New(color.FgWhite)
var boldWhite = white.Add(color.Bold)
var karmorprobe = "karmor-probe"



// Options for probe daemonset options install
type ProbeOptions struct {
    Namespace      string
    ProbeDaemonImage string
    Full          bool
}

// K8sInstaller for karmor install
func probeDaemonInstaller(c *k8s.Client, o ProbeOptions) error {
    
    daemonset := deployment.GenerateDaemonSet(o.Namespace)
    daemonset.Spec.Template.Spec.Containers[0].Image = o.ProbeDaemonImage
    if _, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Create(context.Background(), daemonset, metav1.CreateOptions{}); err != nil {
        if !strings.Contains(err.Error(), "already exists") {
			return errors.New("unable to install kubearmor daemonset: kubernetes environment not found or cluster not configured correctly")
        }
    }
    return nil


}


func probeDaemonUninstaller(c *k8s.Client, o ProbeOptions) error {
    color.Yellow("\t Deleting Karmor Probe DaemonSet ...\n")
	if err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Delete(context.Background(), karmorprobe, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("Karmor probe DaemonSet not found ...\n")
	}


	return nil
}

func PrintProbeResult(c *k8s.Client, o ProbeOptions) error{
	if isSystemdMode(){
		probeSystemdMode()
		return nil
	} 
	if isKubeArmorRunning(c) {
		getKubeArmorDeployments(c)
		printContainers (c)
        err := probeRunningKubeArmorNodes(c)
		if err != nil {
			log.Println("error occured when probing kubearmor nodes", err)
		}
		return nil;		
    }

	/*** if kubearmor is not running: ***/
    if o.Full {
        checkHostAuditSupport()
        checkLsmSupport(getHostSupportedLSM())

        if err := probeDaemonInstaller(c, o); err != nil {
            return err
        }
        color.Yellow("\t Creating probe daemonset ...")

		w, err := c.K8sClientset.AppsV1().DaemonSets(o.Namespace).Get(context.Background(), karmorprobe , metav1.GetOptions{})
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		desired, ready, available := w.Status.DesiredNumberScheduled, w.Status.NumberReady, w.Status.NumberAvailable
		if desired != ready && desired != available{
			err = errors.New("timeout when creating karmor probe daemonset")
			return err
		}
			
        probeNode(c)
        if err := probeDaemonUninstaller(c, o); err != nil {
            return err
        }
	}else{
        checkHostAuditSupport()
        checkLsmSupport(getHostSupportedLSM())
        color.Blue("To get full probe, a daemonset will be deployed in your cluster - This daemonset will be deleted after probing")
        color.Blue("Use --full tag to get full probing")
    }
    return nil
}


func checkLsmSupport(supportedLSM string) {
    fmt.Printf("\t Enforcement:") 
    if strings.Contains(supportedLSM, "selinux"){
        color.Yellow(" Partial (Supported LSMs: "+ supportedLSM + ") \n\t To have full enforcement support, apparmor must be supported")
    }else if strings.Contains(supportedLSM, "apparmor"){
        color.Green(" Full (Supported LSMs: "+ supportedLSM + ")")
    }else{
        color.Red(" None (Supported LSMs: "+ supportedLSM + ") \n\t To have full enforcement support, apparmor must be supported")
    }
}

func getHostSupportedLSM() string {

    b, err := os.ReadFile("/sys/kernel/security/lsm")
    if err != nil {
        log.Println("Unable to read supported LSM on host or security lsm path does not exist")
        return "none"
    }
    s := string(b)
    return s
}



func kernelVersionSupported(kernelVersion string) bool {
    return semver.Compare(kernelVersion, "4.14") >= 0
}

func checkAuditSupport(kernelVersion string, kernelHeaderPresent bool) {

    if kernelVersionSupported(kernelVersion) && kernelHeaderPresent{
        color.Green(" Supported (Kernel Version " + kernelVersion )
    }else if(kernelVersionSupported(kernelVersion)){
        color.Red(" Not Supported : Kernel header must be present")
    }else{
        color.Red(" Not Supported (Kernel Version " + kernelVersion + " \n\t Kernel version must be greater than 4.14) and kernel header must be present")
    }

}

func checkKernelHeaderPresent() bool{
    //check if there's any directory /usr/src/$(uname -r)
    var uname syscall.Utsname
    if err := syscall.Uname(&uname); err == nil {

        var path = ""
        if _, err := os.Stat("/etc/redhat-release"); !os.IsNotExist(err) {
            path = "/usr/src/"+int8ToStr(uname.Release[:])
        }else{
            path = "/usr/src/linux-headers-"+int8ToStr(uname.Release[:])
        }
       
        if _, err := os.Stat(path); !os.IsNotExist(err) {
            return true
        }        
        
	}
     return false
}

/** check if there's any file like $(uname -r) in directory /usr/src/  **/
func checkNodeKernelHeaderPresent(c *k8s.Client, nodeName string, kernelVersion string) bool{
    pods, err := c.K8sClientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
		LabelSelector: "k8s-app=karmor-probe",
        FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return false
	}
    srcPath :=  "/usr/src/"
    fileName := "*"+kernelVersion+"*"
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
    if strings.Contains(s, "No such file or directory") || len(s) == 0{
        return false
    }else{
        return true
    }
    
}



 // A utility to convert the values to proper strings.
func int8ToStr(arr []int8) string {
    b := make([]byte, 0, len(arr))
    for _, v := range arr {
        if v == 0x00 {
            break
        } 
        b = append(b, byte(v))
    }
    return string(b)
}

func checkHostAuditSupport() {
	var uname syscall.Utsname
    if err := syscall.Uname(&uname); err == nil {
        kVersion:= int8ToStr(uname.Release[:])
        s := strings.Split(kVersion, "-")
        kernelVersion := s[0]
        
        _, err := boldWhite.Println("Host:")
        if(err != nil){
            color.Red(" Error")
        }
        fmt.Printf("\t Observability/Audit:") 
        checkAuditSupport(kernelVersion, checkKernelHeaderPresent())
        
	}else{
        color.Red(" Error")
    }
	
}


func getNodeLsmSupport(c *k8s.Client, nodeName string) (string, error) {
    srcPath := "/sys/kernel/security/lsm"
    pods, err := c.K8sClientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
		LabelSelector: "k8s-app=karmor-probe",
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
		return "none",err
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
		return "none",err
	}
    s := string(buf)
    return s, nil
}

func probeNode(c *k8s.Client){
    nodes, _ := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
     if len(nodes.Items) > 0 {
        for i, item := range nodes.Items{
            _, err := boldWhite.Printf("Node %d : \n", i+1)
            if(err != nil){
                color.Red(" Error")
            }
            fmt.Printf("\t Observability/Audit:")
            kernelVersion := item.Status.NodeInfo.KernelVersion    
            check2 := checkNodeKernelHeaderPresent(c, item.Name, kernelVersion)
            checkAuditSupport(kernelVersion, check2)
            lsm, _ := getNodeLsmSupport(c, item.Name)
            checkLsmSupport(lsm)
            
        }
    }else{
        fmt.Println("No kubernetes environment found")
     }

}



type KubeArmorProbeData struct {
	OSImage             string
	KernelVersion       string
	KubeletVersion      string
	ContainerRuntime    string
	SupportedLSMs       string
	KernelHeaderPresent bool
	HostSecurity        bool
	ContainerSecurity   bool
	KubeArmorPosture    KubeArmorPostures
}
type KubeArmorPostures struct {
	DefaultFilePosture             string
	DefaultNetworkPosture          string
	DefaultCapabilitiesPosture     string
	HostDefaultFilePosture         string
	HostDefaultNetworkPosture      string
	HostDefaultCapabilitiesPosture string
}


func isKubeArmorRunning(c *k8s.Client) bool{
	_, err := getKubeArmorDaemonsets(c)
	return err == nil
	
}

func getKubeArmorDaemonsets(c *k8s.Client)  (bool, error) {

	// // KubeArmor DaemonSet
	w, err := c.K8sClientset.AppsV1().DaemonSets("kube-system").Get(context.Background(), "kubearmor", metav1.GetOptions{})
	if err != nil {
		return  false, err
	}
	color.Green("\nFound KubeArmor running in Kubernetes \n\n")
	_, err = boldWhite.Printf("Daemonset : \n")
            if(err != nil){
                color.Red(" Error while printing")
			}
	desired, ready, available := w.Status.DesiredNumberScheduled, w.Status.NumberReady, w.Status.NumberAvailable
	if desired != ready && desired != available {
		fmt.Printf("\t kubearmor \t Desired: %d, Ready: %d, Available: %d \n", desired, ready, available)
		return false, nil
	}
	fmt.Printf("\t kubearmor \t Desired: %d, Ready: %d, Available: %d \n", desired, ready, available)
	return true, nil
}

func getKubeArmorDeployments(c *k8s.Client)  {
	_, err := boldWhite.Printf("Deployments : \n")
            if(err != nil){
                color.Red(" Error while printing")
            }

	//relay deployment
	relayDeployment, err := c.K8sClientset.AppsV1().Deployments("kube-system").Get(context.Background(), "kubearmor-relay", metav1.GetOptions{})
	if err != nil {
		return
	}	
	//not updated replicas- what we need is desired replicas
	desired1, ready1, available1 := relayDeployment.Status.UpdatedReplicas, relayDeployment.Status.ReadyReplicas, relayDeployment.Status.AvailableReplicas 
	fmt.Printf("\t kubearmor-relay \t Desired: %d, Ready: %d, Available: %d \n", desired1, ready1, available1)

	//host policy manager deployment
	hostPolicyDeployment, err := c.K8sClientset.AppsV1().Deployments("kube-system").Get(context.Background(), "kubearmor-host-policy-manager", metav1.GetOptions{})
	if err != nil {
		return
	}	
	//not updated replicas- what we need is desired replicas
	desired2, ready2, available2 := hostPolicyDeployment.Status.UpdatedReplicas, hostPolicyDeployment.Status.ReadyReplicas, hostPolicyDeployment.Status.AvailableReplicas 
	fmt.Printf("\t kubearmor-host-policy-manager \t Desired: %d, Ready: %d, Available: %d \n", desired2, ready2, available2)

	//policy manager deployment
	policyManagerDeployment, err := c.K8sClientset.AppsV1().Deployments("kube-system").Get(context.Background(), "kubearmor-policy-manager", metav1.GetOptions{})
	if err != nil {
		return
	}	
	//not updated replicas- what we need is desired replicas
	desired3, ready3, available3 := policyManagerDeployment.Status.UpdatedReplicas, policyManagerDeployment.Status.ReadyReplicas, policyManagerDeployment.Status.AvailableReplicas 
	fmt.Printf("\t kubearmor-policy-manager \t Desired: %d, Ready: %d, Available: %d \n", desired3, ready3, available3)
}

func printContainers(c *k8s.Client){
	kubearmorContainerCount, kubearmorImageVersion := getKubeArmorContainerCount(c)
	relayContainerCount, relayImageVersion := getKubeArmorRelayContainerCountAndImageVersion(c)
	_, err := boldWhite.Printf("Containers : \n")
            if(err != nil){
                color.Red(" Error while printing")
            }
	fmt.Printf("\t kubearmor \t Running: %d \t <image Version>: %s \n", kubearmorContainerCount, kubearmorImageVersion)
	fmt.Printf("\t kubearmor-relay \t Running: %d \t <image Version>: %s \n", relayContainerCount, relayImageVersion)
	
}

func getKubeArmorContainerCount(c *k8s.Client) (int, string) {
	kubearmorContainerCount := 0;
	kubearmorImageVersion := ""
	kubearmorPods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err != nil {
		log.Println("error occured when getting kubearmor pods", err)
		return kubearmorContainerCount, kubearmorImageVersion
	}

	if len(kubearmorPods.Items) > 0 {
		kubearmorImageVersion = kubearmorPods.Items[0].Spec.Containers[0].Image
		for _, kubearmorPodItem := range kubearmorPods.Items {
			kubearmorContainerCount+= len(kubearmorPodItem.Spec.Containers)			
		}
	}
	
	return kubearmorContainerCount, kubearmorImageVersion

}

func getKubeArmorRelayContainerCountAndImageVersion(c *k8s.Client) (int, string){
	// KubeArmor Relay Pod
	relayContainerCount := 0;
	relayImageVersion := ""
	relayPods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor-relay",
	})
	if err != nil {
		//log.Println("error occured when getting kubearmor relay pods")
		return relayContainerCount,relayImageVersion
	}

	if len(relayPods.Items) > 0 {
		relayImageVersion = relayPods.Items[0].Spec.Containers[0].Image
		for _, relayPodItem := range relayPods.Items {
			relayContainerCount += len(relayPodItem.Spec.Containers)
			
		}
	}
	return relayContainerCount, relayImageVersion
}


func probeRunningKubeArmorNodes(c *k8s.Client) error {

	// // KubeArmor Nodes
	nodes, err := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Println("error getting nodes",err)
		return err
	}

	if len(nodes.Items) > 0 {
		 
		for i, item := range nodes.Items{
            _, err := boldWhite.Printf("Node %d : \n", i+1)
            if(err != nil){
                color.Red(" Error while printing")
            }
            readDataFromKubeArmor(c, item.Name)
            
        }
    }else{
        fmt.Println("No kubernetes environment found")
     }
	 getAnnotatedPods(c)

	return nil
}

func readDataFromKubeArmor(c *k8s.Client, nodeName string) error{
	var kd *KubeArmorProbeData

	srcPath := "/tmp/karmorProbeData.cfg"
	pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
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
		Namespace("kube-system").
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
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err =  json.Unmarshal(buf, &kd)
	if err != nil {
        log.Println("an error occured when parsing file", err)
		return err
	}
	printKubeArmorProbeOutput(kd)
    return nil
}

func printKubeArmorProbeOutput(kd *KubeArmorProbeData){

	fmt.Printf("\t OS Image: ") 
	color.Green( kd.OSImage )
	fmt.Printf("\t Kernel Version: ") 
	color.Green( kd.KernelVersion )
	fmt.Printf("\t Kubelet Version: ") 
	color.Green( kd.KubeletVersion )
	fmt.Printf("\t Container Runtime: ") 
	color.Green( kd.ContainerRuntime )
	fmt.Printf("\t Supported LSMs: ") 
	color.Green( kd.SupportedLSMs )
	fmt.Printf("\t Host Security: ") 
	color.Green(  strconv.FormatBool(kd.HostSecurity) )
	fmt.Printf("\t Container Security: ") 
	color.Green( strconv.FormatBool(kd.ContainerSecurity) )
	fmt.Printf("\t KubeArmor Posture: ") 
	color.Green( kd.KubeArmorPosture.DefaultFilePosture )
}

//sudo systemctl status kubearmor
func isSystemdMode() bool{
	cmd := exec.Command("systemctl", "status", "kubearmor")
	_, err := cmd.CombinedOutput()
	if err != nil {
	  if _, ok := err.(*exec.ExitError); ok {
		return false
	  } else {
		return false
	  }
	}
	color.Green("\nFound KubeArmor running in Systemd mode \n\n")
	return true
  }

  func probeSystemdMode() error{
	var kd *KubeArmorProbeData
    jsonFile, err := os.Open("/tmp/karmorProbeData.cfg")
    if err != nil {
        log.Println(err)
		return err
    }
    defer jsonFile.Close()

    buf, err := ioutil.ReadAll(jsonFile)
    if err != nil {
        log.Println("an error occured when reading file", err)
        return err
    }
    var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err =  json.Unmarshal(buf, &kd)
	if err != nil {
        log.Println("an error occured when parsing file", err)
		return err
	}
	_, err = boldWhite.Printf("Host : \n")
            if(err != nil){
                color.Red(" Error")
            }
	printKubeArmorProbeOutput(kd)
	return nil
  }

  func getAnnotatedPods(c *k8s.Client)error{
	// Annotated Pods Description
	
		pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		_, err = boldWhite.Printf("Annotated pods : \n")
            if(err != nil){
                color.Red(" Error printing bold text")
            }
		fmt.Printf("\t NAMESPACE \t\t NAME\n")

		for _, p := range pods.Items {
			if p.Annotations["kubearmor-policy"] == "enabled" {
				v, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				fmt.Println("\t",v.Namespace,"\t\t ",v.Name)
			}
		}
		return nil
  }