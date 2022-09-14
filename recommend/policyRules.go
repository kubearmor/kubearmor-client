// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding

	"errors"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	"github.com/accuknox/auto-policy-discovery/src/types"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
)

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name         string      `json:"name" yaml:"name"`
	Precondition string      `json:"precondition" yaml:"precondition"`
	Description  Description `json:"description" yaml:"description"`
	Yaml         string      `json:"yaml" yaml:"yaml"`
	Spec         Rules       `json:"spec,omitempty" yaml:"spec,omitempty"`
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

// SysRule specifics a file/process rule. Note that if the Path ends in "/" it is considered to be Directory rule
type SysRule struct {
	Severity         int                          `json:"severity" yaml:"severity"`
	Message          string                       `json:"message" yaml:"message"`
	Tags             []string                     `json:"tags" yaml:"tags"`
	Action           string                       `json:"action" yaml:"action"`
	MatchPaths       []types.KnoxMatchPaths       `json:"matchPaths,omitempty" yaml:"matchPaths,omitempty"`
	MatchDirectories []types.KnoxMatchDirectories `json:"matchDirectories,omitempty" yaml:"matchDirectories,omitempty"`
}

// NetRule specifies a KubeArmor network rule.
type NetRule struct {
	Severity       int                        `json:"severity" yaml:"severity"`
	Message        string                     `json:"message" yaml:"message"`
	Tags           []string                   `json:"tags" yaml:"tags"`
	Action         string                     `json:"action" yaml:"action"`
	MatchProtocols []types.KnoxMatchProtocols `json:"matchProtocols,omitempty" yaml:"matchProtocols,omitempty"`
}

type PolicyFile struct {
	APIVersion string            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       Rules             `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// Rules set of applicable rules. In the future, we might have other types of rules.
type Rules struct {
	Severity int            `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string         `json:"message,omitempty" yaml:"message,omitempty"`
	Selector types.Selector `json:"selector,omitempty" yaml:"selector,omitempty"`
	Process  SysRule        `json:"process,omitempty" yaml:"process,omitempty"`
	File     SysRule        `json:"file,omitempty" yaml:"file,omitempty"`
	Network  NetRule        `json:"network,omitempty" yaml:"network,omitempty"`
	Action   string         `json:"action,omitempty" yaml:"action,omitempty"`
}

var policyRules []MatchSpec

func updateRulesYAML(yamlFile []byte) {
	policyRulesJSON, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		color.Red("failed to convert policy rules yaml to json")
		log.WithError(err).Fatal("failed to convert policy rules yaml to json")
	}
	err = json.Unmarshal(policyRulesJSON, &policyRules)
	if err != nil {
		color.Red("failed to unmarshal policy rules")
		log.WithError(err).Fatal("failed to unmarshal policy rules")
	}
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
