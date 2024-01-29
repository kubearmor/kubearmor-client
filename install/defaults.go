// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

var kubearmor = "kubearmor"

var serviceAccountName = kubearmor
var operatorServiceAccountName = "kubearmor-operator"
var KubeArmorOperatorClusterRoleName = "kubearmor-operator-clusterrole"
var KubeArmorOperatorManageClusterRoleName = "kubearmor-operator-manage-kubearmor-clusterrole"
var KubeArmorOperatorManageControllerClusterRoleName = "kubearmor-operator-manage-controller-clusterrole"
var KubeArmorClusterRoleName = "kubearmor-clusterrole"
var RelayClusterRoleName = "kubearmor-relay-clusterrole"
var KubeArmorControllerClusterRoleName = "kubearmor-controller-clusterrole"
var KubeArmorSnitchClusterRoleName = "kubearmor-snitch"
var KubeArmorControllerProxyClusterRoleName = "kubearmor-controller-proxy-role"

var KubeArmorSnitchClusterroleBindingName = "kubearmor-snitch-binding"
var RelayClusterRoleBindingName = "kubearmor-relay-clusterrolebinding"
var KubeArmorControllerProxyClusterRoleBindingName = "kubearmor-controller-proxy-rolebinding"
var KubeArmorControllerClusterRoleBindingName = "kubearmor-controller-clusterrolebinding"
var KubeArmorClusterRoleBindingName = "kubearmor-clusterrolebinding"
var KubeArmorOperatorManageControllerClusterRoleBindingName = "kubearmor-operator-manage-controller-clusterrole-binding"
var KubeArmorOperatorManageClusterRoleBindingName = "kubearmor-operator-manage-kubearmor-clusterrole-binding"
var KubeArmorOperatorClusterRoleBindingName = "kubearmor-operator-clusterrole-binding"

var relayServiceName = kubearmor
var relayDeploymentName = "kubearmor-relay"
var policyManagerServiceName = "kubearmor-policy-manager-metrics-service"
var policyManagerDeploymentName = "kubearmor-policy-manager"
var hostPolicyManagerServiceName = "kubearmor-host-policy-manager-metrics-service"
var hostPolicyManagerDeploymentName = "kubearmor-host-policy-manager"
