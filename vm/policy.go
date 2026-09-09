// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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

func isManagementUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "connection closed") {
		return true
	}
	if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
		return true
	}
	return false
}

func doPolicyRPC(client pb.PolicyServiceClient, kind string, data []byte) (*pb.Response, error) {
	req := pb.Policy{Policy: data}
	switch kind {
	case KubeArmorPolicy:
		return client.ContainerPolicy(context.Background(), &req)
	case KubeArmorHostPolicy:
		return client.HostPolicy(context.Background(), &req)
	case KubeArmorNetworkPolicy:
		return client.NetworkPolicy(context.Background(), &req)
	default:
		return nil, fmt.Errorf("unknown policy kind %q", kind)
	}
}

func sendPolicyOverGRPC(o PolicyOptions, policyEventData []byte, kind string) error {
	// Primary path: management plane on :32765 with mTLS.
	gRPC := ManagementGRPCAddress(o.GRPC)
	mgmtDir := ManagementTLSCertPath(o.ManagementTLSCertPath)

	conn, err := NewManagementGRPCClient(gRPC, mgmtDir)
	if err != nil {
		if isManagementUnavailable(err) {
			return sendPolicyOverGRPCFallback(gRPC, policyEventData, kind)
		}
		return err
	}
	defer conn.Close()

	resp, err := doPolicyRPC(pb.NewPolicyServiceClient(conn), kind, policyEventData)
	if err != nil {
		if isManagementUnavailable(err) {
			return sendPolicyOverGRPCFallback(gRPC, policyEventData, kind)
		}
		return fmt.Errorf("failed to send policy")
	}

	fmt.Printf("Policy %s \n", resp.Status)
	return nil
}

// sendPolicyOverGRPCFallback is the pre-split-plane path: PolicyService on
// :32767 (log plane port) over an insecure channel. Used when the agent is
// an older KubeArmor that does not serve the management plane on :32765.
func sendPolicyOverGRPCFallback(primaryAddr string, policyEventData []byte, kind string) error {
	legacyAddr := "localhost:32767"
	if host, _, err := net.SplitHostPort(primaryAddr); err == nil && host != "" {
		legacyAddr = net.JoinHostPort(host, "32767")
	} else if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok && val != "" {
		if host, _, err := net.SplitHostPort(val); err == nil && host != "" {
			legacyAddr = net.JoinHostPort(host, "32767")
		}
	}
	conn, err := grpc.NewClient(legacyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := doPolicyRPC(pb.NewPolicyServiceClient(conn), kind, policyEventData)
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
