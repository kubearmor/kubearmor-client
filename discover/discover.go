// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/accuknox/auto-policy-discovery/src/libs"
	wpb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/worker"
	"github.com/accuknox/auto-policy-discovery/src/types"
	"google.golang.org/grpc"
)

// Options Structure
type Options struct {
	Policy string
	GRPC   string
}

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
	fmt.Println("gRPC server: " + gRPC)

	data := &wpb.WorkerRequest{
		Policytype: o.Policy,
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
					log.Fatal(err)
				}

				ciliumpolicy = append(ciliumpolicy, policy)

				str := ""
				arr, _ := json.MarshalIndent(policy, "", "    ")

				str = fmt.Sprintf("%s\n", string(arr))

				log.Printf("%s", str)

				//write discovered policies to file
				libs.WriteCiliumPolicyToYamlFile("", ciliumpolicy)

			}
		}
	} else if o.Policy == "system" {
		policy := types.KubeArmorPolicy{}

		kubearmorpolicy := []types.KubeArmorPolicy{}

		if len(response.Kubearmorpolicy) > 0 {
			for _, val := range response.Kubearmorpolicy {
				policy = types.KubeArmorPolicy{}

				err = json.Unmarshal(val.Data, &policy)
				if err != nil {
					log.Fatal(err)
				}

				kubearmorpolicy = append(kubearmorpolicy, policy)

				str := ""
				arr, _ := json.MarshalIndent(policy, "", "    ")

				str = fmt.Sprintf("%s\n", string(arr))

				log.Printf("%s", str)
				libs.WriteKubeArmorPolicyToYamlFile("kubearmor_policies", kubearmorpolicy)

			}
		}
	}

	return err
}

func DiscoverPolicy(o Options) error {

	if o.Policy == "cilium" {
		o.Policy = "network"
	} else if o.Policy == "kubearmor" {
		o.Policy = "system"
	} else {
		log.Println("Policy type not recognized")
	}

	ConvertPolicy(o)

	return nil
}
