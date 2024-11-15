// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package genericpolicies is responsible for creating and managing policies based on policy generator
package genericpolicies

import (
	_ "embed" // need for embedding
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
	"github.com/kubearmor/kubearmor-client/recommend/report"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

const (
	org   = "kubearmor"
	repo  = "policy-templates"
	url   = "https://github.com/kubearmor/policy-templates/archive/refs/tags/"
	cache = ".cache/karmor/"
)

// GenericPolicy defines Policy Generators
type GenericPolicy struct{}

// Init initializing Policy Generator
func (P GenericPolicy) Init() error {
	if _, err := DownloadAndUnzipRelease(); err != nil {
		log.WithError(err).Error("could not download latest policy-templates version")
	} else {
		log.WithFields(log.Fields{
			"Updated Version": LatestVersion,
		}).Info("policy-templates updated")
	}
	return nil
}

// Scan image and generates policies
func (P GenericPolicy) Scan(img *image.Info, options common.Options) (map[string][]byte, map[string]interface{}, error) {
	var policyMap map[string][]byte
	var msMap map[string]interface{}
	var err error
	if policyMap, msMap, err = getPolicyFromImageInfo(img, options); err != nil {
		log.WithError(err).Error("policy generation from image info failed")
	}
	return policyMap, msMap, nil
}

func checkForSpec(spec string, fl []string) []string {
	var matches []string
	if !strings.HasSuffix(spec, "*") {
		spec = fmt.Sprintf("%s$", spec)
	}

	re := regexp.MustCompile(spec)
	for _, name := range fl {
		if re.Match([]byte(name)) {
			matches = append(matches, name)
		}
	}
	return matches
}

func matchTags(ms *common.MatchSpec, tags []string) bool {
	if len(tags) <= 0 {
		return true
	}
	for _, t := range tags {
		if slices.Contains(ms.Spec.Tags, t) {
			return true
		}
	}
	return false
}

func checkPreconditions(img *image.Info, ms *common.MatchSpec) bool {
	var matches []string
	for _, preCondition := range ms.Precondition {
		matches = append(matches, checkForSpec(filepath.Join(preCondition), img.FileList)...)
		if strings.Contains(preCondition, "OPTSCAN") {
			return true
		}
	}
	return len(matches) >= len(ms.Precondition)
}

func getPolicyFromImageInfo(img *image.Info, options common.Options) (map[string][]byte, map[string]interface{}, error) {
	var policy []byte
	var outFile string
	policyMap := map[string][]byte{}
	msMap := make(map[string]interface{})

	if img.OS != "linux" {
		log.Errorf("non-linux platforms are not supported, yet.")
		return nil, nil, nil
	}

	idx := 0

	if err := report.Start(img, options, CurrentVersion); err != nil {
		log.WithError(err).Error("report start failed")
		return nil, nil, err
	}
	var ms common.MatchSpec
	var err error

	ms, err = getNextRule(&idx)
	for ; err == nil; ms, err = getNextRule(&idx) {

		if !matchTags(&ms, options.Tags) {
			continue
		}

		if !checkPreconditions(img, &ms) {
			continue
		}
		policy, outFile = img.GetPolicy(ms, options)
		policyMap[outFile] = policy
		msMap[outFile] = ms
	}
	return policyMap, msMap, nil
}
