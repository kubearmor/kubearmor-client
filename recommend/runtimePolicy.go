// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var saPath []string

func init() {
	saPath = []string{
		"/var/run/secrets/kubernetes.io/serviceaccount/",
		"/run/secrets/kubernetes.io/serviceaccount/",
	}
}

// createRuntimePolicy function generates runtime policy for service account
func createRuntimePolicy(img *ImageInfo) error {
	var labels string
	for key, value := range img.Labels {
		labels = strings.TrimPrefix(fmt.Sprintf("%s,%s=%s", labels, key, value), ",")
	}
	gRPC := ""
	if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
		gRPC = val
	} else {
		gRPC = "localhost:9089"
	}
	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n[0m")
	}
	defer conn.Close()
	client := opb.NewObservabilityClient(conn)
	podData, err := client.GetPodNames(context.Background(), &opb.Request{
		Label:     labels,
		NameSpace: img.Namespace,
		Aggregate: true,
	})
	if err != nil {
		return err
	}
	var resp *opb.Response
	var sumResp []*opb.Response
	for _, pod := range podData.PodName {
		resp, err = client.Summary(context.Background(), &opb.Request{
			PodName:   pod,
			Label:     labels,
			NameSpace: img.Namespace,
			Type:      "file",
			Aggregate: true,
		})
		if err != nil {
			return err
		}
		sumResp = append(sumResp, resp)
	}

	ms := checkProcessFileData(sumResp, img.Distro)
	if ms != nil {
		img.writePolicyFile(*ms)
	}
	return nil
}

func checkProcessFileData(sumResp []*opb.Response, distro string) *MatchSpec {
	var filePaths pol.FileType
	ref := Ref{
		Name: "MITRE Unsecured Credentials: Container API",
		URL:  []string{"https://attack.mitre.org/techniques/T1552/007/"},
	}
	fromSourceArr := []pol.MatchSourceType{}
	ms := MatchSpec{
		Name: "allow-serviceaccount-runtime",
		Description: Description{
			Refs:     []Ref{ref},
			Tldr:     "Kubernetes serviceaccount folder access should be limited",
			Detailed: "Adversaries may gather credentials via APIs within a containers environment. APIs in these environments, such as the Docker API and Kubernetes APIs, allow a user to remotely manage their container resources and cluster components. An adversary may access the Docker API to collect logs that contain credentials to cloud, container, and various other resources in the environment. An adversary with sufficient permissions, such as via a pod's service account, may also use the Kubernetes API to retrieve credentials from the Kubernetes API server. These credentials may include those needed for Docker API authentication or secrets from Kubernetes cluster components.",
		},
	}
	for _, eachResp := range sumResp {
		for _, fileData := range eachResp.FileData {
			if strings.HasPrefix(fileData.Destination, saPath[0]) || strings.HasPrefix(fileData.Destination, saPath[1]) {
				fromSourceArr = append(fromSourceArr, pol.MatchSourceType{
					Path: pol.MatchPathType(fileData.Source),
				})
			}
		}
	}
	filePaths.MatchDirectories = append(filePaths.MatchDirectories, pol.FileDirectoryType{
		Directory:  pol.MatchDirectoryType(saPath[0]),
		FromSource: fromSourceArr,
		Recursive:  true,
	})
	filePaths.MatchDirectories = append(filePaths.MatchDirectories, pol.FileDirectoryType{
		Directory:  pol.MatchDirectoryType(saPath[1]),
		FromSource: fromSourceArr,
		Recursive:  true,
	})
	ms.Spec = pol.KubeArmorPolicySpec{
		Action:   "Allow",
		Message:  "serviceaccount access detected",
		Tags:     []string{"KUBERNETES", "SERVICE ACCOUNT", "RUNTIME POLICY"},
		Severity: 1,
		File:     filePaths,
	}
	if len(fromSourceArr) < 1 {
		ms.Spec.Action = "Block"
		ms.Name = "block-serviceaccount-runtime"
		ms.Spec.Message = "serviceaccount access blocked"
	}
	return &ms
}
