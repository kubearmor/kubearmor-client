// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package insight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	ipb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/insight"
	"google.golang.org/grpc"
)

// Options Structure
type Options struct {
	GRPC          string
	Labels        string
	Containername string
	Clustername   string
	Fromsource    string
	Namespace     string
	Source        string
	Type          string
	Rule          string
}

// Get insights on observability data
func StartInsight(o Options) error {
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

	fmt.Println("gRPC server: " + gRPC)

	data := &ipb.Request{
		Request:       "observe",
		Source:        o.Source,
		Labels:        o.Labels,
		ContainerName: o.Containername,
		ClusterName:   o.Clustername,
		FromSource:    o.Fromsource,
		Namespace:     o.Namespace,
		Type:          o.Type,
		Rule:          o.Rule,
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ipb.NewInsightClient(conn)

	// var response opb.Response
	response, err := client.GetInsightData(context.Background(), data)
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor log --grpc <info>\033[0m")
	}

	str := ""
	arr, _ := json.MarshalIndent(response, "", "    ")
	str = fmt.Sprintf("%s\n", string(arr))

	log.Printf("%s \n", str)

	return nil
}
