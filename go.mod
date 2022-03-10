module github.com/kubearmor/kubearmor-client

go 1.16

require (
	github.com/cilium/cilium v1.10.0
	github.com/kubearmor/KVMService/src/types v0.0.0-20211201110825-6637c384c9d7
	github.com/kubearmor/KubeArmor/KubeArmor v0.0.0-20211217132903-fd373ac94125
	github.com/kubearmor/KubeArmor/deployments v0.0.0-20220224064008-eb71e99aebc4
	github.com/kubearmor/KubeArmor/pkg/KubeArmorHostPolicy v0.0.0-20220128051912-b9f5851b939b
	github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy v0.0.0-20220128051912-b9f5851b939b
	github.com/kubearmor/KubeArmor/protobuf v0.0.0-20220308043646-0a9827178a4a
	github.com/mholt/archiver/v3 v3.5.1-0.20211001174206-d35d4ce7c5b2
	github.com/rs/zerolog v1.25.0
	github.com/spf13/cobra v1.2.1
	go.mongodb.org/mongo-driver v1.8.4 // indirect
	golang.org/x/mod v0.5.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.22.3
	k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/cli-runtime v0.22.3
	k8s.io/client-go v0.22.3
	sigs.k8s.io/yaml v1.3.0
)
