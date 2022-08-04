// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
)

type MatchSpec struct {
	Name         string  `json:"name"`
	Precondition string  `json:"precondition"`
	Rules        Rules   `json:"rules"`
	OnEvent      OnEvent `json:"onEvent"`
}

type PathRule struct {
	FromSource string   `json:"fromSource"`
	Path       []string `json:"path"`
	Recursive  bool     `json:"recursive"`
}

type Rules struct {
	PathRule PathRule `json:"pathRule"`
}

type OnEvent struct {
	Severity int      `json:"severity"`
	Message  string   `json:"message"`
	Tags     []string `json:"tags"`
	Action   string   `json:"action"`
}

var policyMatch []MatchSpec

//go:embed rules.json
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
