// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package deployment contains configuration for the daemonset deployment we leverage to probe into k8s cluster
package deployment

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Karmorprobe is the identifier for the daemonset we use to probe into k8s cluster
var Karmorprobe = "karmor-probe"

// GenerateDaemonSet Function
func GenerateDaemonSet(namespace string, krnhdr bool) *appsv1.DaemonSet {
	label := map[string]string{
		"kubearmor-app": Karmorprobe,
	}
	privileged := bool(true)
	terminationGracePeriodSeconds := int64(30)
	args := []string{
		"while true; do sleep 30; done;",
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "lsm-path", // lsm (read-only)
			MountPath: "/sys/kernel/security",
			ReadOnly:  true,
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "lsm-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys/kernel/security",
				},
			},
		},
	}

	if krnhdr {
		volumeMounts = append(volumeMounts, []corev1.VolumeMount{
			{
				Name:      "lib-modules", // lib modules (read-only)
				MountPath: "/lib/modules",
				ReadOnly:  true,
			},
			{
				Name:      "kernel-header", // kernel header (read-only)
				MountPath: "/usr/src",
				ReadOnly:  true,
			},
		}...)
		volumes = append(volumes, []corev1.Volume{
			{
				Name: "lib-modules",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/lib/modules",
					},
				},
			},
			{
				Name: "kernel-header",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/usr/src",
					},
				},
			},
		}...)
	}

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      Karmorprobe,
			Labels:    label,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: "Exists",
							Key:      "node-role.kubernetes.io/control-plane",
							Effect:   "NoSchedule",
						},
					},
					RestartPolicy: "Always",
					Containers: []corev1.Container{
						{
							Name:            Karmorprobe,
							Image:           "alpine",
							ImagePullPolicy: "Always",
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Command: []string{
								"/bin/sh",
								"-c",
								"--",
							},
							Args: args,

							VolumeMounts: volumeMounts,

							TerminationMessagePolicy: "File",
							TerminationMessagePath:   "/dev/termination-log",
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Volumes:                       volumes,
				},
			},
		},
	}
}
