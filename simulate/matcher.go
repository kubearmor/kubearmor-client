// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"path/filepath"
	"strings"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"
)

// Verdict labels for simulation output.
const (
	VerdictWouldBlock = "WOULD BLOCK"
	VerdictWouldAudit = "WOULD AUDIT"
	VerdictAllowed    = "ALLOWED"
)

// MatchRule is a flattened policy rule used for evaluation.
type MatchRule struct {
	Operation    string // Process or File
	ResourceType string // Path, Directory, Glob, ExecName
	Resource     string
	Recursive    bool
}

// EventResult is the simulation outcome for one telemetry event.
type EventResult struct {
	Verdict       string
	DisplayPath   string
	PID           int32
	PodName       string
	ContainerName string
	Matched       bool
}

// BuildRules expands a policy spec into match rules (skips fromSource entries in v1).
func BuildRules(spec tp.SecuritySpec) []MatchRule {
	var rules []MatchRule

	addProcess := func(pt tp.ProcessType) {
		for _, p := range pt.MatchPaths {
			if len(p.FromSource) > 0 {
				continue
			}
			res := p.Path
			rtype := "Path"
			if p.ExecName != "" {
				res = p.ExecName
				rtype = "ExecName"
			}
			if res == "" {
				continue
			}
			rules = append(rules, MatchRule{Operation: "Process", ResourceType: rtype, Resource: res})
		}
		for _, d := range pt.MatchDirectories {
			if len(d.FromSource) > 0 {
				continue
			}
			dir := d.Directory
			if dir != "" && !strings.HasSuffix(dir, "/") {
				dir += "/"
			}
			rules = append(rules, MatchRule{
				Operation: "Process", ResourceType: "Directory", Resource: dir, Recursive: d.Recursive,
			})
		}
		for _, pat := range pt.MatchPatterns {
			if pat.Pattern == "" {
				continue
			}
			rules = append(rules, MatchRule{Operation: "Process", ResourceType: "Glob", Resource: pat.Pattern})
		}
	}

	addFile := func(ft tp.FileType) {
		for _, p := range ft.MatchPaths {
			if len(p.FromSource) > 0 {
				continue
			}
			if p.Path == "" {
				continue
			}
			rules = append(rules, MatchRule{Operation: "File", ResourceType: "Path", Resource: p.Path})
		}
		for _, d := range ft.MatchDirectories {
			if len(d.FromSource) > 0 {
				continue
			}
			dir := d.Directory
			if dir != "" && !strings.HasSuffix(dir, "/") {
				dir += "/"
			}
			rules = append(rules, MatchRule{
				Operation: "File", ResourceType: "Directory", Resource: dir, Recursive: d.Recursive,
			})
		}
		for _, pat := range ft.MatchPatterns {
			if pat.Pattern == "" {
				continue
			}
			rules = append(rules, MatchRule{Operation: "File", ResourceType: "Glob", Resource: pat.Pattern})
		}
	}

	addProcess(spec.Process)
	addFile(spec.File)
	return rules
}

// LabelsMatch checks event labels against policy selector matchLabels.
func LabelsMatch(eventLabels string, matchLabels map[string]string) bool {
	if len(matchLabels) == 0 {
		return true
	}
	if eventLabels == "" {
		return false
	}
	labels := parseLabelString(eventLabels)
	for k, v := range matchLabels {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func parseLabelString(s string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
}

// EvaluateEvent runs policy rules against one log event.
func EvaluateEvent(policy tp.K8sKubeArmorPolicy, rules []MatchRule, event pb.Log) EventResult {
	display := displayPath(event)
	result := EventResult{
		Verdict:       VerdictAllowed,
		DisplayPath:   display,
		PID:           event.PID,
		PodName:       event.PodName,
		ContainerName: event.ContainerName,
	}

	if !LabelsMatch(event.Labels, policy.Spec.Selector.MatchLabels) {
		return result
	}

	for _, rule := range rules {
		if matchRule(rule, event) {
			result.Matched = true
			result.Verdict = verdictForAction(policy.Spec.Action)
			return result
		}
	}
	return result
}

func verdictForAction(action string) string {
	switch strings.ToLower(action) {
	case "block":
		return VerdictWouldBlock
	case "audit":
		return VerdictWouldAudit
	default:
		return VerdictAllowed
	}
}

func displayPath(event pb.Log) string {
	if event.Operation == "Process" || event.Operation == "process" {
		if event.ProcessName != "" {
			return event.ProcessName
		}
	}
	if event.Resource != "" {
		return strings.Split(event.Resource, " ")[0]
	}
	if event.ProcessName != "" {
		return event.ProcessName
	}
	return event.Source
}

func matchRule(rule MatchRule, event pb.Log) bool {
	op := strings.ToLower(event.Operation)
	ruleOp := strings.ToLower(rule.Operation)
	if op != ruleOp && op != "" {
		// Allow File rules to match when operation is empty but resource is set
		if !(ruleOp == "file" && event.Resource != "") {
			return false
		}
	}

	switch rule.Operation {
	case "File":
		return matchFileRule(rule, event)
	case "Process":
		return matchProcessRule(rule, event)
	}
	return false
}

func matchFileRule(rule MatchRule, event pb.Log) bool {
	firstResource := strings.Split(event.Resource, " ")[0]
	switch rule.ResourceType {
	case "Path":
		return rule.Resource == firstResource
	case "Directory":
		return matchDirectory(firstResource, rule.Resource, rule.Recursive)
	case "Glob":
		ok, _ := filepath.Match(rule.Resource, firstResource)
		return ok
	}
	return false
}

func matchProcessRule(rule MatchRule, event pb.Log) bool {
	name := event.ProcessName
	if name == "" {
		name = strings.Split(event.Resource, " ")[0]
	}
	switch rule.ResourceType {
	case "Path", "ExecName":
		return rule.Resource == name
	case "Directory":
		return matchDirectory(name, rule.Resource, rule.Recursive)
	case "Glob":
		ok, _ := filepath.Match(rule.Resource, name)
		return ok
	}
	return false
}

func getDirectoryPart(path string) string {
	dir := filepath.Dir(path)
	if strings.HasPrefix(dir, "/") {
		return dir + "/"
	}
	return "__not_absolute_path__"
}

func matchDirectory(path, policyDir string, recursive bool) bool {
	pathDir := getDirectoryPart(path)
	dirCount := strings.Count(pathDir, "/")
	policyCount := strings.Count(policyDir, "/")

	if policyDir == "/" && recursive {
		return strings.HasPrefix(pathDir, "/")
	}

	if !strings.HasPrefix(pathDir, policyDir) {
		return false
	}
	if !recursive && dirCount != policyCount {
		return false
	}
	if recursive && dirCount < policyCount {
		return false
	}
	if policyDir == path+"/" || policyDir == pathDir {
		return true
	}
	return strings.HasPrefix(pathDir, policyDir)
}
