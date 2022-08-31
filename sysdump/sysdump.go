// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kubearmor/kubearmor-client/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/mholt/archiver/v3"
)

// Collect Function
func Collect(c *k8s.Client) error {
	var errs errgroup.Group

	d, err := os.MkdirTemp("", "karmor-sysdump")
	if err != nil {
		return err
	}

	// k8s Server Version
	errs.Go(func() error {
		v, err := c.K8sClientset.Discovery().ServerVersion()
		if err != nil {
			return err
		}
		if err := writeToFile(path.Join(d, "version.txt"), v.String()); err != nil {
			return err
		}
		return nil
	})

	// Node Info
	errs.Go(func() error {
		v, err := c.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "node-info.yaml"), v); err != nil {
			return err
		}
		return nil
	})

	// KubeArmor DaemonSet
	errs.Go(func() error {
		v, err := c.K8sClientset.AppsV1().DaemonSets("kube-system").Get(context.Background(), "kubearmor", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "kubearmor-daemonset.yaml"), v); err != nil {
			return err
		}
		return nil
	})

	// KubeArmor Security Policies
	errs.Go(func() error {
		v, err := c.KSPClientset.KubeArmorPolicies("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if err := writeYaml(path.Join(d, "ksp.yaml"), v); err != nil {
			return err
		}
		return nil
	})

	// KubeArmor Pod
	errs.Go(func() error {
		pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app=kubearmor",
		})
		if err != nil {
			return err
		}

		for _, p := range pods.Items {
			// KubeArmor Logs
			v := c.K8sClientset.CoreV1().Pods("kube-system").GetLogs(p.Name, &corev1.PodLogOptions{})
			s, err := v.Stream(context.Background())
			if err != nil {
				return err
			}
			defer s.Close()
			var logs bytes.Buffer
			if _, err = io.Copy(&logs, s); err != nil {
				return err
			}
			if err := writeToFile(path.Join(d, "ka-pod-"+p.Name+"-log.txt"), logs.String()); err != nil {
				return err
			}

			// KubeArmor Describe
			pod, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, "ka-pod-"+p.Name+".yaml"), pod); err != nil {
				return err
			}

			// KubeArmor Event
			e, err := c.K8sClientset.CoreV1().Events(p.Namespace).Search(scheme.Scheme, pod)
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, "ka-pod-events-"+p.Name+".yaml"), e); err != nil {
				return err
			}
		}
		return nil
	})

	// Annotated Pods Description
	errs.Go(func() error {
		pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, p := range pods.Items {
			if p.Annotations["kubearmor-policy"] == "enabled" {
				v, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if err := writeYaml(path.Join(d, p.Namespace+"-pod-"+p.Name+".yaml"), v); err != nil {
					return err
				}
				e, err := c.K8sClientset.CoreV1().Events(p.Namespace).Search(scheme.Scheme, v)
				if err != nil {
					return err
				}
				if err := writeYaml(path.Join(d, p.Namespace+"-pod-events-"+p.Name+".yaml"), e); err != nil {
					return err
				}
			}
		}
		return nil
	})

	// AppArmor Gzip
	errs.Go(func() error {
		if err := copyFromPod("/etc/apparmor.d", path.Join(d, "apparmor.tar.gz"), c); err != nil {
			return err
		}
		return nil
	})

	dumpError := errs.Wait()

	emptyDump, err := IsDirEmpty(d)
	if err != nil {
		return err
	}

	if emptyDump {
		return dumpError
	}

	sysdumpFile := "karmor-sysdump-" + time.Now().Format(time.UnixDate) + ".zip"

	if err := archiver.Archive([]string{d}, sysdumpFile); err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	if err := os.RemoveAll(d); err != nil {
		return err
	}

	fmt.Printf("Sysdump at %s\n", sysdumpFile)

	if dumpError != nil {
		return dumpError
	}

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

// IsDirEmpty Function
func IsDirEmpty(name string) (bool, error) {
	files, err := os.ReadDir(name)

	if err != nil {
		return false, err
	}

	if len(files) != 0 {
		return false, nil
	}

	return true, nil
}
