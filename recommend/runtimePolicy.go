// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"
	"github.com/clarketm/json"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var img ImageInfo

func RuntimePolicy(options *Options) error {

	for _, labels := range options.UseLabels {
		label := strings.FieldsFunc(strings.TrimSpace(labels), MultiSplit)
		img.RepoTags = append(img.RepoTags, fmt.Sprintf("%s:%s", label[0], label[1]))

	}
	img.RepoTags = append(img.RepoTags, (strings.Join(options.UseLabels[:], ",")))

	err := summarySearch(strings.Join(options.UseLabels[:], ","), options.UseNamespace)
	if err != nil {
		return err
	}
	return nil
}

func summarySearch(labels, namespace string) error {

	if namespace == "" {
		namespace = "default"
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

	podResp, err := client.GetPodNames(context.Background(), &opb.Request{
		Label:     labels,
		NameSpace: namespace,
	})

	if err != nil {
		return err
	}
	for _, podName := range podResp.PodName {

		sumResp, err := client.Summary(context.Background(), &opb.Request{
			PodName: podName,
			Type:    "process",
		})
		if err != nil {
			return err
		}
		checkProcessFileData(sumResp)
		sumResp, err = client.Summary(context.Background(), &opb.Request{
			PodName: podName,
			Type:    "file",
		})
		if err != nil {
			return err
		}
		checkProcessFileData(sumResp)

	}

	return nil

}

func createRuntimePolicy(path, source, policyType string) {

	var pol MatchSpec
	if policyType == "file" {
		pol = MatchSpec{
			Rules: Rules{
				FileRule: &SysRule{
					FromSource: source,
					Path:       []string{path},
				},
			},
		}

	} else {
		pol = MatchSpec{
			Rules: Rules{
				ProcessRule: &SysRule{
					FromSource: source,
					Path:       []string{path},
				},
			},
		}

	}
	pol.Precondition = "/bin/.*"
	pol.Name = fmt.Sprintf("audit-serviceaccount-%s", strings.ToLower(randString(3)))
	pol.OnEvent.Tags = []string{"KUBERNETES", "SERVICE ACCOUNT", "RUNTIME POLICY"}
	pol.OnEvent.Message = "serviceaccount access detected"

	policy, _ := img.createPolicy(pol)

	poldir := fmt.Sprintf("%s/%s", options.OutDir, mkPathFromTag(img.RepoTags[0]))
	_ = os.Mkdir(poldir, 0750)

	outfile := fmt.Sprintf("%s/%s.yaml", poldir, policy.Metadata["name"])
	f, err := os.Create(filepath.Clean(outfile))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", outfile))
	}

	arr, _ := json.Marshal(policy)
	yamlarr, _ := yaml.JSONToYAML(arr)
	if _, err := f.WriteString(string(yamlarr)); err != nil {
		log.WithError(err).Error("WriteString failed")
	}

}

func checkProcessFileData(sumResp *opb.Response) error {
	if len(sumResp.ProcessData) > 0 {

		for _, procData := range sumResp.ProcessData {
			if strings.Contains(procData.ProcName, "/run/secrets/kubernetes.io/serviceaccount") {
				createRuntimePolicy(procData.ProcName, procData.ParentProcName, "process")

			}
		}

	}

	if len(sumResp.FileData) > 0 {

		for _, fileData := range sumResp.FileData {
			if strings.Contains(fileData.ProcName, "/run/secrets/kubernetes.io/serviceaccount") {
				createRuntimePolicy(fileData.ProcName, fileData.ParentProcName, "file")

			}

		}

	}
	return nil

}
