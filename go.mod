module github.com/kubearmor/kubearmor-client

go 1.16

require (
	github.com/kubearmor/KubeArmor/pkg/KubeArmorHostPolicy v0.0.0-20211028102308-7c7d59ec12b4
	github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy v0.0.0-20211028102308-7c7d59ec12b4
	github.com/kubearmor/KubeArmor/protobuf v0.0.0-20211028102308-7c7d59ec12b4 // indirect
	github.com/kubearmor/kubearmor-log-client/common v0.0.0-20210706110248-699fa8535e5c // indirect
	github.com/kubearmor/kubearmor-log-client/core v0.0.0-20210706110248-699fa8535e5c
	github.com/mholt/archiver/v3 v3.5.1-0.20211001174206-d35d4ce7c5b2
	github.com/rs/zerolog v1.25.0
	github.com/spf13/cobra v1.2.1
	golang.org/x/mod v0.5.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	k8s.io/api v0.22.3
	k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/cli-runtime v0.22.3
	k8s.io/client-go v0.22.3
)
