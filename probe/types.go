package probe

import (
	"io"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
)

// Options provides probe daemonset options install
type Options struct {
	Namespace string
	Full      bool
	Output    string
	GRPC      string
	Writer    io.Writer
}

// KubeArmorProbeData structure definition
type KubeArmorProbeData struct {
	OSImage                 string
	KernelVersion           string
	KubeletVersion          string
	ContainerRuntime        string
	ActiveLSM               string
	KernelHeaderPresent     bool
	HostSecurity            bool
	ContainerSecurity       bool
	ContainerDefaultPosture tp.DefaultPosture
	HostDefaultPosture      tp.DefaultPosture
	HostVisibility          string
}

// Status data
type Status struct {
	Desired   string `json:"desired"`
	Ready     string `json:"ready"`
	Available string `json:"available"`
}

// KubeArmorPodSpec structure definition
type KubeArmorPodSpec struct {
	Running       string `json:"running"`
	Image_Version string `json:"image_version"`
}

// NamespaceData structure definition
type NamespaceData struct {
	NsPostureString    string            `json:"-"`
	NsVisibilityString string            `json:"-"`
	NsDefaultPosture   tp.DefaultPosture `json:"default_posture"`
	NsVisibility       Visibility        `json:"visibility"`
	NsPodList          []PodInfo         `json:"pod_list"`
}

// Visibility data structure definition
type Visibility struct {
	File         bool `json:"file"`
	Capabilities bool `json:"capabilities"`
	Process      bool `json:"process"`
	Network      bool `json:"network"`
}

// PodInfo structure definition
type PodInfo struct {
	PodName string `json:"pod_name"`
	Policy  string `json:"policy"`
}
