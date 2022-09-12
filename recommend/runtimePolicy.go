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
		})
		if err != nil {
			return err
		}
		sumResp = append(sumResp, resp)
	}

	ms, err := checkProcessFileData(sumResp)
	if err != nil {
		return err
	}
	img.writePolicyFile(ms)
	return nil
}

func checkProcessFileData(sumResp []*opb.Response) (MatchSpec, error) {
	var filePaths SysRule
	ms := MatchSpec{}
	ms.OnEvent.Severity = 1
	ms.Description.Tldr = "Kubernetes serviceaccount folder access should be limited "
	ms.Name = "audit-serviceaccount-runtime"
	ms.OnEvent.Tags = []string{"KUBERNETES", "SERVICE ACCOUNT", "RUNTIME POLICY"}
	ms.OnEvent.Message = "serviceaccount access detected"
	ms.OnEvent.Action = "Audit"
	for _, eachResp := range sumResp {
		for _, fileData := range eachResp.FileData {
			if strings.HasPrefix(fileData.ProcName, saPath[0]) || strings.HasPrefix(fileData.ProcName, saPath[1]) {
				filePaths.FromSource = append(filePaths.FromSource, fileData.ParentProcName)
			}
		}
	}
	filePaths.Path = saPath
	filePaths.Recursive = true

	ms.Rules = Rules{
		FileRule: &filePaths,
	}
	return ms, nil
}
