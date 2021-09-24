module github.com/kubearmor/kubearmor-client

go 1.16

require (
	github.com/kubearmor/KubeArmor/pkg/KubeArmorHostPolicy v0.0.0-20210922085805-a58e667065ab
	github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy v0.0.0-20210922085805-a58e667065ab
	github.com/kubearmor/KubeArmor/protobuf v0.0.0-20210915063509-49cf6deba1ce // indirect
	github.com/kubearmor/kubearmor-log-client/common v0.0.0-20210706110248-699fa8535e5c // indirect
	github.com/kubearmor/kubearmor-log-client/core v0.0.0-20210706110248-699fa8535e5c
	github.com/rs/zerolog v1.24.0
	github.com/spf13/cobra v1.2.1
	golang.org/x/mod v0.4.2
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/cli-runtime v0.22.1
	k8s.io/client-go v0.22.1
)
