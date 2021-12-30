package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func sendPolicyOverGrpc(o PolicyOptions, policyEventData []byte) error {

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

	if resp, err := client.HostPolicy(context.Background(), &req); err == nil {
		if resp.Status == 1 {
			fmt.Print("Success")
		} else {
			return fmt.Errorf("failed to send policy")
		}
	}

	return nil
}

func sendPolicyOverHttp(policyEventData []byte) error {

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "http://127.0.0.1:8080/policy", bytes.NewBuffer(policyEventData))
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Response from non-k8s control plane : [%s]", string(respBody))
	return nil
}

//PolicyHandling Function recives path to YAML file with the type of event and emits an Host Policy Event to KubeArmor gRPC/HTTP Server
func PolicyHandling(t string, path string, o PolicyOptions) error {

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

	if err = sendPolicyOverHttp(policyEventData); err != nil {
		// HTTP connection is not active, hence trying to send policy over gRPC
		if err = sendPolicyOverGrpc(o, policyEventData); err != nil {
			// Failed to send policy over gRPC
			return err
		}
	}
	return nil
}
