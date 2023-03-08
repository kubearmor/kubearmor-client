// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"errors"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
)

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name         string                  `json:"name" yaml:"name"`
	Precondition []string                `json:"precondition" yaml:"precondition"`
	Description  Description             `json:"description" yaml:"description"`
	Yaml         string                  `json:"yaml" yaml:"yaml"`
	Spec         pol.KubeArmorPolicySpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// Ref for the policy rules
type Ref struct {
	Name string   `json:"name" yaml:"name"`
	URL  []string `json:"url" yaml:"url"`
}

// Description detailed description for the policy rule
type Description struct {
	Refs     []Ref  `json:"refs" yaml:"refs"`
	Tldr     string `json:"tldr" yaml:"tldr"`
	Detailed string `json:"detailed" yaml:"detailed"`
}

var policyRules []MatchSpec

func updateRulesYAML(yamlFile []byte, policyRules *[]MatchSpec) (string, error) {
	policyRulesJSON, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		return "", err
	}
	var jsonRaw map[string]json.RawMessage
	err = json.Unmarshal(policyRulesJSON, &jsonRaw)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(jsonRaw["policyRules"], policyRules)

	if err != nil {
		return "", err
	}
	return string(jsonRaw["version"]), nil
}

func getNextRule(idx *int, policyRules []MatchSpec) (MatchSpec, error) {
	if *idx < 0 {
		(*idx)++
	}
	if *idx >= len(policyRules) {
		return MatchSpec{}, errors.New("no rule at idx")
	}
	r := policyRules[*idx]
	(*idx)++
	return r, nil
}
