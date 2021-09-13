module github.com/kubearmor/kubearmor-client

go 1.16

replace github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy => github.com/daemon1024/KubeArmor/pkg/KubeArmorPolicy v0.0.0-20210909152203-f9d3cb0a82e2

require (
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy v0.0.0-20210907115459-ac6861442e2e
	github.com/rs/zerolog v1.24.0
	github.com/spf13/cobra v1.2.1
	golang.org/x/sys v0.0.0-20210817190340-bfb29a6856f2 // indirect
	k8s.io/apimachinery v0.22.1
	k8s.io/cli-runtime v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176 // indirect
)
