package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"sigs.k8s.io/yaml"
)

// Options Structure
type Options struct {
	Config string
	Action string
	Policy string
}

type KubeArmorCfg struct {
	DefaultPosture string
}

func StartSimulation(o Options) error {
	policyFile, err := os.ReadFile(filepath.Clean(o.Policy))
	if err != nil {
		return err
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
	// pr := walkProcessTree(karmorPolicy)
	// fmt.Printf("%v", pr)
	action, err := GetUserAction(o.Action)
	if err != nil {
		return err
	}
	fmt.Printf("%v", action)
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
		for _, rule := range src.Spec.Process.MatchPaths {
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
				for _, path := range rule.FromSource {
					mr.fromSource = append(mr.fromSource, path.Path)
				}
			}

			pr.rules = append(pr.rules, mr)

		}
	}
	if len(src.Spec.Process.MatchDirectories) > 0 {
		for _, rule := range src.Spec.Process.MatchDirectories {
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
				for _, path := range rule.FromSource {
					mr.fromSource = append(mr.fromSource, path.Path)
				}
			}
			pr.rules = append(pr.rules, mr)
		}
	}
	return &pr

}

// GetUserAction takes an input action and returns a slice of the action type(exec,fopen,socket...) along with the corresponding path
// eg "exec:/bin/sleep" -> [exec,/bin/sleep]
func GetUserAction(action string) ([]string, error) {
	// check if the action is supported
	supportedActions := []string{"exec", "fopen", "socket", "accept"}
	act := action[:strings.IndexByte(action, ':')]
	if !contains(supportedActions, act) {
		return []string{}, fmt.Errorf("the supplied action is currently unsupported. Supported actions: %v", supportedActions)
	}
	path := strings.Join(strings.Split(action, ":")[1:], ":")
	return []string{act, path}, nil
}

func contains(src []string, target string) bool {
	for _, a := range src {
		if a == target {
			return true
		}
	}
	return false
}
