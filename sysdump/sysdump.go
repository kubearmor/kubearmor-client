// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package sysdump collects and dumps information for troubleshooting KubeArmor
package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/mholt/archiver/v3"
)

// Options options for sysdump
type Options struct {
	Filename string
}

// Collect Function
func Collect(c *k8s.Client, o Options) error {
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
		v, err := c.K8sClientset.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app=kubearmor",
		})
		if err != nil {
			fmt.Printf("kubearmor daemonset not found. (possible if kubearmor is running in process mode)\n")
			return nil
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
			fmt.Printf("kubearmor CRD not found!\n")
			return nil
		}
		if err := writeYaml(path.Join(d, "ksp.yaml"), v); err != nil {
			return err
		}
		return nil
	})

	// KubeArmor Pod
	errs.Go(func() error {
		pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app",
		})
		if err != nil {
			fmt.Printf("kubearmor pod not found. (possible if kubearmor is running in process mode)\n")
			return nil
		}
		fmt.Print("Checking all pods labeled with kubearmor-app\n")

		for _, p := range pods.Items {
			// Iterate over containers in the pod
			for _, container := range p.Spec.Containers {

				// KubeArmor Logs
				fmt.Printf("getting logs from pod=%s container=%s\n", p.Name, container.Name)
				v := c.K8sClientset.CoreV1().Pods(p.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: container.Name})
				s, err := v.Stream(context.Background())
				if err != nil {
					fmt.Printf("failed getting logs from pod=%s err=%s\n", p.Name, err)
					continue
				}
				defer func() {
					if err := s.Close(); err != nil {
						kg.Warnf("Error closing io stream %s\n", err)
					}
				}()
				var logs bytes.Buffer
				if _, err = io.Copy(&logs, s); err != nil {
					return err
				}
				if err := writeToFile(path.Join(d, "ka-pod-"+p.Name+"-log.txt"), logs.String()); err != nil {
					return err
				}
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
		return nil
	})

	// AppArmor GzipS
	errs.Go(func() error {
		if err := copyFromPod("/etc/apparmor.d", d, c); err != nil {
			return err
		}
		return nil
	})
	// Saves the probe data in the zip file
	errs.Go(func() error {
		reader, writer, err := os.Pipe()
		if err != nil {
			return err
		}
		err = probe.PrintProbeResultCmd(c, probe.Options{
			Namespace: "",
			Full:      false,
			Output:    "no-color",
			GRPC:      "",
			Writer:    writer,
		})
		if err != nil {
			return err
		}
		err = writer.Close()
		if err != nil {
			return err
		}
		out, _ := io.ReadAll(reader)
		err = writeToFile(path.Join(d, "karmor-probe.txt"), string(out))
		if err != nil {
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

	sysdumpFile := ""
	if o.Filename == "" {
		sysdumpFile = "karmor-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	} else {
		sysdumpFile = o.Filename
	}

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
	return os.WriteFile(p, []byte(v), 0o600)
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

func copyFromPod(srcPath string, d string, c *k8s.Client) error {
	pods, err := c.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err != nil {
		fmt.Printf("kubearmor not deployed in pod mode\n")
		return nil
	}
	for _, pod := range pods.Items {
		destPath := path.Join(d, fmt.Sprintf("%s_apparmor.tar.gz", pod.Name))
		reader, outStream := io.Pipe()
		cmdArr := []string{"tar", "czf", "-", srcPath}
		req := c.K8sClientset.CoreV1().RESTClient().
			Get().
			Namespace(pod.Namespace).
			Resource("pods").
			Name(pod.Name).
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
		if err := os.WriteFile(destPath, buf, 0o600); err != nil {
			return err
		}
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
