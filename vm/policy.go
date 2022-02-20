package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"

	"sigs.k8s.io/yaml"

	"google.golang.org/grpc"
)

//PolicyOptions are optional configuration for kArmor vm policy
type PolicyOptions struct {
	GRPC string
}

func sendPolicyOverGRPC(o PolicyOptions, policyEventData []byte) error {

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

	resp, err := client.HostPolicy(context.Background(), &req)
	if err != nil || resp.Status != 1 {
		return fmt.Errorf("failed to send policy")
	}

	fmt.Println("Success")
	return nil
}

func sendPolicyOverHTTP(address string, policyEventData []byte) error {

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", address+"/policy/kubearmor", bytes.NewBuffer(policyEventData))
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
func PolicyHandling(t string, path string, o PolicyOptions, httpAddress string, isNonK8sEnv bool) error {

	var policy tp.K8sKubeArmorHostPolicy
	policyFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(policyFile, &policy)
	if err != nil {
		return err
	}

	policyEvent := tp.K8sKubeArmorHostPolicyEvent{
		Type:   t,
		Object: policy,
	}

	policyEventData, err := json.Marshal(policyEvent)
	if err != nil {
		return err
	}

	if isNonK8sEnv {
		// Non-K8s control plane with kvmservice, hence send policy over HTTP
		if err = sendPolicyOverHTTP(httpAddress, policyEventData); err != nil {
			return err
		}
	} else {
		// Systemd mode, hence send policy over gRPC
		if err = sendPolicyOverGRPC(o, policyEventData); err != nil {
			return err
		}
	}
	return nil
}
