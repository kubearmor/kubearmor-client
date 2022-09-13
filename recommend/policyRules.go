// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding
	"fmt"
	"os"
	"path/filepath"

	"errors"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
)

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name         string      `json:"name" yaml:"name"`
	Precondition string      `json:"precondition" yaml:"precondition"`
	Description  Description `json:"description" yaml:"description"`
	Rules        Rules       `json:"rules" yaml:"rules"`
	OnEvent      OnEvent     `json:"onEvent" yaml:"onEvent"`
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
	FromSource string   `json:"fromSource" yaml:"fromSource"`
	Path       []string `json:"path" yaml:"path"`
	Recursive  bool     `json:"recursive" yaml:"recursive"`
	OwnerOnly  bool     `json:"ownerOnly" yaml:"ownerOnly"`
}

// NetRule specifies a KubeArmor network rule.
type NetRule struct {
	FromSource string   `json:"fromSource" yaml:"fromSource"`
	Protocol   []string `json:"protocol" yaml:"protocol"`
}

// Rules set of applicable rules. In the future, we might have other types of rules.
type Rules struct {
	FileRule    *SysRule `json:"fileRule" yaml:"fileRule"`
	ProcessRule *SysRule `json:"processRule" yaml:"processRule"`
	NetworkRule *NetRule `json:"networkRule" yaml:"networkRule"`
}

// OnEvent the information that is emitted in the telemetry/alert when the matching event is witnessed
type OnEvent struct {
	Severity int      `json:"severity" yaml:"severity"`
	Message  string   `json:"message" yaml:"message"`
	Tags     []string `json:"tags" yaml:"tags"`
	Action   string   `json:"action" yaml:"action"`
}

var policyRules []MatchSpec

func updateRulesYAML() {

	yamlFile, err := os.ReadFile(filepath.Clean(fmt.Sprintf("%s/.cache/karmor/rules.yaml", userHome())))
	if err != nil {
		color.Red("failed to read rules.yaml")
		log.WithError(err).Fatal("failed to read rules.yaml")
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
