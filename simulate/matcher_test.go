// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"testing"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testPolicy(action string, spec tp.SecuritySpec) tp.K8sKubeArmorPolicy {
	spec.Action = action
	return tp.K8sKubeArmorPolicy{
		Metadata: metav1.ObjectMeta{Name: "test-policy"},
		Spec:     spec,
	}
}

func TestEvaluateEvent_processPathBlock(t *testing.T) {
	policy := testPolicy("Block", tp.SecuritySpec{
		Process: tp.ProcessType{
			MatchPaths: []tp.ProcessPathType{{Path: "/bin/bash"}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "Process", ProcessName: "/bin/bash", PodName: "api-74x"}
	r := EvaluateEvent(policy, rules, event)
	if r.Verdict != VerdictWouldBlock {
		t.Fatalf("verdict = %q, want %q", r.Verdict, VerdictWouldBlock)
	}
}

func TestEvaluateEvent_processPathAllowed(t *testing.T) {
	policy := testPolicy("Block", tp.SecuritySpec{
		Process: tp.ProcessType{
			MatchPaths: []tp.ProcessPathType{{Path: "/bin/bash"}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "Process", ProcessName: "/usr/bin/curl", PodName: "api-74x"}
	r := EvaluateEvent(policy, rules, event)
	if r.Verdict != VerdictAllowed {
		t.Fatalf("verdict = %q, want %q", r.Verdict, VerdictAllowed)
	}
}

func TestEvaluateEvent_processDirectoryBlock(t *testing.T) {
	policy := testPolicy("Block", tp.SecuritySpec{
		Process: tp.ProcessType{
			MatchDirectories: []tp.ProcessDirectoryType{{Directory: "/bin/", Recursive: true}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "Process", ProcessName: "/bin/sh", PodName: "test-pod"}
	r := EvaluateEvent(policy, rules, event)
	if r.Verdict != VerdictWouldBlock {
		t.Fatalf("verdict = %q, want %q", r.Verdict, VerdictWouldBlock)
	}
}

func TestEvaluateEvent_auditVerdict(t *testing.T) {
	policy := testPolicy("Audit", tp.SecuritySpec{
		Process: tp.ProcessType{
			MatchPaths: []tp.ProcessPathType{{Path: "/bin/bash"}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "Process", ProcessName: "/bin/bash"}
	r := EvaluateEvent(policy, rules, event)
	if r.Verdict != VerdictWouldAudit {
		t.Fatalf("verdict = %q, want %q", r.Verdict, VerdictWouldAudit)
	}
}

func TestBuildRules_skipsFromSource(t *testing.T) {
	spec := tp.SecuritySpec{
		Process: tp.ProcessType{
			MatchPaths: []tp.ProcessPathType{
				{Path: "/secret.txt", FromSource: []tp.MatchSourceType{{Path: "/bin/cat"}}},
				{Path: "/bin/bash"},
			},
		},
	}
	rules := BuildRules(spec)
	if len(rules) != 1 {
		t.Fatalf("rules len = %d, want 1 (fromSource path skipped)", len(rules))
	}
	if rules[0].Resource != "/bin/bash" {
		t.Fatalf("rule resource = %q", rules[0].Resource)
	}
}

func TestLabelsMatch(t *testing.T) {
	if !LabelsMatch("app=myapp,env=prod", map[string]string{"app": "myapp"}) {
		t.Fatal("expected label match")
	}
	if LabelsMatch("app=other", map[string]string{"app": "myapp"}) {
		t.Fatal("expected label mismatch")
	}
}

func TestEvaluateEvent_filePath(t *testing.T) {
	policy := testPolicy("Block", tp.SecuritySpec{
		File: tp.FileType{
			MatchPaths: []tp.FilePathType{{Path: "/etc/passwd"}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "File", Resource: "/etc/passwd"}
	r := EvaluateEvent(policy, rules, event)
	if r.Verdict != VerdictWouldBlock {
		t.Fatalf("verdict = %q, want %q", r.Verdict, VerdictWouldBlock)
	}
}

func TestEvaluateEvent_selectorMismatch(t *testing.T) {
	policy := testPolicy("Block", tp.SecuritySpec{
		Selector: tp.SelectorType{MatchLabels: map[string]string{"app": "myapp"}},
		Process: tp.ProcessType{
			MatchPaths: []tp.ProcessPathType{{Path: "/bin/bash"}},
		},
	})
	rules := BuildRules(policy.Spec)
	event := pb.Log{Operation: "Process", ProcessName: "/bin/bash", Labels: "app=other"}
	r := EvaluateEvent(policy, rules, event)
	if r.Matched {
		t.Fatal("expected no match when labels mismatch")
	}
}
