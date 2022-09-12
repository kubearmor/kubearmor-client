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

func (r *SysRule) convertToKnoxRule() types.KnoxSys {
	knoxRule := types.KnoxSys{}
	fromSourceArr := []types.KnoxFromSource{}

	if r.FromSource != "" {
		if strings.HasSuffix(r.FromSource, "/") {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Dir: r.FromSource,
			})
		} else {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Path: r.FromSource,
			})
		}
	}
	for _, path := range r.Path {
		if strings.HasSuffix(path, "/") {

			dirRule := types.KnoxMatchDirectories{
				Dir:        path,
				Recursive:  r.Recursive,
				FromSource: fromSourceArr,
				OwnerOnly:  r.OwnerOnly,
			}
			knoxRule.MatchDirectories = append(knoxRule.MatchDirectories, dirRule)
		} else {
			pathRule := types.KnoxMatchPaths{
				Path:       path,
				FromSource: fromSourceArr,
				OwnerOnly:  r.OwnerOnly,
			}
			knoxRule.MatchPaths = append(knoxRule.MatchPaths, pathRule)
		}
	}

	return knoxRule
}

// networkToKnoxRule function to include KubeArmor network rules
func (r *NetRule) networkToKnoxRule() types.NetworkRule {
	knoxNetRule := types.NetworkRule{}
	fromSourceArr := []types.KnoxFromSource{}

	if r.FromSource != "" {
		if strings.HasSuffix(r.FromSource, "/") {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Dir: r.FromSource,
			})
		} else {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Path: r.FromSource,
			})
		}
	}
	for _, protocol := range r.Protocol {

		protoRule := types.KnoxMatchProtocols{
			Protocol:   protocol,
			FromSource: fromSourceArr,
		}
		knoxNetRule.MatchProtocols = append(knoxNetRule.MatchProtocols, protoRule)

	}

	return knoxNetRule
}

func addPolicyRule(policy *types.KubeArmorPolicy, r Rules) {
	if r.FileRule != nil {
		policy.Spec.File = r.FileRule.convertToKnoxRule()
	}
	if r.ProcessRule != nil {
		policy.Spec.Process = r.ProcessRule.convertToKnoxRule()
	}
	if r.NetworkRule != nil {
		policy.Spec.Network = r.NetworkRule.networkToKnoxRule()
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

func (img *ImageInfo) createPolicy(ms MatchSpec) (types.KubeArmorPolicy, error) {
	policy := types.KubeArmorPolicy{
		APIVersion: "security.kubearmor.com/v1",
		Kind:       "KubeArmorPolicy",
		Metadata:   map[string]string{},
		Spec: types.KnoxSystemSpec{
			Severity: 1, // by default
			Selector: types.Selector{
				MatchLabels: map[string]string{}},
		},
	}

	policy.Metadata["name"] = img.getPolicyName(ms.Name)

	if img.Namespace != "" {
		policy.Metadata["namespace"] = img.Namespace
	}

	policy.Spec.Action = ms.OnEvent.Action
	policy.Spec.Severity = ms.OnEvent.Severity
	if ms.OnEvent.Message != "" {
		policy.Spec.Message = ms.OnEvent.Message
	}
	if len(ms.OnEvent.Tags) > 0 {
		policy.Spec.Tags = ms.OnEvent.Tags
	}

	if len(img.Labels) > 0 {
		policy.Spec.Selector.MatchLabels = img.Labels
	} else {
		repotag := strings.Split(img.RepoTags[0], ":")
		policy.Spec.Selector.MatchLabels["kubearmor.io/container.name"] = repotag[0]
	}

	addPolicyRule(&policy, ms.Rules)
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
		if slices.Contains(ms.OnEvent.Tags, t) {
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
