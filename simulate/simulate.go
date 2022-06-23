package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"sigs.k8s.io/yaml"
)

// Options Structure
type Options struct {
	Config string
	Action string
	Policy string
}
type SimulationOutput struct {
	ClusterName   string
	HostName      string
	PodName       string
	ContainerID   string
	ContainerName string
	Labels        string
	Policy        string
	Severity      int
	Type          string
	Source        string
	Operation     string
	Resource      string
	Data          string
	Action        string
	Result        string
}

type KubeArmorCfg struct {
	DefaultPosture string
}

func StartSimulation(o Options) error {
	policyFile, err := os.ReadFile(filepath.Clean(o.Policy))
	karmorPolicy := &tp.K8sKubeArmorPolicy{}
	response := &SimulationOutput{
		ClusterName: "Unkown",
		HostName:    "Unkown",
		PodName:     "Unknown",
		Labels:      "Unkown",
		Type:        "MatchedPolicy",
		ContainerID: "Unkown",
	}
	if err != nil {
		return fmt.Errorf("unable to read policy file; %s", err.Error())
	}
	js, err := yaml.YAMLToJSON(policyFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(js, karmorPolicy)
	if err != nil {
		return err
	}
	if len(karmorPolicy.Spec.Process.MatchPaths) > 0 {
		response.Resource = karmorPolicy.Spec.Process.MatchPaths[0].Path
		response.Policy = o.Policy
		response.Severity = 1
		response.Source = karmorPolicy.Spec.Process.MatchPaths[0].Path
		response.Data = "syscall=SYS_EXECVE"
		response.Action = karmorPolicy.Spec.Process.Action
		response.Result = "Permission Denied"
		PrintResults(response, "Block")
		return nil
	} else if len(karmorPolicy.Spec.File.MatchPaths) > 0 {
		response.Resource = karmorPolicy.Spec.File.MatchPaths[0].Path
		response.Policy = o.Policy
		response.Severity = 1
		response.Source = karmorPolicy.Spec.File.MatchPaths[0].Path
		response.Data = "syscall=SYS_FOPEN"
		if karmorPolicy.Spec.Action == "Block" {
			response.Action = "Block"
			response.Result = "Permission Denied"
		} else if karmorPolicy.Spec.Action == "Allow" {
			response.Action = "Allow"
			response.Resource = ""
		}

	} else if len(karmorPolicy.Spec.Network.MatchProtocols) > 0 {
		// todo implement match protocols
	}

	return nil
}

func PrintResults(out *SimulationOutput, title string) {
	fmt.Printf(`
	Action: %s
	Telemetry Event:
	== Alert ==
	Cluster Name: %s
	Host Name: %s
	Namespace Name: %s
	Pod Name: %s
	Container ID: %s
	Container Name: %s
	Labels: %s
	Policy: %s
	Severity: %d
	Type: %s
	Source: %s
	Operation: %s
	Resource: %s
	Data: %s
	Action: %s
	Result: %s
	`, title, out.ClusterName, out.HostName, out.PodName, out.ContainerID, out.ContainerName, out.Labels, out.Policy, out.Severity, out.Type, out.Source, out.Operation, out.Resource, out.Data, out.Action, out.Result)
}
