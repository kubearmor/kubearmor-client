// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package discover fetches policies from discovery engine
package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/accuknox/accuknox-cli/k8s"
	"github.com/accuknox/accuknox-cli/utils"
	"github.com/clarketm/json"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"

	nv1 "k8s.io/api/networking/v1"

	wpb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/worker"
	"github.com/accuknox/auto-policy-discovery/src/types"
	"google.golang.org/grpc"
)

// Options Structure
type Options struct {
	GRPC           string
	Format         string
	Policy         string
	Namespace      string
	Clustername    string
	Labels         string
	Fromsource     string
	IncludeNetwork bool
}

var matchLabels = map[string]string{"app": "discovery-engine"}
var port int64 = 9089

// ConvertPolicy converts the knoxautopolicies to KubeArmor and Cilium policies
func ConvertPolicy(c *k8s.Client, o Options) ([]string, error) {
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

	data := &wpb.WorkerRequest{
		Policytype:     o.Policy,
		Namespace:      o.Namespace,
		Clustername:    o.Clustername,
		Labels:         o.Labels,
		Fromsource:     o.Fromsource,
		Includenetwork: o.IncludeNetwork,
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := wpb.NewWorkerClient(conn)

	var response *wpb.WorkerResponse
	response, err = client.Convert(context.Background(), data)
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- kubectl get po -n accuknox-agents")
	}

	if o.Policy == "CiliumNetworkPolicy" {

		if len(response.Ciliumpolicy) > 0 {
			for _, val := range response.Ciliumpolicy {
				policy := types.CiliumNetworkPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Error().Msg(err.Error())
					return nil, err
				}

				if o.Format == "json" {
					arr, _ := json.MarshalIndent(policy, "", "    ")
					pstr := fmt.Sprintf("%s\n", string(arr))
					str = append(str, pstr)
				} else if o.Format == "yaml" {
					arr, _ := json.Marshal(policy)
					yamlarr, _ := yaml.JSONToYAML(arr)
					pstr := fmt.Sprintf("%s", string(yamlarr))
					str = append(str, pstr)
				} else {
					log.Printf("Currently supported formats are json and yaml\n")
					break
				}
			}
			return str, err
		}
	} else if o.Policy == "KubearmorSecurityPolicy" {

		if len(response.Kubearmorpolicy) > 0 {
			for _, val := range response.Kubearmorpolicy {
				policy := types.KubeArmorPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Error().Msg(err.Error())
					return nil, err
				}

				if o.Format == "json" {
					arr, _ := json.MarshalIndent(policy, "", "    ")
					pstr := fmt.Sprintf("%s\n", string(arr))
					str = append(str, pstr)
				} else if o.Format == "yaml" {
					arr, _ := json.Marshal(policy)
					yamlarr, _ := yaml.JSONToYAML(arr)
					pstr := fmt.Sprintf("%s", string(yamlarr))
					str = append(str, pstr)
				} else {
					fmt.Printf("Currently supported formats are json and yaml\n")
					break
				}
			}
			return str, err
		}
	} else if o.Policy == "NetworkPolicy" {

		if len(response.K8SNetworkpolicy) > 0 {
			for _, val := range response.K8SNetworkpolicy {
				policy := nv1.NetworkPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Error().Msg(err.Error())
					return nil, err
				}

				if o.Format == "json" {
					arr, _ := json.MarshalIndent(policy, "", "    ")
					pstr := fmt.Sprintf("%s\n", string(arr))
					str = append(str, pstr)
				} else if o.Format == "yaml" {
					arr, _ := json.Marshal(policy)
					yamlarr, _ := yaml.JSONToYAML(arr)
					pstr := fmt.Sprintf("%s", string(yamlarr))
					str = append(str, pstr)
				} else {
					fmt.Printf("Currently supported formats are json and yaml\n")
					break
				}
			}
			return str, err
		}
	}

	return str, err
}

// Policy discovers Cilium or KubeArmor policies
func Policy(c *k8s.Client, o Options) error {
	var str []string
	var err error
	if o.Policy != "CiliumNetworkPolicy" && o.Policy != "NetworkPolicy" && o.Policy != "KubearmorSecurityPolicy" {
		log.Error().Msgf("Policy type not recognized.\nCurrently supported policies are cilium, kubearmor and k8snetpol\n")
	}

	if str, err = ConvertPolicy(c, o); err != nil {
		return err
	}
	for _, policy := range str {
		if o.Format == "yaml" {
			fmt.Printf("%s---\n", policy)
		}
		if o.Format == "json" {
			fmt.Printf("%s", policy)
		}
	}
	return nil
}
