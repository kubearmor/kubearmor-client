package recommend

import "github.com/kubearmor/kubearmor-client/recommend/common"

type Client interface {
	ListDeployments(o common.Options) ([]Deployment, error)
}
