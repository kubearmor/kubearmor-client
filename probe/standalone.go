// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package probe

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/internal/transport"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Explicit standalone probes use the management RPC without assuming the agent
// shares this machine's systemd service or local host metadata.
func printStandaloneProbe(o Options, host *KubeArmorProbeData) error {
	policyData, err := getPolicyData(o)
	if err != nil {
		return err
	}
	armoredContainers, containerMap := getArmoredContainerData(policyData.ContainerList, policyData.ContainerMap)
	if o.Output == "json" {
		data := map[string]interface{}{
			"HostPolicies":      policyData.HostMap,
			"ArmoredContainers": containerMap,
		}
		if host != nil {
			data["Host"] = host
		}
		out, err := json.Marshal(map[string]interface{}{"Probe Data": data})
		if err != nil {
			return err
		}
		o.printLn(string(out))
		return nil
	}

	if host != nil {
		o.printToOutput(green, "\nFound KubeArmor running in Systemd mode \n\n")
		o.printToOutput(boldWhite, "Host : \n")
		o.printKubeArmorProbeOutput(*host)
	} else {
		o.printToOutput(green, "\nFound KubeArmor at "+transport.ManagementGRPCAddress(o.GRPC)+"\n\n")
	}
	if len(policyData.HostMap) > 0 {
		o.printHostPolicy(getHostPolicyData(policyData))
	}
	o.printContainersSystemd(armoredContainers)
	return nil
}

func getPolicyData(o Options) (*pb.ProbeResponse, error) {
	address := transport.ManagementGRPCAddress(o.GRPC)
	connection, err := transport.NewManagementGRPCClient(address, transport.ManagementTLSCertPath(o.ManagementTLSCertPath))
	if err != nil {
		return nil, fmt.Errorf("connect to management endpoint %s: %w", address, err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), transport.RPCTimeout)
	defer cancel()
	response, err := pb.NewProbeServiceClient(connection).GetProbeData(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("get policies from management endpoint %s: %w", address, err)
	}
	return response, nil
}

func getHostPolicyData(policyData *pb.ProbeResponse) [][]string {
	var data [][]string
	for host, policies := range policyData.HostMap {
		for _, policy := range policies.PolicyList {
			data = append(data, []string{host, policy})
		}
	}
	return data
}

func getArmoredContainerData(containerList []string, containerMap map[string]*pb.ContainerData) ([][]string, map[string][]string) {
	var data [][]string
	for _, containerName := range containerList {
		if container, ok := containerMap[containerName]; ok {
			if container.PolicyEnabled == 1 {
				for _, policyName := range container.PolicyList {
					data = append(data, []string{containerName, policyName})
				}
			}
		} else {
			data = append(data, []string{containerName, ""})
		}
	}
	policies := make(map[string][]string)
	for _, row := range data {
		policies[row[0]] = append(policies[row[0]], row[1])
	}
	return data, policies
}
