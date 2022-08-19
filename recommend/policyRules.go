// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding
	"errors"

	"github.com/clarketm/json"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
)

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name         string      `json:"name"`
	Precondition string      `json:"precondition"`
	Description  Description `json:"description"`
	Rules        Rules       `json:"rules"`
	OnEvent      OnEvent     `json:"onEvent"`
}

// Ref for the policy rules
type Ref struct {
	Name string   `json:"name"`
	URL  []string `json:"url"`
}

// Description detailed description for the policy rule
type Description struct {
	Refs     []Ref  `json:"refs"`
	Tldr     string `json:"tldr"`
	Detailed string `json:"detailed"`
}

// PathRule specifics for the path/dir rule. Note that if the Path ends in "/" it is considered to be Directory rule
type PathRule struct {
	FromSource string   `json:"fromSource"`
	Path       []string `json:"path"`
	Recursive  bool     `json:"recursive"`
	Owneronly  bool     `json:"owneronly"`
}

// Rules set of applicable rules. In the future, we might have other types of rules.
type Rules struct {
	PathRule PathRule `json:"pathRule"`
}

// OnEvent the information that is emitted in the telemetry/alert when the matching event is witnessed
type OnEvent struct {
	Severity int      `json:"severity"`
	Message  string   `json:"message"`
	Tags     []string `json:"tags"`
	Action   string   `json:"action"`
}

var policyMatch []MatchSpec

//go:embed json/rules.json
var ruleSpecJSON []byte

func init() {
	err := json.Unmarshal(ruleSpecJSON, &policyMatch)
	if err != nil {
		color.Red("failed to unmarshal json rules")
		log.WithError(err).Fatal("failed to unmarshal json rules")
	}
}

func getNextRule(idx *int) (MatchSpec, error) {
	if *idx < 0 {
		(*idx)++
	}
	if *idx >= len(policyMatch) {
		return MatchSpec{}, errors.New("no rule at idx")
	}
	r := policyMatch[*idx]
	(*idx)++
	return r, nil
}
