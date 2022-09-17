// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clarketm/json"
	"github.com/fatih/color"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/yaml"
)

func addPolicyRule(policy *pol.KubeArmorPolicy, r pol.KubeArmorPolicySpec) {

	if len(r.File.MatchDirectories) != 0 || len(r.File.MatchPaths) != 0 {
		policy.Spec.File = r.File
	}
	if len(r.Process.MatchDirectories) != 0 || len(r.Process.MatchPaths) != 0 {
		policy.Spec.Process = r.Process
	}
	if len(r.Network.MatchProtocols) != 0 {
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

func (img *ImageInfo) createPolicy(ms MatchSpec) (pol.KubeArmorPolicy, error) {
	policy := pol.KubeArmorPolicy{
		Spec: pol.KubeArmorPolicySpec{
			Severity: 1, // by default
			Selector: pol.SelectorType{
				MatchLabels: map[string]string{}},
		},
	}
	policy.APIVersion = "security.kubearmor.com/v1"
	policy.Kind = "KubeArmorPolicy"

	policy.ObjectMeta.Name = img.getPolicyName(ms.Name)

	if img.Namespace != "" {
		policy.ObjectMeta.Namespace = img.Namespace
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
	var matches []string
	for _, preCondition := range ms.Precondition {
		matches = append(matches, checkForSpec(filepath.Join(preCondition), img.FileList)...)
	}
	return len(matches) >= len(ms.Precondition)
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

func (img *ImageInfo) writePolicyFile(ms MatchSpec) {
	policy, err := img.createPolicy(ms)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"image": img, "spec": ms,
		}).Error("create policy failed, skipping")

	}

	outFile := img.getPolicyFile(ms.Name)
	_ = os.MkdirAll(filepath.Dir(outFile), 0750)

	f, err := os.Create(filepath.Clean(outFile))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", outFile))

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
	var ms MatchSpec
	var err error

	err = createRuntimePolicy(img)
	if err != nil {
		log.Infof("Failed to create runtime policy for %s/%s/%s. err=%s", img.Namespace, img.Deployment, img.Name, err)
	}

	ms, err = getNextRule(&idx)
	for ; err == nil; ms, err = getNextRule(&idx) {
		// matches preconditions

		if !matchTags(ms) {
			continue
		}

		if !img.checkPreconditions(ms) {
			continue
		}
		img.writePolicyFile(ms)
	}

	_ = ReportSectEnd(img)
}
