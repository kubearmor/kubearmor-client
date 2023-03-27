// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package summary shows observability data from discovery engine
package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultReqType : default option for request type
var DefaultReqType = "process,file,network"
var matchLabels = map[string]string{"app": "discovery-engine"}
var port int64 = 9089

// Options Structure
type Options struct {
	GRPC          string
	Labels        string
	Namespace     string
	PodName       string
	ClusterName   string
	ContainerName string
	Type          string
	Output        string
	RevDNSLookup  bool
	Aggregation   bool
	DeployName    string
	DeployType    string
}

// GetSummary on pods
func GetSummary(c *k8s.Client, o Options) ([]string, error) {
	var str []string
	gRPC := ""
	targetSvc := "discovery-engine"

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
			gRPC = val
		} else {
			pf, err := utils.InitiatePortForward(c, port, port, matchLabels, targetSvc)
			if err != nil {
				return nil, err
			}
			gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
		}
	}

	data := &opb.Request{
		Label:         o.Labels,
		NameSpace:     o.Namespace,
		PodName:       o.PodName,
		ClusterName:   o.ClusterName,
		ContainerName: o.ContainerName,
		Aggregate:     o.Aggregation,
		Type:          o.Type,
		DeployName:    o.DeployName,
	}

	switch o.DeployType {
	case "sts", "statefulset", "StatefulSet", "statefulsets", "StatefulSets":
		data.DeployType = "StatefulSet"
		o.DeployType = "StatefulSet"
	case "rs", "replicaset", "ReplicaSet", "replicasets", "ReplicaSets":
		data.DeployType = "ReplicaSet"
		o.DeployType = "ReplicaSet"
	case "ds", "daemonset", "DaemonSet", "daemonsets", "DaemonSets":
		data.DeployType = "DaemonSet"
		o.DeployType = "DaemonSet"
	case "deploy", "Deployment", "Deployments":
		data.DeployType = "Deployment"
		o.DeployType = "Deployment"
	default:
		data.DeployType = ""
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- kubectl get po -n accuknox-agents")
	}
	defer conn.Close()

	client := opb.NewObservabilityClient(conn)

	if data.PodName != "" {
		sumResp, err := client.Summary(context.Background(), data)
		if err != nil {
			return nil, err
		}

		if o.Output == "" {
			DisplaySummaryOutput(sumResp, data, o.RevDNSLookup)
		}

		sumstr := ""
		if o.Output == "json" {
			arr, _ := json.MarshalIndent(sumResp, "", "    ")
			sumstr = fmt.Sprintf("%s\n", string(arr))
			str = append(str, sumstr)
			return str, nil
		}

	} else if data.DeployName != "" && data.DeployType == "" {
		sumResp, err := client.SummaryPerDeploy(context.Background(), data)
		if err != nil {
			return nil, err
		}

		if o.Output == "" {
			DisplaySummaryOutput(sumResp, data, o.RevDNSLookup)
		}

		sumstr := ""
		if o.Output == "json" {
			arr, _ := json.MarshalIndent(sumResp, "", "    ")
			sumstr = fmt.Sprintf("%s\n", string(arr))
			str = append(str, sumstr)
			return str, nil
		}

	} else {
		deployResp, err := client.GetDeployNames(context.Background(), data)
		if err != nil {
			return nil, err
		}

		for key, value := range deployResp.DeployData {
			if key == "" {
				continue
			}
			if o.DeployType != "" && value != o.DeployType {
				continue
			}
			data.DeployName = key
			data.DeployType = value
			sumResp, err := client.SummaryPerDeploy(context.Background(), data)
			if err != nil {
				return nil, err
			}
			if o.Output == "" {
				DisplaySummaryOutput(sumResp, data, o.RevDNSLookup)
			}

			sumstr := ""
			if o.Output == "json" {
				arr, _ := json.MarshalIndent(sumResp, "", "    ")
				sumstr = fmt.Sprintf("%s\n", string(arr))
				str = append(str, sumstr)
			}
		}

	}
	return str, nil
}

// Summary - printing the summary output
func Summary(c *k8s.Client, o Options) error {

	summary, err := GetSummary(c, o)
	if err != nil {
		return err
	}
	for _, sum := range summary {
		if o.Output == "json" {
			fmt.Printf("%s", sum)
		}
	}
	return nil
}
