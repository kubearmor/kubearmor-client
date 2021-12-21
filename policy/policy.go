package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"

	"sigs.k8s.io/yaml"

	"google.golang.org/grpc"
)

//PolicyOptions are optional configuration for kArmor vm policy
type PolicyOptions struct {
	GRPC string
}

//PolicyHandling Function recives path to YAML file with the type of event and emits an Host Policy Event to KubeArmor gRPC Server
func PolicyHandling(t string, path string, o PolicyOptions) error {
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
