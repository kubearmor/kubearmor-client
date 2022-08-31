// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package summary shows observability data from discovery engine
package summary

import (
	"context"
	"os"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Options Structure
type Options struct {
	GRPC          string
	Labels        string
	Namespace     string
	PodName       string
	ClusterName   string
	ContainerName string
	Type          string
	RevDNSLookup  bool
}

// Summary : Get summary on pods
func Summary(o Options) error {
	gRPC := ""

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
			gRPC = val
		} else {
			gRPC = "localhost:9089"
		}
	}

	data := &opb.Request{
		Label:         o.Labels,
		NameSpace:     o.Namespace,
		PodName:       o.PodName,
		ClusterName:   o.ClusterName,
		ContainerName: o.ContainerName,
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := opb.NewObservabilityClient(conn)

	if data.PodName != "" {
		sumResp, err := client.Summary(context.Background(), &opb.Request{
			PodName: data.PodName,
			Type:    "system",
		})
		if err != nil {
			return err
		}
		DisplaySummaryOutput(sumResp, o.RevDNSLookup)

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
				PodName: podname,
				Type:    "system",
			})
			if err != nil {
				return err
			}
			DisplaySummaryOutput(sumResp, o.RevDNSLookup)

		}
	}
	return nil
}
