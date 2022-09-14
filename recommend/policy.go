// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clarketm/json"

	"github.com/accuknox/auto-policy-discovery/src/types"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/yaml"
)

func addPolicyRule(policy *PolicyFile, r Rules) {
	if r.File.MatchPaths != nil {
		policy.Spec.File = r.File
	}
	if r.Process.MatchPaths != nil {
		policy.Spec.Process = r.Process
	}
	if r.Network.MatchProtocols != nil {
		policy.Spec.Network = r.Network
	}
}

func mkPathFromTag(tag string) string {
	r := strings.NewReplacer(
		"/", "-",
		":", "-",
		"\\", "-",
		".", "-",
		"@", "-",
	)
	return r.Replace(tag)
}

func (img *ImageInfo) createPolicy(ms MatchSpec) (PolicyFile, error) {
	policy := PolicyFile{
		APIVersion: "security.kubearmor.com/v1",
		Kind:       "KubeArmorPolicy",
		Metadata:   map[string]string{},
		Spec: Rules{
			Severity: 1, // by default
			Selector: types.Selector{
				MatchLabels: map[string]string{}},
		},
	}

	policy.Metadata["name"] = img.getPolicyName(ms.Name)

	if img.Namespace != "" {
		policy.Metadata["namespace"] = img.Namespace
	}

	policy.Spec.Action = ms.Spec.Action
	policy.Spec.Severity = ms.Spec.Severity
	if ms.Spec.Message != "" {
		policy.Spec.Message = ms.Spec.Message
	}
	if len(ms.Spec.Tags) > 0 {
		policy.Spec.Tags = ms.Spec.Tags
	}

	if len(img.Labels) > 0 {
		policy.Spec.Selector.MatchLabels = img.Labels
	} else {
		repotag := strings.Split(img.RepoTags[0], ":")
		policy.Spec.Selector.MatchLabels["kubearmor.io/container.name"] = repotag[0]
	}

	addPolicyRule(&policy, ms.Spec)
	return policy, nil
}

func (img *ImageInfo) checkPreconditions(ms MatchSpec) bool {
	matches := checkForSpec(filepath.Join(ms.Precondition), img.FileList)
	return len(matches) > 0
}

func matchTags(ms MatchSpec) bool {
	if len(options.Tags) <= 0 {
		return true
	}
	for _, t := range options.Tags {
		if slices.Contains(ms.Spec.Tags, t) {
			return true
		}
	}
	return false
}

func (img *ImageInfo) getPolicyFromImageInfo() {
	if img.OS != "linux" {
		color.Red("non-linux platforms are not supported, yet.")
		return
	}
	idx := 0
	if err := ReportStart(img); err != nil {
		log.WithError(err).Error("report start failed")
		return
	}
	ms, err := getNextRule(&idx)
	for ; err == nil; ms, err = getNextRule(&idx) {
		// matches preconditions

		if !matchTags(ms) {
			continue
		}

		if !img.checkPreconditions(ms) {
			continue
		}

		policy, err := img.createPolicy(ms)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"image": img, "spec": ms,
			}).Error("create policy failed, skipping")
			continue
		}

		outFile := img.getPolicyFile(ms.Name)
		_ = os.MkdirAll(filepath.Dir(outFile), 0750)

		f, err := os.Create(filepath.Clean(outFile))
		if err != nil {
			log.WithError(err).Error(fmt.Sprintf("create file %s failed", outFile))
			continue
		}

		arr, _ := json.Marshal(policy)
		yamlArr, _ := yaml.JSONToYAML(arr)
		if _, err := f.WriteString(string(yamlArr)); err != nil {
			log.WithError(err).Error("WriteString failed")
		}
		if err := f.Sync(); err != nil {
			log.WithError(err).Error("file sync failed")
		}
		if err := f.Close(); err != nil {
			log.WithError(err).Error("file close failed")
		}
		_ = ReportRecord(ms, outFile)
		color.Green("created policy %s ...", outFile)
	}
	_ = ReportSectEnd(img)
}
