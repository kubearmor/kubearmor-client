// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package image scan and provide image info
package image

import (
	_ "embed" // need for embedding
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clarketm/json"

	"github.com/fatih/color"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
	"github.com/kubearmor/kubearmor-client/hacks"
	"github.com/kubearmor/kubearmor-client/recommend/common"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

type distroRule struct {
	Name  string `json:"name" yaml:"name"`
	Match []struct {
		Path string `json:"path" yaml:"path"`
	} `json:"match" yaml:"match"`
}

//go:embed yaml/distro.yaml
var distroYAML []byte

var distroRules []distroRule

// Info contains image information
type Info struct {
	Name       string
	Namespace  string
	Labels     LabelMap
	Deployment string
	Image      string

	RepoTags []string
	Arch     string
	Distro   string
	OS       string
	FileList []string
	DirList  []string

	TempDir string
}

// LabelMap is an alias for map[string]string
type LabelMap = map[string]string

func init() {
	distroJSON, err := yaml.YAMLToJSON(distroYAML)
	if err != nil {
		color.Red("failed to convert distro rules yaml to json")
		log.WithError(err).Fatal("failed to convert distro rules yaml to json")
	}

	var jsonRaw map[string]json.RawMessage
	err = json.Unmarshal(distroJSON, &jsonRaw)
	if err != nil {
		color.Red("failed to unmarshal distro rules json")
		log.WithError(err).Fatal("failed to unmarshal distro rules json")
	}

	err = json.Unmarshal(jsonRaw["distroRules"], &distroRules)
	if err != nil {
		color.Red("failed to unmarshal distro rules")
		log.WithError(err).Fatal("failed to unmarshal distro rules")
	}
}

// GetImageInfo fetches information about the image and reads its manifest
func (img *Info) GetImageInfo() {
	matches := checkForSpec(filepath.Join(img.TempDir, "manifest.json"), img.FileList)
	if len(matches) != 1 {
		log.WithFields(log.Fields{
			"len":     len(matches),
			"matches": matches,
		}).Fatal("expecting one manifest.json!")
	}
	img.readManifest(matches[0])

	img.GetDistro()
}

// GetDistro identifies the distribution of the image
func (img *Info) GetDistro() {
	for _, d := range distroRules {
		match := true
		for _, m := range d.Match {
			matches := checkForSpec(filepath.Clean(img.TempDir+m.Path), img.FileList)
			if len(matches) == 0 {
				match = false
				break
			}
		}
		if len(d.Match) > 0 && match {
			color.Green("Distribution %s", d.Name)
			img.Distro = d.Name
			return
		}
	}
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

func (img *Info) readManifest(manifest string) {
	// read manifest file
	barr, err := getFileBytes(manifest)
	if err != nil {
		log.WithError(err).Fatal("manifest read failed")
	}
	var manres []map[string]interface{}
	err = json.Unmarshal(barr, &manres)
	if err != nil {
		log.WithError(err).Fatal("manifest json unmarshal failed")
	}
	if len(manres) < 1 {
		log.WithFields(log.Fields{
			"len":     len(manres),
			"results": manres,
		}).Fatal("expecting atleast one config in manifest!")
	}

	var man map[string]interface{}
	for _, man = range manres {
		if man["RepoTags"] != nil {
			break
		}
	}

	// read config file
	config := filepath.Join(img.TempDir, man["Config"].(string))
	barr, err = getFileBytes(config)
	if err != nil {
		log.WithFields(log.Fields{
			"config": config,
		}).Fatal("config read failed")
	}
	var cfgres map[string]interface{}
	err = json.Unmarshal(barr, &cfgres)
	if err != nil {
		log.WithError(err).Fatal("config json unmarshal failed")
	}
	img.Arch = cfgres["architecture"].(string)
	img.OS = cfgres["os"].(string)

	if man["RepoTags"] == nil {
		// If the image name contains sha256 digest,
		// then manifest["RepoTags"] will be `nil`.
		img.RepoTags = append(img.RepoTags, shortenImageNameWithSha256(img.Name))
	} else {
		for _, tag := range man["RepoTags"].([]interface{}) {
			img.RepoTags = append(img.RepoTags, tag.(string))
		}
	}
}

// shortenImageNameWithSha256 truncates the sha256 digest in image name
func shortenImageNameWithSha256(name string) string {
	if strings.Contains(name, "@sha256:") {
		// shorten sha256 to first 8 chars
		return name[:len(name)-56]
	}
	return name
}

func getFileBytes(fname string) ([]byte, error) {
	f, err := os.Open(filepath.Clean(fname))
	if err != nil {
		log.WithFields(log.Fields{
			"file": fname,
		}).Fatal("open file failed")
	}
	defer hacks.CloseCheckErr(f, fname)
	return io.ReadAll(f)
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

func (img *Info) getPolicyName(spec string) string {
	var policyName string

	if img.Deployment == "" {
		// policy recommendation for container images
		policyName = fmt.Sprintf("%s-%s", mkPathFromTag(img.RepoTags[0]), spec)
	} else {
		// policy recommendation based on k8s manifest
		policyName = fmt.Sprintf("%s-%s-%s", img.Deployment, mkPathFromTag(img.RepoTags[0]), spec)
	}
	return policyName
}

// GetPolicyDir generates a policy directory path based on the image information
func (img *Info) GetPolicyDir(outDir string) string {
	var policyDir string

	if img.Deployment == "" {
		// policy recommendation for container images
		if img.Namespace == "" {
			policyDir = mkPathFromTag(img.RepoTags[0])
		} else {
			policyDir = fmt.Sprintf("%s-%s", img.Namespace, mkPathFromTag(img.RepoTags[0]))
		}
	} else {
		// policy recommendation based on k8s manifest
		policyDir = fmt.Sprintf("%s-%s", img.Namespace, img.Deployment)
	}
	return filepath.Join(outDir, policyDir)
}

func (img *Info) getPolicyFile(spec string, outDir string) string {
	var policyFile string

	if img.Deployment != "" {
		// policy recommendation based on k8s manifest
		policyFile = fmt.Sprintf("%s-%s.yaml", mkPathFromTag(img.RepoTags[0]), spec)
	} else {
		policyFile = fmt.Sprintf("%s.yaml", spec)
	}

	return filepath.Join(img.GetPolicyDir(outDir), policyFile)
}

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

func (img *Info) createPolicy(ms common.MatchSpec) (pol.KubeArmorPolicy, error) {
	policy := pol.KubeArmorPolicy{
		Spec: pol.KubeArmorPolicySpec{
			Severity: 1, // by default
			Selector: pol.SelectorType{
				MatchLabels: map[string]string{},
			},
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

// GetPolicy - creates policy and return back
func (img *Info) GetPolicy(ms common.MatchSpec, options common.Options) ([]byte, string) {
	policy, err := img.createPolicy(ms)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"image": img, "spec": ms,
		}).Error("create policy failed, skipping")
	}

	arr, _ := json.Marshal(policy)
	outFile := img.getPolicyFile(ms.Name, options.OutDir)
	err = os.MkdirAll(filepath.Dir(outFile), 0o750)
	if err != nil {
		log.WithError(err).Error("failed to create directory")
	}
	_, err = os.Create(filepath.Clean(outFile))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", outFile))
	}

	return arr, outFile
}
