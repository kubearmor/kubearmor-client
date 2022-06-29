// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ipb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/insight"
	"github.com/rs/zerolog/log"
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

type Resp struct {
	Resp []Res `json:"Res"`
}

type Res struct {
	ClusterName     string            `json:"ClusterName"`
	NameSpace       string            `json:"NameSpace"`
	Labels          string            `json:"Labels"`
	SystemResource  []SystemResource  `json:"SystemResource"`
	NetworkResource []NetworkResource `json:"NetworkResource"`
}

type SystemResource struct {
	SysResource []SysResource `json:"SysResource"`
}

type NetworkResource struct {
	NetResource []NetResource `json:"NetResource"`
}

type SysResource struct {
	FromSource      string   `json:"fromSource"`
	FilePaths       []string `json:"filePaths"`
	NetworkProtocol []string `json:"networkProtocol"`
	ProcessPaths    []string `json:"processPaths"`
}

type NetResource struct {
	Egressess  []Egressess  `json:"Egressess"`
	Ingressess []Ingressess `json:"Ingressess"`
}

type Egressess struct {
	MatchLabels MatchLabels   `json:"MatchLabels"`
	ToPorts     []ToPorts     `json:"ToPorts"`
	ToEndtities []ToEndtities `json:"ToEndtities"`
}

type Ingressess struct {
	MatchLabels  MatchLabels    `json:"MatchLabels"`
	ToPorts      []ToPorts      `json:"ToPorts"`
	FromEntities []FromEntities `json:"FromEntities"`
}

type MatchLabels struct {
	Container    string `json:"container"`
	K8snamespace string `json:"k8s:io.kubernetes.pod.namespace"`
}

type ToPorts struct {
	Port     string `json:"Port"`
	Protocol string `json:"Protocol"`
}

type FromEntities struct {
	Host      string `json:"host"`
	Apiserver string `json:"kube-apiserver"`
}

type ToEndtities struct {
	Host      string `json:"host"`
	Apiserver string `json:"kube-apiserver"`
}

// Get summary on observability data
func StartSummary(o Options) error {
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

	var resp Resp
	arr, _ := json.Marshal(response)
	err = json.Unmarshal([]byte(arr), &resp)
	if err != nil {
		return err
	}

	fmt.Println("Deployment Details:")
	for _, res := range resp.Resp {

		if o.Source == "system" || o.Source == "all" {
			for _, sys := range res.SystemResource {
				fmt.Println("\n---")
				if res.Labels != "" {
					fmt.Println("\nLabels: " + res.Labels)
				}

				fmt.Println("Namespace: " + res.NameSpace)
				for _, sysRes := range sys.SysResource {
					if sysRes.FromSource != "" {
						fmt.Println("\nProcess: " + sysRes.FromSource)
					}
					for _, procpath := range sysRes.ProcessPaths {
						if sysRes.FromSource == "" {
							fmt.Println("\nProcess: " + procpath)
						} else {
							fmt.Println("Child process: " + procpath)
						}
					}

					for _, filepath := range sysRes.FilePaths {
						fmt.Println("File system access: " + filepath)
					}

					for _, netpath := range sysRes.NetworkProtocol {
						fmt.Println("Protocol: " + netpath)
					}

				}
			}
		} else if o.Source == "network" || o.Source == "all" {

			// for _, net := range res.NetworkResource {
			// 	fmt.Println(len(net.NetResource))
			// 	for _, netRes := range net.NetResource {
			// 		fmt.Println("Parent process: " + Res.FromSource)
			// 		for _, filepath := range sysRes.FilePaths {
			// 			fmt.Println("Child processes: " + filepath)
			// 		}

			// 		for _, netpath := range sysRes.NetworkProtocol {
			// 			fmt.Println("Child processes: " + netpath)
			// 		}

			// 		for _, procpath := range sysRes.ProcessPaths {
			// 			fmt.Println("Child processes: " + procpath)
			// 		}

			// 	}
			// }
		} else {
			log.Error().Msgf("Source type not recognized. Source type available are system and network\n")
		}
	}

	return nil
}
