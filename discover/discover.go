// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package discover fetches policies from discovery engine
package discover

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/clarketm/json"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"

	wpb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/worker"
	"github.com/accuknox/auto-policy-discovery/src/types"
	"google.golang.org/grpc"
)

// Options Structure
type Options struct {
	GRPC        string
	Format      string
	Policy      string
	Namespace   string
	Clustername string
	Labels      string
	Fromsource  string
}

// ConvertPolicy converts the knoxautopolicies to KubeArmor and Cilium policies
func ConvertPolicy(o Options) error {
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

	data := &wpb.WorkerRequest{
		Policytype:  o.Policy,
		Namespace:   o.Namespace,
		Clustername: o.Clustername,
		Labels:      o.Labels,
		Fromsource:  o.Fromsource,
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return nil
	}
	defer conn.Close()

	client := wpb.NewWorkerClient(conn)

	var response *wpb.WorkerResponse
	response, err = client.Convert(context.Background(), data)
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor log --grpc <info>\033[0m")
	}

	if o.Policy == "network" {
		policy := types.CiliumNetworkPolicy{}

		ciliumpolicy := []types.CiliumNetworkPolicy{}

		if len(response.Ciliumpolicy) > 0 {
			for _, val := range response.Ciliumpolicy {
				policy = types.CiliumNetworkPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Error().Msg(err.Error())
					return err
				}

				ciliumpolicy = append(ciliumpolicy, policy)

				str := ""
				if o.Format == "json" {
					arr, _ := json.MarshalIndent(policy, "", "    ")
					str = fmt.Sprintf("%s\n", string(arr))
					fmt.Printf("%s", str)
				} else if o.Format == "yaml" {
					arr, _ := json.Marshal(policy)
					yamlarr, _ := yaml.JSONToYAML(arr)
					str = fmt.Sprintf("%s", string(yamlarr))
					fmt.Printf("%s---\n", str)
				} else {
					log.Printf("Currently supported formats are json and yaml\n")
					break
				}
			}
		}
	} else if o.Policy == "system" {
		kubearmorpolicy := []types.KubeArmorPolicy{}

		if len(response.Kubearmorpolicy) > 0 {
			for _, val := range response.Kubearmorpolicy {
				policy := types.KubeArmorPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Error().Msg(err.Error())
					return err
				}

				kubearmorpolicy = append(kubearmorpolicy, policy)

				str := ""
				if o.Format == "json" {
					arr, _ := json.MarshalIndent(policy, "", "    ")
					str = fmt.Sprintf("%s\n", string(arr))
					fmt.Printf("%s", str)
				} else if o.Format == "yaml" {
					arr, _ := json.Marshal(policy)
					yamlarr, _ := yaml.JSONToYAML(arr)
					str = fmt.Sprintf("%s", string(yamlarr))
					fmt.Printf("%s---\n", str)
				} else {
					fmt.Printf("Currently supported formats are json and yaml\n")
					break
				}
			}
		}
	}

	return err
}

// Policy discovers Cilium or KubeArmor policies
func Policy(o Options) error {
	if o.Policy == "cilium" {
		o.Policy = "network"
	} else if o.Policy == "kubearmor" {
		o.Policy = "system"
	} else {
		log.Error().Msgf("Policy type not recognized.\nCurrently supported policies are cilium and kubearmor\n")
	}

	if err := ConvertPolicy(o); err != nil {
		return err
	}
	return nil
}
