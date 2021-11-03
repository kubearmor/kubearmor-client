// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var serviceAccountName = "kubearmor"

var serviceAccount = &corev1.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{
		Name: serviceAccountName,
	},
}

var clusterRoleBindingName = "kubearmor"

var clusterRoleBinding = &rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleBindingName,
	},
	RoleRef: rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "cluster-admin",
	},
	Subjects: []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "kubearmor",
			Namespace: "kube-system",
		},
	},
}

var relayServiceName = "kubearmor"

var relayService = &corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: relayServiceName,
	},
	Spec: corev1.ServiceSpec{
		Selector: relayDeploymentLabels,
		Ports: []corev1.ServicePort{
			{
				Port: 32767,
				//Protocol is by default TCP so no need to mention
			},
		},
	},
}

var replicas = int32(1)

var relayDeploymentLabels = map[string]string{
	"kubearmor-app": "kubearmor-relay",
}

var relayDeploymentName = "kubearmor-relay"

var relayDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:   relayDeploymentName,
		Labels: relayDeploymentLabels,
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: relayDeploymentLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"kubearmor-policy": "audited",
				},
				Labels: relayDeploymentLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "kubearmor",
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Containers: []corev1.Container{
					{
						Name:  "kubearmor-relay-server",
						Image: "kubearmor/kubearmor-relay-server:latest",
						//imagePullPolicy is Always since image has latest tag
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 32767,
							},
						},
					},
				},
			},
		},
	},
}

var terminationGracePeriodSeconds = int64(10)

var policyManagerDeploymentLabels = map[string]string{
	"kubearmor-app": "kubearmor-policy-manager",
}

var policyManagerServiceName = "kubearmor-policy-manager-metrics-service"

var policyManagerService = &corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:   policyManagerServiceName,
		Labels: policyManagerDeploymentLabels,
	},
	Spec: corev1.ServiceSpec{
		Selector: policyManagerDeploymentLabels,
		Ports: []corev1.ServicePort{
			{
				Name:       "https",
				Port:       32767,
				TargetPort: intstr.FromString("https"),
			},
		},
	},
}

var policyManagerDeploymentName = "kubearmor-policy-manager"

var policyManagerDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:   policyManagerDeploymentName,
		Labels: policyManagerDeploymentLabels,
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: policyManagerDeploymentLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"kubearmor-policy": "audited",
				},
				Labels: policyManagerDeploymentLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "kubearmor",
				Containers: []corev1.Container{
					{
						Name:  "kube-rbac-proxy",
						Image: "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0",
						Args: []string{
							"--secure-listen-address=0.0.0.0:8443",
							"--upstream=http://127.0.0.1:8080/",
							"--logtostderr=true",
							"--v=10",
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8443,
								Name:          "https",
							},
						},
					},
					{
						Name:  "kubearmor-policy-manager",
						Image: "kubearmor/kubearmor-policy-manager:latest",
						Args: []string{
							"--metrics-addr=127.0.0.1:8080",
							"--enable-leader-election",
						},
						Command: []string{"/manager"},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("30Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
					},
				},
				TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			},
		},
	},
}

var hostPolicyManagerDeploymentLabels = map[string]string{
	"kubearmor-app": "kubearmor-host-policy-manager",
}

var hostPolicyManagerServiceName = "kubearmor-host-policy-manager-metrics-service"

var hostPolicyManagerService = &corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:   hostPolicyManagerServiceName,
		Labels: hostPolicyManagerDeploymentLabels,
	},
	Spec: corev1.ServiceSpec{
		Selector: hostPolicyManagerDeploymentLabels,
		Ports: []corev1.ServicePort{
			{
				Name:       "https",
				Port:       8443,
				TargetPort: intstr.FromString("https"),
			},
		},
	},
}

var hostPolicyManagerDeploymentName = "kubearmor-host-policy-manager"

var hostPolicyManagerDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:   hostPolicyManagerDeploymentName,
		Labels: hostPolicyManagerDeploymentLabels,
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: hostPolicyManagerDeploymentLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"kubearmor-policy": "audited",
				},
				Labels: hostPolicyManagerDeploymentLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "kubearmor",
				Containers: []corev1.Container{
					{
						Name:  "kube-rbac-proxy",
						Image: "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0",
						Args: []string{
							"--secure-listen-address=0.0.0.0:8443",
							"--upstream=http://127.0.0.1:8080/",
							"--logtostderr=true",
							"--v=10",
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8443,
								Name:          "https",
							},
						},
					},
					{
						Name:  "kubearmor-host-policy-manager",
						Image: "kubearmor/kubearmor-policy-manager:latest",
						Args: []string{
							"--metrics-addr=127.0.0.1:8080",
							"--enable-leader-election",
						},
						Command: []string{"/manager"},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("30Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
					},
				},
				TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			},
		},
	},
}

func generateDaemonSet(env string) *appsv1.DaemonSet {

	var label = map[string]string{
		"kubearmor-app": "kubearmor",
	}
	var privileged = bool(true)
	var args = []string{
		"-gRPC=32767",
		"-logPath=/tmp/kubearmor.log",
	}
	var volumeMounts = []corev1.VolumeMount{
		{
			Name:      "usr-src-path", //BPF (read-only)
			MountPath: "/usr/src",
			ReadOnly:  true,
		},
		{
			Name:      "lib-modules-path", //BPF (read-only)
			MountPath: "/lib/modules",
			ReadOnly:  true,
		},
		{
			Name:      "sys-fs-bpf-path", //BPF (read-write)
			MountPath: "/sys/fs/bpf",
		},
		{
			Name:      "sys-kernel-debug-path", //BPF (read-only)
			MountPath: "/sys/kernel/debug",
		},
		{
			Name:      "os-release-path", //BPF (read-only)
			MountPath: "/media/root/etc/os-release",
			ReadOnly:  true,
		},
	}
	var terminationGracePeriodSeconds = int64(30)

	var hostPathDirectory = corev1.HostPathDirectory
	var hostPathDirectoryOrCreate = corev1.HostPathDirectoryOrCreate
	var hostPathFile = corev1.HostPathFile
	var hostPathSocket = corev1.HostPathSocket

	var volumes = []corev1.Volume{
		{
			Name: "usr-src-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/src",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "lib-modules-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/lib/modules",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys-fs-bpf-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys/fs/bpf",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys-kernel-debug-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys/kernel/debug",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "os-release-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/os-release",
					Type: &hostPathFile,
				},
			},
		},
	}

	// Don't enable host policy in minikube and microk8s
	if env != "minikube" && env != "microk8s" && env != "eks" {
		args = append(args, "-enableKubeArmorHostPolicy")
	}

	// Don't Mount AppArmor in Minikube
	if env != "minikube" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "etc-apparmor-d-path",
			MountPath: "/etc/apparmor.d",
		})
		volumes = append(volumes, corev1.Volume{
			Name: "etc-apparmor-d-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/apparmor.d",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		})
	}

	// Mount Socket accourding to Container Runtime Environment
	if env == "docker" || env == "minikube" || env == "eks" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "docker-sock-path", // docker (read-only)
			MountPath: "/var/run/docker.sock",
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "docker-sock-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/docker.sock",
					Type: &hostPathSocket,
				},
			},
		})
	} else if env == "microk8s" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "containerd-sock-path", // containerd
			MountPath: "/var/run/containerd/containerd.sock",
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "containerd-sock-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/snap/microk8s/common/run/containerd.sock",
					Type: &hostPathSocket,
				},
			},
		})
	} else {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "containerd-sock-path", // containerd
			MountPath: "/var/run/containerd/containerd.sock",
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "containerd-sock-path",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/containerd/containerd.sock",
					Type: &hostPathSocket,
				},
			},
		})
	}

	var daemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kubearmor",
			Labels: label,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
					Annotations: map[string]string{
						"container.apparmor.security.beta.kubernetes.io/kubearmor": "unconfined",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kubearmor",
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: "Exists",
						},
					},
					HostPID:       true,
					HostNetwork:   true,
					RestartPolicy: "Always",
					DNSPolicy:     "ClusterFirstWithHostNet",
					Containers: []corev1.Container{
						{
							Name:  "kubearmor",
							Image: "kubearmor/kubearmor:latest",
							//imagePullPolicy is Always since image has latest tag
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Args: args,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 32767,
								},
							},
							VolumeMounts: volumeMounts,
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/bash",
											"-c",
											"if [ -z $(pgrep kubearmor) ]; then exit 1; fi;",
										},
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       10,
							},
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
	return daemonSet
}
