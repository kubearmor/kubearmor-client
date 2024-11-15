// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

var kubearmor = "kubearmor"

var (
	serviceAccountName                               = kubearmor
	operatorServiceAccountName                       = "kubearmor-operator"
	KubeArmorOperatorClusterRoleName                 = "kubearmor-operator-clusterrole"
	KubeArmorOperatorManageClusterRoleName           = "kubearmor-operator-manage-kubearmor-clusterrole"
	KubeArmorOperatorManageControllerClusterRoleName = "kubearmor-operator-manage-controller-clusterrole"
	KubeArmorClusterRoleName                         = "kubearmor-clusterrole"
	RelayClusterRoleName                             = "kubearmor-relay-clusterrole"
	KubeArmorControllerClusterRoleName               = "kubearmor-controller-clusterrole"
	KubeArmorSnitchClusterRoleName                   = "kubearmor-snitch"
	KubeArmorControllerProxyClusterRoleName          = "kubearmor-controller-proxy-role"
)

var (
	KubeArmorSnitchClusterroleBindingName                   = "kubearmor-snitch-binding"
	RelayClusterRoleBindingName                             = "kubearmor-relay-clusterrolebinding"
	KubeArmorControllerProxyClusterRoleBindingName          = "kubearmor-controller-proxy-rolebinding"
	KubeArmorControllerClusterRoleBindingName               = "kubearmor-controller-clusterrolebinding"
	KubeArmorClusterRoleBindingName                         = "kubearmor-clusterrolebinding"
	KubeArmorOperatorManageControllerClusterRoleBindingName = "kubearmor-operator-manage-controller-clusterrole-binding"
	KubeArmorOperatorManageClusterRoleBindingName           = "kubearmor-operator-manage-kubearmor-clusterrole-binding"
	KubeArmorOperatorClusterRoleBindingName                 = "kubearmor-operator-clusterrole-binding"
)

var (
	relayServiceName                = kubearmor
	relayDeploymentName             = "kubearmor-relay"
	policyManagerServiceName        = "kubearmor-policy-manager-metrics-service"
	policyManagerDeploymentName     = "kubearmor-policy-manager"
	hostPolicyManagerServiceName    = "kubearmor-host-policy-manager-metrics-service"
	hostPolicyManagerDeploymentName = "kubearmor-host-policy-manager"
)
