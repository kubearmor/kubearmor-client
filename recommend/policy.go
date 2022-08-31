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

func addPolicyRule(policy *types.KubeArmorPolicy, r Rules) {
	var fromSourceArr []types.KnoxFromSource
	pr := r.PathRule
	if pr.FromSource != "" {
		if strings.HasSuffix(pr.FromSource, "/") {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Dir: pr.FromSource,
			})
		} else {
			fromSourceArr = append(fromSourceArr, types.KnoxFromSource{
				Path: pr.FromSource,
			})
		}
	}
	for _, path := range pr.Path {
		if strings.HasSuffix(path, "/") {
			dirRule := types.KnoxMatchDirectories{
				Dir:        path,
				Recursive:  pr.Recursive,
				FromSource: fromSourceArr,
			}
			policy.Spec.File.MatchDirectories = append(policy.Spec.File.MatchDirectories, dirRule)
		} else {
			pathRule := types.KnoxMatchPaths{
				Path:       path,
				FromSource: fromSourceArr,
			}
			policy.Spec.File.MatchPaths = append(policy.Spec.File.MatchPaths, pathRule)
		}
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
		policy.Metadata["name"] = ms.Name
	} else {
		policy.Metadata["name"] = "ksp-" + mkPathFromTag(img.RepoTags[0])
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
	policy.Spec.Selector.MatchLabels["kubearmor.io/container.name"] = repotag[0]

	addPolicyRule(&policy, ms.Rules)
	return policy, nil
}

func (img *ImageInfo) checkPreconditions(ms MatchSpec) bool {
	matches := checkForSpec(filepath.Join(ms.Precondition), img.FileList)
	if len(matches) <= 0 {
		return false
	}
	return true
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

		poldir := fmt.Sprintf("%s/%s", options.Outdir, mkPathFromTag(img.RepoTags[0]))
		_ = os.Mkdir(poldir, 0750)

		outfile := fmt.Sprintf("%s/%s.yaml", poldir, ms.Name)
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
