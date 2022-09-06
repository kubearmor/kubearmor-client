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

	//	policy.Metadata["containername"] = img.RepoTags[0]
	if ms.Name != "" {
		policy.Metadata["name"] = strings.TrimPrefix(fmt.Sprintf("%s-%s-%s", options.UseNamespace, mkPathFromTag(img.RepoTags[0]), ms.Name), "-")
	} else {
		policy.Metadata["name"] = "ksp-" + strings.TrimPrefix(fmt.Sprintf("%s-%s", options.UseNamespace, mkPathFromTag(img.RepoTags[0])), "-")
	}

	// Condition to set namespace, if user defined namespace value is available
	if options.UseNamespace != "" {
		policy.Metadata["namespace"] = options.UseNamespace
	}

	policy.Spec.Action = ms.OnEvent.Action
	policy.Spec.Severity = ms.OnEvent.Severity
	if ms.OnEvent.Message != "" {
		policy.Spec.Message = ms.OnEvent.Message
	}
	if len(ms.OnEvent.Tags) > 0 {
		policy.Spec.Tags = ms.OnEvent.Tags
	}

	// add container selector
	repotag := strings.Split(img.RepoTags[0], ":")
	// If user defined labels are present, update the matchLabels with them or use default matchLabels
	if len(options.UseLabels) > 0 {
		for _, uselabel := range options.UseLabels {
			userLabel := strings.FieldsFunc(strings.TrimSpace(uselabel), MultiSplit)
			policy.Spec.Selector.MatchLabels[userLabel[0]] = userLabel[1]
		}
	} else {
		policy.Spec.Selector.MatchLabels["kubearmor.io/container.name"] = repotag[0]
	}

	addPolicyRule(&policy, ms.Rules)
	return policy, nil
}

// MultiSplit function: to split string using multiple delimiters
func MultiSplit(r rune) bool {
	return r == ':' || r == '='
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

		poldir := fmt.Sprintf("%s/%s", options.OutDir, mkPathFromTag(img.RepoTags[0]))
		_ = os.Mkdir(poldir, 0750)

		outfile := fmt.Sprintf("%s/%s.yaml", poldir, policy.Metadata["name"])
		f, err := os.Create(filepath.Clean(outfile))
		if err != nil {
			log.WithError(err).Error(fmt.Sprintf("create file %s failed", outfile))
			continue
		}

		arr, _ := json.Marshal(policy)
		yamlarr, _ := yaml.JSONToYAML(arr)
		if _, err := f.WriteString(string(yamlarr)); err != nil {
			log.WithError(err).Error("WriteString failed")
		}
		if err := f.Sync(); err != nil {
			log.WithError(err).Error("file sync failed")
		}
		if err := f.Close(); err != nil {
			log.WithError(err).Error("file close failed")
		}
		_ = ReportRecord(ms, outfile)
		color.Green("created policy %s ...", outfile)
	}
	_ = ReportSectEnd(img)
}
