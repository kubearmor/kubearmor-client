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
var DefaultReqType = "process,file,network,syscall"
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

// Summary : Get summary on pods
func Summary(c *k8s.Client, o Options) error {
	gRPC := ""

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
			gRPC = val
		} else {
			pf, err := utils.InitiatePortForward(c, port, port, matchLabels)
			if err != nil {
				return err
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
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n[0m")
	}
	defer conn.Close()

	client := opb.NewObservabilityClient(conn)

	if data.PodName != "" {
		sumResp, err := client.Summary(context.Background(), &opb.Request{
			PodName:   data.PodName,
			Type:      o.Type,
			Aggregate: o.Aggregation,
		})
		if err != nil {
			return err
		}
		DisplaySummaryOutput(sumResp, o.RevDNSLookup, o.Type)

	} else {
		//Fetch Summary Logs
		podNameResp, err := client.GetPodNames(context.Background(), data)
		if err != nil {
			return err
		}

		for _, podname := range podNameResp.PodName {
			if podname == "" {
				continue
			}
			sumResp, err := client.Summary(context.Background(), &opb.Request{
				PodName:   podname,
				Type:      o.Type,
				Aggregate: o.Aggregation,
			})
			if err != nil {
				return err
			}
			if o.Output == "" {
				DisplaySummaryOutput(sumResp, o.RevDNSLookup, o.Type)
			}

			str := ""
			if o.Output == "json" {
				arr, _ := json.MarshalIndent(sumResp, "", "    ")
				str = fmt.Sprintf("%s\n", string(arr))
				fmt.Printf("%s", str)
			}
		}
	}
	return nil
}
