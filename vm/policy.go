// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/protobuf/types/known/emptypb"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
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
	GRPC   string
	Output string
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			kg.Warnf("Error closing http stream %s\n", err)
		}
	}()

	fmt.Println("Success")
	return nil
}

// PolicyHandling Function recives path to YAML file with the type of event and emits an Host Policy Event to KubeArmor gRPC/HTTP Server
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

func GetPolicy(o PolicyOptions) (*pb.ProbeResponse, error) {
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

func (o *PolicyOptions) PrintContainersSystemd(podData [][]string) {
	o.printToOutput(color.New(color.FgWhite, color.Bold), "Armored Up Containers : \n")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"CONTAINER NAME", "POLICY"})
	for _, v := range podData {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()
}

func (o *PolicyOptions) printToOutput(c *color.Color, s string) {
	red := color.New(color.FgRed)
	if o.Output == "no-color" || c == nil {
		_, err := fmt.Fprint(os.Stdout, s)
		if err != nil {
			_, printErr := red.Printf(" error while printing to os.Stdout %s ", err.Error())
			if printErr != nil {
				fmt.Printf("printing error %s", printErr.Error())
			}
		}
	} else {
		_, err := c.Fprintf(os.Stdout, s)
		if err != nil {
			_, printErr := red.Printf(" error while printing to os.Stdout %s ", err.Error())
			if printErr != nil {
				fmt.Printf("printing error %s", printErr.Error())
			}
		}
	}
}

func PrettyPrintPolicy(policy pb.Policy) error {
	var policyJSON tp.SecurityPolicy
	err := json.Unmarshal(policy.Policy, &policyJSON)
	if err != nil {
		return err
	}
	printPolicy(parsePolicy(policyJSON))
	return nil
}

// parsePolicy parses the policy data into a 2D slice
// which can be printed in an indent format
func parsePolicy(policy tp.SecurityPolicy) [][]string {
	// Metadata
	policyData := [][]string{
		{"Name: ", policy.Metadata["policyName"]},
	}

	// Spec
	policyData = append(policyData, [][]string{
		{"Spec:"},
		{"", "Selector:"},
		{"", "", "MatchLabels:"},
	}...)

	// MatchLabels
	for k, v := range policy.Spec.Selector.MatchLabels {
		policyData = append(policyData, []string{
			"", "", "", k + ": " + v},
		)
	}

	// Severity, Tags and Message
	policyData = append(policyData, []string{
		"", "Severity: " + fmt.Sprint(policy.Spec.Severity),
	})
	if policy.Spec.Message != "" {
		policyData = append(policyData, []string{
			"", "Message: " + policy.Spec.Message},
		)
	}
	if len(policy.Spec.Tags) > 0 {
		policyData = append(policyData, []string{
			"", "Tags: " + strings.Join(policy.Spec.Tags, ", "),
		})
	}

	// Process
	if len(policy.Spec.Process.MatchDirectories)|len(policy.Spec.Process.MatchPaths)|len(policy.Spec.Process.MatchDirectories) > 0 {
		policyData = append(policyData, []string{
			"", "Process:",
		})
		// MatchPaths
		if len(policy.Spec.Process.MatchPaths) > 0 {
			policyData = append(policyData, []string{
				"", "", "MatchPaths:",
			})
			for _, v := range policy.Spec.Process.MatchPaths {
				policyData = append(policyData, []string{
					"", "", "- ", "Path: ", v.Path,
				})
				policyData = append(policyData, []string{
					"", "", "", "OwnerOnly: ", fmt.Sprint(v.OwnerOnly),
				})
				if len(v.FromSource) > 0 {
					policyData = append(policyData, []string{
						"", "", "", "FromSource: ",
					})
					for _, v1 := range v.FromSource {
						policyData = append(policyData, []string{
							"", "", "", "", v1.Path,
						})
					}
				}
			}
		}

		// MatchDirectories
		if len(policy.Spec.Process.MatchDirectories) > 0 {
			policyData = append(policyData, []string{
				"", "", "MatchDirectory:",
			})
			for _, v := range policy.Spec.Process.MatchDirectories {
				policyData = append(policyData, []string{
					"", "", "- ", "Directory: ", v.Directory,
				})
				policyData = append(policyData, []string{
					"", "", "", "Recursive: ", fmt.Sprint(v.Recursive),
					"", "", "", "OwnerOnly: ", fmt.Sprint(v.OwnerOnly),
				})
				if len(v.FromSource) > 0 {
					policyData = append(policyData, []string{
						"", "", "", "FromSource: ",
					})
					for _, v1 := range v.FromSource {
						policyData = append(policyData, []string{
							"", "", "", "", v1.Path,
						})
					}
				}
			}
		}

		// MatchPatterns
		if len(policy.Spec.Process.MatchPatterns) > 0 {
			policyData = append(policyData, []string{
				"", "", "MatchPatterns:",
			})
			for _, v := range policy.Spec.Process.MatchPatterns {
				policyData = append(policyData, []string{
					"", "", "- ", "Pattern: ", v.Pattern,
				})
				policyData = append(policyData, []string{
					"", "", "", "OwnerOnly: ", fmt.Sprint(v.OwnerOnly),
				})
			}
		}
	}

	return policyData
}

// PrintPolicy prints the policy data in an indent format
func printPolicy(data [][]string) {
	for _, v := range data {
		for _, v1 := range v {
			if v1 == "" {
				fmt.Printf("  ")
				continue
			}
			fmt.Printf("%s", v1)
		}
		fmt.Println()
	}
}
