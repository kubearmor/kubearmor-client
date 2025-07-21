package docker

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/kubearmor/kubearmor-client/recommend/common"
)

type Client struct {
	*client.Client
}

func ConnectDockerClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli}, nil
}

func (c *Client) ListObjects(_ common.Options) ([]common.Object, error) {
	var result []common.Object
	containers, err := c.Client.ContainerList(context.Background(), container.ListOptions{
		Filters: filters.NewArgs(),
	})
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		result = append(result, common.Object{
			Name:   strings.TrimPrefix(ctr.Names[0], "/"),
			Images: []string{ctr.Image},
			Labels: ctr.Labels,
		})
	}
	return result, nil
}
