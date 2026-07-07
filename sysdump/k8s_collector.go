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

	"golang.org/x/sync/errgroup"

	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type K8sCollector struct {
	k8sClient *k8s.Client
	options   Options
}

func NewK8sCollector(c *k8s.Client, o Options) *K8sCollector {
	return &K8sCollector{
		k8sClient: c,
		options:   o,
	}
}

func (kc *K8sCollector) Collect(d string) error {
	var errs errgroup.Group

	errs.Go(func() error {
		v, err := kc.k8sClient.K8sClientset.Discovery().ServerVersion()
		if err != nil {
			return err
		}
		return writeToFile(path.Join(d, "version.txt"), v.String())
	})

	errs.Go(func() error {
		v, err := kc.k8sClient.K8sClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		return writeYaml(path.Join(d, "node-info.yaml"), v)
	})

	errs.Go(func() error {
		v, err := kc.k8sClient.K8sClientset.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app=kubearmor",
		})
		if err != nil {
			fmt.Printf("kubearmor daemonset not found\n")
			return nil
		}
		return writeYaml(path.Join(d, "kubearmor-daemonset.yaml"), v)
	})

	errs.Go(func() error {
		v, err := kc.k8sClient.KSPClientset.KubeArmorPolicies("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("kubearmor CRD not found!\n")
			return nil
		}
		return writeYaml(path.Join(d, "ksp.yaml"), v)
	})

	errs.Go(func() error {
		pods, err := kc.k8sClient.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			LabelSelector: "kubearmor-app",
		})
		if err != nil {
			fmt.Printf("kubearmor pod not found\n")
			return nil
		}
		fmt.Print("Checking all pods labeled with kubearmor-app\n")

		for _, p := range pods.Items {
			for _, container := range p.Spec.Containers {
				fmt.Printf("getting logs from pod=%s container=%s\n", p.Name, container.Name)
				v := kc.k8sClient.K8sClientset.CoreV1().Pods(p.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: container.Name})
				s, err := v.Stream(context.Background())
				if err != nil {
					fmt.Printf("failed getting logs from pod=%s err=%s\n", p.Name, err)
					continue
				}
				defer s.Close()
				var logs bytes.Buffer
				if _, err = io.Copy(&logs, s); err != nil {
					return err
				}
				if err := writeToFile(path.Join(d, "ka-pod-"+p.Name+"-log.txt"), logs.String()); err != nil {
					return err
				}
			}

			pod, err := kc.k8sClient.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, "ka-pod-"+p.Name+".yaml"), pod); err != nil {
				return err
			}

			e, err := kc.k8sClient.K8sClientset.CoreV1().Events(p.Namespace).Search(scheme.Scheme, pod)
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, "ka-pod-events-"+p.Name+".yaml"), e); err != nil {
				return err
			}
		}
		return nil
	})

	errs.Go(func() error {
		pods, err := kc.k8sClient.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, p := range pods.Items {
			v, err := kc.k8sClient.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, p.Namespace+"-pod-"+p.Name+".yaml"), v); err != nil {
				return err
			}
			e, err := kc.k8sClient.K8sClientset.CoreV1().Events(p.Namespace).Search(scheme.Scheme, v)
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, p.Namespace+"-pod-events-"+p.Name+".yaml"), e); err != nil {
				return err
			}
		}
		return nil
	})

	errs.Go(func() error {
		if err := kc.copyAppArmorFromPod("/etc/apparmor.d", d); err != nil {
			kg.Warnf("Failed to copy AppArmor profiles: %v\n", err)
			return nil
		}
		return nil
	})

	errs.Go(func() error {
		reader, writer, err := os.Pipe()
		if err != nil {
			return err
		}
		err = probe.PrintProbeResultCmd(kc.k8sClient, probe.Options{
			Namespace: "",
			Full:      false,
			Output:    "no-color",
			GRPC:      "",
			Writer:    writer,
		})
		if err != nil {
			writer.Close()
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

	return errs.Wait()
}

func (kc *K8sCollector) copyAppArmorFromPod(srcPath string, d string) error {
	pods, err := kc.k8sClient.K8sClientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
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
		req := kc.k8sClient.K8sClientset.CoreV1().RESTClient().
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
		exec, err := remotecommand.NewSPDYExecutor(kc.k8sClient.Config, "POST", req.URL())
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
