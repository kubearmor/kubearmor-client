// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"

	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

const (
	// KubeArmorPolicy is the Kind used for KubeArmor container policies
	KubeArmorPolicy = "KubeArmorPolicy"
	// KubeArmorHostPolicy is the Kind used for KubeArmor host policies
	KubeArmorHostPolicy = "KubeArmorHostPolicy"
	// CiliumNetworkPolicy is the Kind used for Cilium network policies
	CiliumNetworkPolicy = "CiliumNetworkPolicy"
	// CiliumClusterwideNetworkPolicy is the Kind used for Cilium network policies
	CiliumClusterwideNetworkPolicy = "CiliumClusterwideNetworkPolicy"
)

// PolicyOptions are optional configuration for kArmor vm policy
type PolicyOptions struct {
	GRPC string
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
		if err != nil || resp.Status != 1 {
			return fmt.Errorf("failed to send policy")
		}
	} else {
		resp, err := client.ContainerPolicy(context.Background(), &req)
		if err != nil || resp.Status != 1 {
			return fmt.Errorf("failed to send policy")
		}
	}
	fmt.Println("Success")
	return nil
}

func sendPolicyOverHTTP(address string, kind string, policyEventData []byte) error {

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	var url string
	if kind == KubeArmorHostPolicy {
		url = address + "/policy/kubearmor"
	} else {
		url = address + "/policy/cilium"
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(policyEventData))
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return fmt.Errorf("failed to send policy")
	}

	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send policy")
	}
	defer resp.Body.Close()

	fmt.Println("Success")
	return nil
}

//PolicyHandling Function recives path to YAML file with the type of event and emits an Host Policy Event to KubeArmor gRPC/HTTP Server
func PolicyHandling(t string, path string, o PolicyOptions, httpAddress string, isKvmsEnv bool) error {
	var k struct {
		Kind string `json:"kind"`
	}

	policyFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}

	policies := strings.Split(string(policyFile), "---")

	for _, policy := range policies {

		if matched, _ := regexp.MatchString("^\\s*$", policy); matched {
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
		var networkPolicy v2.CiliumNetworkPolicy
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

		} else if k.Kind == CiliumNetworkPolicy || k.Kind == CiliumClusterwideNetworkPolicy {
			err = json.Unmarshal(js, &networkPolicy)
			if err != nil {
				return err
			}

			if networkPolicy.Spec == nil {
				continue
			}

			policyEvent = NetworkPolicyRequest{
				Type:   t,
				Object: networkPolicy,
			}

		}

		policyEventData, err := json.Marshal(policyEvent)
		if err != nil {
			return err
		}

		if isKvmsEnv {
			// Non-K8s control plane with kvmservice, hence send policy over HTTP
			if err = sendPolicyOverHTTP(httpAddress, k.Kind, policyEventData); err != nil {
				return err
			}
		} else {
			// Systemd mode, hence send policy over gRPC
			if err = sendPolicyOverGRPC(o, policyEventData, k.Kind); err != nil {
				return err

			}
		}
	}

	return nil
}
