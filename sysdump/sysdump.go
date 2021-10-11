package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/kubearmor/kubearmor-client/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/mholt/archiver/v3"
)

func Collect(c *k8s.Client) error {

	d, err := os.MkdirTemp("", "karmor-sysdump")
	if err != nil {
		return err
	}

	// k8s Server Version
	{
		v, err := c.K8sClientset.Discovery().ServerVersion()
		if err != nil {
			return err
		}
		if err := writeToFile(path.Join(d, "version.txt"), v.String()); err != nil {
			return err
		}
	}

	// Node Info
	{
		v, err := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "node-info.yaml"), v); err != nil {
			return err
		}
	}

	// KubeArmor DaemonSet
	{
		v, err := c.K8sClientset.AppsV1().DaemonSets("kube-system").Get(context.Background(), "kubearmor", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "kubearmor-daemonset.yaml"), v); err != nil {
			return err
		}
	}

	// KubeArmor Security Policies
	{
		v, err := c.KSPClientset.KubeArmorPolicies("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "ksp.yaml"), v); err != nil {
			return err
		}
	}

	// KubeArmor Logs
	{
		pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app=kubearmor",
		})
		if err != nil {
			return err
		}

		for _, p := range pods.Items {
			v := c.K8sClientset.CoreV1().Pods("kube-system").GetLogs(p.Name, &corev1.PodLogOptions{})
			s, err := v.Stream(context.Background())
			if err != nil {
				return err
			}
			defer s.Close()
			var b bytes.Buffer
			if _, err = io.Copy(&b, s); err != nil {
				return err
			}
			if err := writeToFile(path.Join(d, "ka-pod-"+p.Name+"-log.txt"), b.String()); err != nil {
				return err
			}
		}
	}

	// AppArmor Gzip
	{
		if err := copyFromPod("/etc/apparmor.d/", path.Join(d, "apparmor.tar.gz"), c); err != nil {
			return err
		}
	}

	sysdumpFile := "karmor-sysdump-" + time.Now().Format(time.UnixDate) + ".zip"

	if err := archiver.Archive([]string{d}, sysdumpFile); err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	if err := os.RemoveAll(d); err != nil {
		return err
	}

	fmt.Printf("Sysdump at %s\n", sysdumpFile)

	return nil
}

func writeToFile(p, v string) error {
	return os.WriteFile(p, []byte(v), 0600)
}

func writeYaml(p string, o runtime.Object) error {
	var j printers.YAMLPrinter
	w, err := printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&j, nil)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := w.PrintObj(o, &b); err != nil {
		return err
	}
	return writeToFile(p, b.String())
}

func copyFromPod(srcPath string, destPath string, c *k8s.Client) error {
	pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err != nil {
		return err
	}
	reader, outStream := io.Pipe()
	cmdArr := []string{"tar", "cf", "-", srcPath}
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
		return err
	}
	if err := os.WriteFile(destPath, buf, 0600); err != nil {
		return err
	}
	return nil
}
