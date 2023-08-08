// Package engines provides interfaces and implementations for policy generation
package engines

import (
	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
)

// Engine interface used by policy generators to generate policies
type Engine interface {
	Init() error
	Scan(img *image.Info, options common.Options) (map[string][]byte, map[string]interface{}, error)
}
