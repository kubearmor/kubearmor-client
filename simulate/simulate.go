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
	if err != nil {
		return fmt.Errorf("unable to read policy file; %s", err.Error())
	}
	karmorPolicy := &tp.K8sKubeArmorPolicy{}
	js, err := yaml.YAMLToJSON(policyFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(js, karmorPolicy)
	if err != nil {
		return err
	}
	pr := walkProcessTree(karmorPolicy)
	fmt.Printf("%v", pr)
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

// walkProcessTree takes the src kubearmor policy and returns all the associated process rules
func walkProcessTree(src *tp.K8sKubeArmorPolicy) *processRules {
	pr := processRules{}
	if len(src.Spec.Process.MatchPaths) > 0 {
		for i, rule := range src.Spec.Process.MatchPaths {
			switch src.Spec.Action {
			case "Allow":
				pr.action = Allow
			case "Block":
				pr.action = Block
			case "Audit":
				pr.action = Audit
				// if the action does not match any of these cases use default posture?
			}

			mr := matchRule{
				path:        rule.Path,
				isownerOnly: rule.OwnerOnly,
				isDir:       false,
			}
			if len(rule.FromSource) > 0 {
				mr.fromSource = rule.FromSource[i].Path
			}

			pr.rules = append(pr.rules, mr)

		}
	}
	if len(src.Spec.Process.MatchDirectories) > 0 {
		for i, rule := range src.Spec.Process.MatchDirectories {
			switch src.Spec.Action {
			case "Allow":
				pr.action = Allow
			case "Block":
				pr.action = Block
			case "Audit":
				pr.action = Audit
				// if the action does not match any of these cases use default posture?
			}

			mr := matchRule{
				path:        rule.Directory,
				isownerOnly: rule.OwnerOnly,
				isDir:       true,
			}
			if len(rule.FromSource) > 0 {
				mr.fromSource = rule.FromSource[i].Path
			}
			pr.rules = append(pr.rules, mr)
		}
	}
	return &pr

}
