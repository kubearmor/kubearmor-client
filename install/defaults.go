// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package install

var kubearmor = "kubearmor"
var port int32 = 32767

var serviceAccountName = kubearmor
var clusterRoleBindingName = kubearmor
var relayServiceName = kubearmor
var relayDeploymentName = "kubearmor-relay"
var policyManagerServiceName = "kubearmor-policy-manager-metrics-service"
var policyManagerDeploymentName = "kubearmor-policy-manager"
var hostPolicyManagerServiceName = "kubearmor-host-policy-manager-metrics-service"
var hostPolicyManagerDeploymentName = "kubearmor-host-policy-manager"
