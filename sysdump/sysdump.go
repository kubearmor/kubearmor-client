package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/kubearmor/kubearmor-client/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"

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

	if err := archiver.Archive([]string{d}, "karmor-sysdump.zip"); err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	if err := os.RemoveAll(d); err != nil {
		return err
	}

	return nil
}

func writeToFile(p, v string) error {
	return os.WriteFile(p, []byte(v), 0666)
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
