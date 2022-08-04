// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/accuknox/auto-policy-discovery/src/types"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
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
				Recursive:  true,
				FromSource: fromSourceArr,
			}
			policy.Spec.File.MatchDirectories = append(policy.Spec.File.MatchDirectories, dirRule)
		}
	}
}

func createPolicy(img *ImageInfo, ms MatchSpec) (types.KubeArmorPolicy, error) {
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
		r := strings.NewReplacer(
			"/", "-",
			":", "-",
			"\\", "-",
		)
		policy.Metadata["name"] = "ksp-" + r.Replace(img.RepoTags[0])
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
	policy.Spec.Selector.MatchLabels["container"] = repotag[0]

	addPolicyRule(&policy, ms.Rules)
	return policy, nil
}

func getPolicyFromImageInfo(img *ImageInfo) {
	if img.OS != "linux" {
		color.Red("non-linux platforms are not supported, yet.")
		return
	}
	isFirst := true
	polFile, err := os.Create(options.Outfile)
	defer closeCheckErr(polFile, options.Outfile)
	idx := 0
	ms, err := getNextRule(&idx)
	for ; err == nil; ms, err = getNextRule(&idx) {
		log.WithFields(log.Fields{
			"spec": ms,
			"idx":  idx,
		}).Info("processing spec")
		matches := checkForSpec(filepath.Join(ms.Precondition), img.FileList)
		if len(matches) <= 0 {
			continue
		}

		policy, err := createPolicy(img, ms)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"image": img,
				"spec":  ms,
			}).Error("create policy failed, skipping")
			continue
		}

		arr, _ := json.Marshal(policy)
		yamlarr, _ := yaml.JSONToYAML(arr)
		if !isFirst {
			_, _ = polFile.WriteString("---\n")
		}
		if _, err := polFile.WriteString(string(yamlarr)); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"file": options.Outfile,
			}).Error("WriteString failed")
		}
		isFirst = false
	}
	if err := polFile.Sync(); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"file": options.Outfile,
		}).Error("file sync failed")
	}
}
