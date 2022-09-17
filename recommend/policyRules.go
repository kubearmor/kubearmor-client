// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding

	"errors"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	"github.com/fatih/color"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	log "github.com/sirupsen/logrus"
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

//go:embed yaml/rules.yaml
var policyRulesYAML []byte

func updateRulesYAML(yamlFile []byte) string {
	policyRules = []MatchSpec{}
	if len(yamlFile) < 30 {
		yamlFile = policyRulesYAML
	}
	policyRulesJSON, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		color.Red("failed to convert policy rules yaml to json")
		log.WithError(err).Fatal("failed to convert policy rules yaml to json")
	}
	var jsonRaw map[string]json.RawMessage
	err = json.Unmarshal(policyRulesJSON, &jsonRaw)
	if err != nil {
		color.Red("failed to unmarshal policy rules json")
		log.WithError(err).Fatal("failed to unmarshal policy rules json")
	}
	err = json.Unmarshal(jsonRaw["policyRules"], &policyRules)
	if err != nil {
		color.Red("failed to unmarshal policy rules")
		log.WithError(err).Fatal("failed to unmarshal policy rules")
	}
	return string(jsonRaw["version"])
}

func getNextRule(idx *int) (MatchSpec, error) {
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
