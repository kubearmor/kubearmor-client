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

	"github.com/accuknox/accuknox-cli/k8s"
	"github.com/accuknox/accuknox-cli/utils"
	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"

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
			DisplaySummaryOutput(sumResp, o.RevDNSLookup, o.Type)
		}

		sumstr := ""
		if o.Output == "json" {
			arr, _ := json.MarshalIndent(sumResp, "", "    ")
			sumstr = fmt.Sprintf("%s\n", string(arr))
			str = append(str, sumstr)
			return str, nil
		}

	} else {
		//Fetch Summary Logs
		podNameResp, err := client.GetPodNames(context.Background(), data)
		if err != nil {
			return nil, err
		}

		for _, podname := range podNameResp.PodName {
			if podname == "" {
				continue
			}
			data.PodName = podname
			sumResp, err := client.Summary(context.Background(), data)
			if err != nil {
				return nil, err
			}
			if o.Output == "" {
				DisplaySummaryOutput(sumResp, o.RevDNSLookup, o.Type)
			}

			sumstr := ""
			if o.Output == "json" {
				arr, _ := json.MarshalIndent(sumResp, "", "    ")
				sumstr = fmt.Sprintf("%s\n", string(arr))
				str = append(str, sumstr)
			}
		}
		if o.Output == "json" {
			return str, nil
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
