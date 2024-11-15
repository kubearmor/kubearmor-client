package utils

type VMMode string

const (
	VMMode_Docker  VMMode = "docker"
	VMMode_Systemd VMMode = "systemd"

	MinDockerVersion                  = "v19.0.3"
	MinDockerComposeVersion           = "v1.27.0"
	MinDockerComposeWithWaitSupported = "v2.17.0"
	DefaultConfigPathDirName          = ".kubearmor-config"
	DefaultDockerTag                  = "stable"
	// systemd path
	DownloadDir           string = "/tmp/kubearmor-downloads/"
	SystemdDir            string = "/usr/lib/systemd/system/"
	KubeArmorPort                = "32767"
	DefaultDockerRegistry        = "docker.io"
	// KubeArmor related image/image registries are fixed as of now
	DefaultKubeArmorRepo                = "kubearmor"
	DefaultKubeArmorImage               = "kubearmor/kubearmor"
	DefaultKubeArmorInitImage           = "kubearmor/kubearmor-init"
	DefaultKubeArmorSystemdImage        = "kubearmor/kubearmor-systemd"
	KAconfigPath                 string = "/opt/kubearmor/"
)
