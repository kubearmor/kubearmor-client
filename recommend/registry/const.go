package registry

import "errors"

const (
	DefaultTempDirPrefix = "karmor-store-"
	DefaultRegistry      = "docker.io"
	DefaultTag           = "latest"

	artifactType = "application/vnd.cncf.kubearmor.config.v1+json"
	mediaType    = "application/vnd.cncf.kubearmor.policy.layer.v1.yaml"

	// Connect to remote repository via HTTP instead of HTTPS when
	// set to "true".
	EnvOCIInsecure = "KARMOR_OCI_TLS_INSECURE"
)

var (
	ErrInvalidImage = errors.New("invalid image path")
)