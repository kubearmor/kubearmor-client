// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"sigs.k8s.io/yaml"
)

const kubeArmorPolicyKind = "KubeArmorPolicy"

var blankDocRE = regexp.MustCompile(`^\s*$`)

// LoadPolicy reads the first KubeArmorPolicy document from a YAML file.
func LoadPolicy(path string) (tp.K8sKubeArmorPolicy, error) {
	var policy tp.K8sKubeArmorPolicy

	policyFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return policy, fmt.Errorf("read policy file: %w", err)
	}

	docs := strings.Split(string(policyFile), "---")
	for _, doc := range docs {
		if blankDocRE.MatchString(doc) {
			continue
		}

		js, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			return policy, fmt.Errorf("parse policy YAML: %w", err)
		}

		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(js, &kind); err != nil {
			return policy, fmt.Errorf("parse policy kind: %w", err)
		}

		switch kind.Kind {
		case kubeArmorPolicyKind:
			if err := json.Unmarshal(js, &policy); err != nil {
				return policy, fmt.Errorf("parse KubeArmorPolicy: %w", err)
			}
			if policy.Metadata.Name == "" {
				return policy, fmt.Errorf("policy metadata.name is required")
			}
			return policy, nil
		case "KubeArmorHostPolicy":
			return policy, fmt.Errorf("KubeArmorHostPolicy is not supported in simulate v1 (use KubeArmorPolicy)")
		case "KubeArmorNetworkPolicy":
			return policy, fmt.Errorf("KubeArmorNetworkPolicy is not supported in simulate v1 (use KubeArmorPolicy)")
		default:
			if kind.Kind != "" {
				return policy, fmt.Errorf("unsupported policy kind %q (expected KubeArmorPolicy)", kind.Kind)
			}
		}
	}

	return policy, fmt.Errorf("no KubeArmorPolicy document found in %s", path)
}
