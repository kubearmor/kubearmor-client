// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/utils"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

const (
	// KubeArmorPolicy is the Kind used for KubeArmor container policies
	KubeArmorPolicy = "KubeArmorPolicy"
	// KubeArmorHostPolicy is the Kind used for KubeArmor host policies
	KubeArmorHostPolicy = "KubeArmorHostPolicy"
)

var (
	BoldWhite = color.New(color.FgWhite, color.Bold)
)

// PolicyOptions are optional configuration for kArmor vm policy
type PolicyOptions struct {
	GRPC string
	Type string
}

func sendPolicyOverGRPC(o PolicyOptions, policyEventData []byte, kind string) error {
	gRPC := ""

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
			gRPC = val
		} else {
			gRPC = "localhost:32767"
		}
	}

	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := pb.NewPolicyServiceClient(conn)

	req := pb.Policy{
		Policy: policyEventData,
	}

	if kind == KubeArmorHostPolicy {
		resp, err := client.HostPolicy(context.Background(), &req)
		if err != nil {
			return fmt.Errorf("failed to send policy")
		}
		fmt.Printf("Policy %s \n", resp.Status)
		return nil

	}
	resp, err := client.ContainerPolicy(context.Background(), &req)
	if err != nil {
		return fmt.Errorf("failed to send policy")
	}
	fmt.Printf("Policy %s \n", resp.Status)
	return nil

}

// PolicyHandling Function recives path to YAML file with the type of event and emits an Host Policy Event to KubeArmor gRPC/HTTP Server
func PolicyHandling(t string, path string, o PolicyOptions) error {
	var k struct {
		Kind string `json:"kind"`
	}

	policyFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}

	policies := strings.Split(string(policyFile), "---")

	for _, policy := range policies {
		re := regexp.MustCompile(`^\\s*$`)
		if matched := re.MatchString(policy); matched {
			continue
		}

		js, err := yaml.YAMLToJSON([]byte(policy))
		if err != nil {
			return err
		}

		err = json.Unmarshal(js, &k)
		if err != nil {
			return err
		}

		var containerPolicy tp.K8sKubeArmorPolicy
		var hostPolicy tp.K8sKubeArmorHostPolicy
		var policyEvent interface{}

		if k.Kind == KubeArmorHostPolicy {
			err = json.Unmarshal(js, &hostPolicy)
			if err != nil {
				return err
			}

			policyEvent = tp.K8sKubeArmorHostPolicyEvent{
				Type:   t,
				Object: hostPolicy,
			}

		} else if k.Kind == KubeArmorPolicy {
			err = json.Unmarshal(js, &containerPolicy)
			if err != nil {
				return err
			}

			policyEvent = tp.K8sKubeArmorPolicyEvent{
				Type:   t,
				Object: containerPolicy,
			}

		}
		policyEventData, err := json.Marshal(policyEvent)
		if err != nil {
			return err
		}

		// Systemd mode, hence send policy over gRPC
		if err = sendPolicyOverGRPC(o, policyEventData, k.Kind); err != nil {
			return err

		}

	}

	return nil
}

func (o *PolicyOptions) getPolicyData() (*pb.ProbeResponse, error) {
	gRPC := ""
	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
			gRPC = val
		} else {
			gRPC = "localhost:32767"
		}
	}
	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := pb.NewProbeServiceClient(conn)

	resp, err := client.GetProbeData(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (o *PolicyOptions) HandleGet(args []string) error {
	policyData, err := o.getPolicyData()
	if err != nil {
		return err
	}
	switch o.Type {
	case "ksp", "Container", "container":
		if len(args) == 0 {
			armoredContainer, _ := utils.GetArmoredContainerData(policyData.ContainerList, policyData.ContainerMap)
			o.printContainerTable(armoredContainer)
			return nil
		}
		targetPolicy := args[0]
		for _, container := range policyData.GetContainerMap() {
			for i, policy := range container.GetPolicyList() {
				if policy == targetPolicy { // access the policyDataList using the index from the policyList
					return prettyPrintPolicy(*container.GetPolicyDataList()[i])
				}
			}
		}
		return errors.New("Policy " + targetPolicy + " not found")
	case "hsp", "Host", "host":
		if len(args) == 0 {
			hostPolicyData := utils.GetHostPolicyData(policyData)
			if len(hostPolicyData) == 0 {
				return errors.New("no host policies found")
			}
			o.printHostTable(hostPolicyData)
			return nil
		}
		targetPolicy := args[0]
		for _, host := range policyData.HostMap {
			for i, policy := range host.GetPolicyList() {
				if policy == targetPolicy {
					return prettyPrintPolicy(*host.GetPolicyDataList()[i])
				}
			}
		}
		return errors.New("Policy " + targetPolicy + " not found")
	default:
		return errors.New("invalid type: " + o.Type)
	}
	return nil
}

func (o *PolicyOptions) printContainerTable(podData [][]string) {
	BoldWhite.Println("Armored Up Containers:")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"CONTAINER NAME", "POLICY"})
	for _, v := range podData {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()
}

func (o *PolicyOptions) printHostTable(hostPolicy [][]string) {
	BoldWhite.Println("Host Policy:")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"HOST NAME ", "POLICY"})
	for _, v := range hostPolicy {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()
}

func prettyPrintPolicy(policy pb.Policy) error {
	var policyJSON tp.SecurityPolicy
	err := json.Unmarshal(policy.Policy, &policyJSON)
	if err != nil {
		return err
	}
	yamlPolicy, err := yaml.Marshal(policyJSON)
	if err != nil {
		return err
	}
	fmt.Println(strings.TrimRight(string(yamlPolicy), "\n"))
	return nil
}
