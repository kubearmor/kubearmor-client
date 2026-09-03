// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"

	"sigs.k8s.io/yaml"
)

const (
	// KubeArmorPolicy is the Kind used for KubeArmor container policies
	KubeArmorPolicy = "KubeArmorPolicy"
	// KubeArmorHostPolicy is the Kind used for KubeArmor host policies
	KubeArmorHostPolicy = "KubeArmorHostPolicy"
	// KubeArmorNetworkPolicy is the Kind used for KubeArmor network policies
	KubeArmorNetworkPolicy = "KubeArmorNetworkPolicy"
)

// PolicyOptions are optional configuration for kArmor vm policy
type PolicyOptions struct {
	GRPC string
	// ManagementTLSCertPath is the management trust-plane directory
	// (management/ca.crt + management/client.crt/client.key). Empty means
	// resolve via KUBEARMOR_MANAGEMENT_TLS_CERT_PATH or the default.
	ManagementTLSCertPath string
}

func sendPolicyOverGRPC(o PolicyOptions, policyEventData []byte, kind string) error {
	var (
		resp *pb.Response
		err  error
	)

	// PolicyService lives on the management plane; always use the
	// management CA + client identity, never the log-plane credentials
	// and never an insecure channel.
	gRPC := ManagementGRPCAddress(o.GRPC)
	mgmtDir := ManagementTLSCertPath(o.ManagementTLSCertPath)

	conn, err := NewManagementGRPCClient(gRPC, mgmtDir)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewPolicyServiceClient(conn)

	req := pb.Policy{
		Policy: policyEventData,
	}

	switch kind {
	case KubeArmorPolicy:
		resp, err = client.ContainerPolicy(context.Background(), &req)
	case KubeArmorHostPolicy:
		resp, err = client.HostPolicy(context.Background(), &req)
	case KubeArmorNetworkPolicy:
		resp, err = client.NetworkPolicy(context.Background(), &req)
	}

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

		var (
			containerPolicy tp.K8sKubeArmorPolicy
			hostPolicy      tp.K8sKubeArmorHostPolicy
			networkPolicy   tp.K8sKubeArmorNetworkPolicy
			policyEvent     any
		)

		switch k.Kind {
		case KubeArmorHostPolicy:
			err = json.Unmarshal(js, &hostPolicy)
			if err != nil {
				return err
			}

			policyEvent = tp.K8sKubeArmorHostPolicyEvent{
				Type:   t,
				Object: hostPolicy,
			}

		case KubeArmorPolicy:
			err = json.Unmarshal(js, &containerPolicy)
			if err != nil {
				return err
			}

			policyEvent = tp.K8sKubeArmorPolicyEvent{
				Type:   t,
				Object: containerPolicy,
			}

		case KubeArmorNetworkPolicy:
			err = json.Unmarshal(js, &networkPolicy)
			if err != nil {
				return err
			}

			policyEvent = tp.K8sKubeArmorNetworkPolicyEvent{
				Type:   t,
				Object: networkPolicy,
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
