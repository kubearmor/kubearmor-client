package engines

import (
	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
)

type Engine interface {
	Init() error
	Scan(img *image.ImageInfo, options common.Options, tags []string) error
}
