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
	Policy    string
	Severity  int
	Type      string
	Source    string
	Operation string
	Resource  string
	Data      string
	Action    string
	Result    string
}

type KubeArmorCfg struct {
	DefaultPosture string
}

func StartSimulation(o Options) error {
	policyFile, err := os.ReadFile(filepath.Clean(o.Policy))
	karmorPolicy := &tp.K8sKubeArmorPolicy{}
	response := &SimulationOutput{
		Type: "MatchedPolicy",
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
		response.Policy = karmorPolicy.Metadata.Name
		response.Severity = karmorPolicy.Spec.Severity
		response.Source = karmorPolicy.Spec.Process.MatchPaths[0].Path
		response.Data = "syscall=SYS_EXECVE"
		response.Action = karmorPolicy.Spec.Process.Action
		response.Result = "Permission Denied"
		printSimulation(response, response.Action)
	} else if len(karmorPolicy.Spec.File.MatchDirectories) > 0 {
		response.Resource = karmorPolicy.Spec.File.MatchDirectories[0].Directory
		response.Policy = karmorPolicy.Metadata.Name
		response.Severity = karmorPolicy.Spec.Severity
		response.Source = karmorPolicy.Spec.File.MatchPaths[0].Path
		response.Data = "syscall=SYS_OPENAT"

		// Todo: Consider kubearmor.cfg
		if karmorPolicy.Spec.File.Action == "Deny" {
			response.Action = karmorPolicy.Spec.Action
			response.Result = "Permission Denied"
		}
		response.Action = karmorPolicy.Spec.Action
		response.Result = "Success"
	} else if len(karmorPolicy.Spec.Network.MatchProtocols) > 0 {
		// todo implement match protocols
	}

	return nil
}

func printSimulation(out *SimulationOutput, title string) {
	fmt.Printf("Action: %s", title)
	fmt.Printf(`
Telemetry Event:
== Alert ==
Cluster Name: unknown
Host Name: unknown
Namespace Name: unknown
Pod Name: unknown
Container ID: unknown
Container Name: unknown
Labels: unknown
Policy: %s
Severity: %d
Type: MatchedPolicy
Source: %s
Operation: %s
Resource: %s
Data: %s
Action: %s
Result: %s
`, out.Policy, out.Severity, out.Source, out.Operation, out.Resource, out.Data, out.Action, out.Result)
}
