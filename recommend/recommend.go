// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Options for karmor recommend
type Options struct {
	Images     []string
	Labels     []string
	Tags       []string
	Namespace  string
	OutDir     string
	ReportFile string
	Config     string
	Merge      bool
}

// LabelMap is an alias for map[string]string
type LabelMap = map[string]string

// Deployment contains brief information about a k8s deployment
type Deployment struct {
	Name      string
	Namespace string
	Labels    LabelMap
	Images    []string
}

// ======================== //
// == Knox System Policy == //
// ======================== //

// KnoxFromSource Structure
type KnoxFromSource struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	Dir  string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

// KnoxMatchPaths Structure
type KnoxMatchPaths struct {
	Path       string           `json:"path,omitempty" yaml:"path,omitempty"`
	ReadOnly   bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	OwnerOnly  bool             `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []KnoxFromSource `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`
	Action     string           `json:"action,omitempty" yaml:"action,omitempty"`
	Severity   int              `json:"severity,omitempty" yaml:"severity,omitempty"`
	Message    string           `json:"message,omitempty" yaml:"message,omitempty"`
	Tags       []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// KnoxMatchDirectories Structure
type KnoxMatchDirectories struct {
	Dir        string           `json:"dir,omitempty" yaml:"dir,omitempty"`
	Recursive  bool             `json:"recursive,omitempty" yaml:"recursive,omitempty"`
	ReadOnly   bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	OwnerOnly  bool             `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []KnoxFromSource `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`
	Action     string           `json:"action,omitempty" yaml:"action,omitempty"`
	Severity   int              `json:"severity,omitempty" yaml:"severity,omitempty"`
	Message    string           `json:"message,omitempty" yaml:"message,omitempty"`
	Tags       []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// KnoxMatchProtocols Structure
type KnoxMatchProtocols struct {
	Protocol   string           `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	FromSource []KnoxFromSource `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`
}

// Selector Structure
type Selector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty" yaml:"matchLabels,omitempty" bson:"matchLabels,omitempty"`
}

// KnoxSys Structure
type KnoxSys struct {
	MatchPaths       []KnoxMatchPaths       `json:"matchPaths,omitempty" yaml:"matchPaths,omitempty"`
	MatchDirectories []KnoxMatchDirectories `json:"matchDirectories,omitempty" yaml:"matchDirectories,omitempty"`
}

// NetworkRule Structure
type NetworkRule struct {
	MatchProtocols []KnoxMatchProtocols `json:"matchProtocols,omitempty" yaml:"matchProtocols,omitempty"`
}

// KnoxSystemSpec Structure
type KnoxSystemSpec struct {
	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`

	Selector Selector `json:"selector,omitempty" yaml:"selector,omitempty"`

	Process KnoxSys     `json:"process,omitempty" yaml:"process,omitempty"`
	File    KnoxSys     `json:"file,omitempty" yaml:"file,omitempty"`
	Network NetworkRule `json:"network,omitempty" yaml:"network,omitempty"`

	Action string `json:"action,omitempty" yaml:"action,omitempty"`
}

// KnoxSystemPolicy Structure
type KnoxSystemPolicy struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty" bson:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty" bson:"kind,omitempty"`
	// LogIDs     []int             `json:"log_ids,omitempty" yaml:"log_ids,omitempty" bson:"log_ids,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty" bson:"metadata,omitempty"`
	Outdated string            `json:"outdated,omitempty" yaml:"outdated,omitempty" bson:"outdated,omitempty"`

	Spec KnoxSystemSpec `json:"spec,omitempty" yaml:"spec,omitempty" bson:"spec,omitempty"`

	GeneratedTime int64 `json:"generatedTime,omitempty" yaml:"generatedTime,omitempty" bson:"generatedTime,omitempty"`
	UpdatedTime   int64 `json:"updatedTime,omitempty" yaml:"updatedTime,omitempty" bson:"updatedTime,omitempty"`
	Latest        bool  `json:"latest,omitempty" yaml:"latest,omitempty" bson:"latest,omitempty"`
}

// ============================= //
// == KubeArmor System Policy == //
// ============================= //

// KubeArmorPolicy Structure
type KubeArmorPolicy struct {
	APIVersion string            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec KnoxSystemSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

var options Options

func unique(s []string) []string {
	inResult := make(map[string]bool)
	var result []string
	for _, str := range s {
		str = strings.Trim(str, " ")
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

func createOutDir(dir string) error {
	if dir == "" {
		return nil
	}
	_, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(dir, 0750)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func finalReport() {
	repFile := filepath.Clean(filepath.Join(options.OutDir, options.ReportFile))
	_ = ReportRender(repFile)
	color.Green("output report in %s ...", repFile)
	if strings.Contains(repFile, ".html") {
		return
	}
	data, err := os.ReadFile(repFile)
	if err != nil {
		log.WithError(err).Fatal("failed to read report file")
		return
	}
	fmt.Println(string(data))
}

// Recommend handler for karmor cli tool
func Recommend(c *k8s.Client, o Options) error {
	deployments := []Deployment{}
	var err error
	if !isLatest() {
		log.WithFields(log.Fields{
			"Current Version": CurrentVersion,
		}).Info("Found outdated version of policy-templates")
		log.Info("Downloading latest version [", LatestVersion, "]")
		if _, err := DownloadAndUnzipRelease(); err != nil {
			return err
		}
		log.WithFields(log.Fields{
			"Updated Version": CurrentVersion,
		}).Info("policy-templates updated")
	}

	if err = createOutDir(o.OutDir); err != nil {
		return err
	}

	if o.Merge {
		mergeSysPolicies([]KubeArmorPolicy{})
	}

	if o.ReportFile != "" {
		ReportInit(o.ReportFile)
	}

	labelMap := labelArrayToLabelMap(o.Labels)

	if len(o.Images) == 0 {
		// recommendation based on k8s manifest
		dps, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return err
		}
		for _, dp := range dps.Items {

			if matchLabels(labelMap, dp.Spec.Template.Labels) {
				images := []string{}
				for _, container := range dp.Spec.Template.Spec.Containers {
					images = append(images, container.Image)
				}

				deployments = append(deployments, Deployment{
					Name:      dp.Name,
					Namespace: dp.Namespace,
					Labels:    dp.Spec.Template.Labels,
					Images:    images,
				})
			}
		}
	} else {
		deployments = append(deployments, Deployment{
			Namespace: o.Namespace,
			Labels:    labelMap,
			Images:    o.Images,
		})
	}

	// o.Images = unique(o.Images)
	o.Tags = unique(o.Tags)
	options = o

	for _, dp := range deployments {
		err := handleDeployment(dp)
		if err != nil {
			log.Error(err)
		}
	}

	finalReport()
	return nil
}

func handleDeployment(dp Deployment) error {

	var err error
	for _, img := range dp.Images {
		tempDir, err = os.MkdirTemp("", "karmor")
		if err != nil {
			log.WithError(err).Error("could not create temp dir")
		}
		err = imageHandler(dp.Namespace, dp.Name, dp.Labels, img, options.Config)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"image": img,
			}).Error("could not handle container image")
		}
		_ = os.RemoveAll(tempDir)
	}

	return nil
}

func matchLabels(filter, selector LabelMap) bool {
	match := true
	for k, v := range filter {
		if selector[k] != v {
			match = false
			break
		}
	}
	return match
}

func labelArrayToLabelMap(labels []string) LabelMap {
	labelMap := LabelMap{}
	for _, label := range labels {
		kvPair := strings.FieldsFunc(label, labelSplitter)
		if len(kvPair) != 2 {
			continue
		}
		labelMap[kvPair[0]] = kvPair[1]
	}
	return labelMap
}

func labelSplitter(r rune) bool {
	return r == ':' || r == '='
}

func cmpGenPathDir(p1 string, p1fs []KnoxFromSource, p2 string, p2fs []KnoxFromSource) bool {
	if len(p1fs) > 0 {
		for _, v := range p1fs {
			p1 = p1 + v.Path
		}
	}

	if len(p2fs) > 0 {
		for _, v := range p2fs {
			p2 = p2 + v.Path
		}
	}
	return p1 < p2
}

func cmpPaths(p1 KnoxMatchPaths, p2 KnoxMatchPaths) bool {
	return cmpGenPathDir(p1.Path, p1.FromSource, p2.Path, p2.FromSource)
}

func cmpProts(p1 KnoxMatchProtocols, p2 KnoxMatchProtocols) bool {
	return cmpGenPathDir(p1.Protocol, p1.FromSource, p2.Protocol, p2.FromSource)
}

func cmpDirs(p1 KnoxMatchDirectories, p2 KnoxMatchDirectories) bool {
	return cmpGenPathDir(p1.Dir, p1.FromSource, p2.Dir, p2.FromSource)
}

func hashInt(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func checkIfMetadataMatches(pin KubeArmorPolicy, hay []KubeArmorPolicy) int {
	for idx, v := range hay {
		if pin.Metadata["clusterName"] == v.Metadata["clusterName"] &&
			pin.Metadata["namespace"] == v.Metadata["namespace"] &&
			pin.Metadata["containername"] == v.Metadata["containername"] &&
			pin.Metadata["labels"] == v.Metadata["labels"] {
			return idx
		}
	}
	return -1
}

func mergeFromSource(pols []KubeArmorPolicy) []KubeArmorPolicy {
	var results []KubeArmorPolicy
	for _, pol := range pols {
		checked := false
	check:
		i := checkIfMetadataMatches(pol, results)
		if i < 0 {
			if checked {
				log.Error("assumptions went wrong. some policies wont work %+v", pol)
				continue
			}
			newpol := pol
			newpol.Spec.Process = KnoxSys{}
			newpol.Spec.File = KnoxSys{}
			newpol.Spec.Network = NetworkRule{}
			results = append(results, newpol)
			checked = true
			goto check
		}

		mergeFromSourceMatchPaths(pol.Spec.File.MatchPaths, &results[i].Spec.File.MatchPaths)
		mergeFromSourceMatchDirs(pol.Spec.File.MatchDirectories, &results[i].Spec.File.MatchDirectories)

		mergeFromSourceMatchPaths(pol.Spec.Process.MatchPaths, &results[i].Spec.Process.MatchPaths)
		mergeFromSourceMatchDirs(pol.Spec.Process.MatchDirectories, &results[i].Spec.Process.MatchDirectories)

		mergeFromSourceMatchProt(pol.Spec.Network.MatchProtocols, &results[i].Spec.Network.MatchProtocols)
	}
	return results
}

func mergeFromSourceMatchPaths(pmp []KnoxMatchPaths, mp *[]KnoxMatchPaths) {
	for _, pp := range pmp {
		match := false
		for i := range *mp {
			rp := &(*mp)[i]
			if pp.Path == (*rp).Path {
				(*rp).FromSource = append((*rp).FromSource, pp.FromSource...)
				//remove dups
				match = true
			}
			sortFromSource(&(*rp).FromSource)
		}
		if !match {
			*mp = append(*mp, pp)
		}
	}
}

func mergeFromSourceMatchDirs(pmp []KnoxMatchDirectories, mp *[]KnoxMatchDirectories) {
	for _, pp := range pmp {
		match := false
		for i := range *mp {
			rp := &(*mp)[i]
			if pp.Dir == (*rp).Dir {
				(*rp).FromSource = append((*rp).FromSource, pp.FromSource...)
				//remove dups
				match = true
			}
			sortFromSource(&(*rp).FromSource)
		}
		if !match {
			*mp = append(*mp, pp)
		}
	}
}

func mergeFromSourceMatchProt(pmp []KnoxMatchProtocols, mp *[]KnoxMatchProtocols) {
	for _, pp := range pmp {
		match := false
		for i := range *mp {
			rp := &(*mp)[i]
			if pp.Protocol == (*rp).Protocol {
				(*rp).FromSource = append((*rp).FromSource, pp.FromSource...)
				//remove dups
				match = true
			}
			sortFromSource(&(*rp).FromSource)
		}
		if !match {
			*mp = append(*mp, pp)
		}
	}
}

func sortFromSource(fs *[]KnoxFromSource) {
	if len(*fs) <= 1 {
		return
	}
	sort.Slice(*fs, func(x, y int) bool {
		return (*fs)[x].Path+(*fs)[x].Dir < (*fs)[y].Path+(*fs)[y].Dir
	})
}

func mergeSysPolicies(pols []KubeArmorPolicy) []KubeArmorPolicy {
	// Get a list of YAML files in a directory
	files, err := filepath.Glob("*/*/*.yaml")
	if err != nil {
		// handle error
		fmt.Printf("Error reading YAML file %s: %v\n", files, err)
	}

	// Read policy files and unmarshal into KubeArmorPolicy structs
	var policies []KubeArmorPolicy
	for _, filename := range files {
		policyYaml, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}

		var policy KubeArmorPolicy
		err = yaml.Unmarshal(policyYaml, &policy)
		if err != nil {
			fmt.Printf("error unmarshaling YAML in file %s: %v\n", filename, err)
			os.Exit(1)
		}

		policies = append(policies, policy)
	}

	// Merge policies into one policy
	mp := KubeArmorPolicy{
		APIVersion: policies[0].APIVersion,
		Kind:       policies[0].Kind,
		Metadata: map[string]string{
			"name":      "harden-system-policy",
			"namespace": "default",
		},
		Spec: KnoxSystemSpec{
			Selector: Selector{
				MatchLabels: map[string]string{
					"app": "app",
				},
			},
			Process: KnoxSys{
				MatchDirectories: []KnoxMatchDirectories{
					policies[0].Spec.Process.MatchDirectories[0],
					{
						Action:   policies[0].Spec.Action,
						Severity: policies[0].Spec.Severity,
						Message:  policies[0].Spec.Message,
						Tags:     policies[0].Spec.Tags,
					},
				},
				MatchPaths: []KnoxMatchPaths{
					policies[0].Spec.Process.MatchPaths[0],
					{
						Action:   policies[0].Spec.Action,
						Severity: policies[0].Spec.Severity,
						Message:  policies[0].Spec.Message,
						Tags:     policies[0].Spec.Tags,
					},
				},
			},

			File: KnoxSys{
				MatchDirectories: []KnoxMatchDirectories{
					policies[0].Spec.Process.MatchDirectories[0],
					{
						Action:   policies[0].Spec.Action,
						Severity: policies[0].Spec.Severity,
						Message:  policies[0].Spec.Message,
						Tags:     policies[0].Spec.Tags,
					},
				},
				MatchPaths: []KnoxMatchPaths{
					policies[0].Spec.Process.MatchPaths[0],
					{
						Action:   policies[0].Spec.Action,
						Severity: policies[0].Spec.Severity,
						Message:  policies[0].Spec.Message,
						Tags:     policies[0].Spec.Tags,
					},
				},
			},
		},
	}

	for _, pol := range policies {
		pol.Metadata["name"] = "harden-system-policy" + strconv.FormatUint(uint64(hashInt(pol.Metadata["labels"]+pol.Metadata["namespace"]+pol.Metadata["clustername"]+pol.Metadata["containername"])), 10)
		i := checkIfMetadataMatches(pol, policies)
		if i < 0 {
			policies = append(policies, pol)
			continue
		}

		if len(pol.Spec.File.MatchPaths) > 0 {
			mp := &policies[i].Spec.File.MatchPaths
			*mp = append(*mp, pol.Spec.File.MatchPaths...)
		}
		if len(pol.Spec.File.MatchDirectories) > 0 {
			mp := &policies[i].Spec.File.MatchDirectories
			*mp = append(*mp, pol.Spec.File.MatchDirectories...)
		}
		if len(pol.Spec.Process.MatchPaths) > 0 {
			mp := &policies[i].Spec.Process.MatchPaths
			*mp = append(*mp, pol.Spec.Process.MatchPaths...)
		}
		if len(pol.Spec.Process.MatchDirectories) > 0 {
			mp := &policies[i].Spec.Process.MatchDirectories
			*mp = append(*mp, pol.Spec.Process.MatchDirectories...)
		}
		if len(pol.Spec.Network.MatchProtocols) > 0 {
			mp := &policies[i].Spec.Network.MatchProtocols
			*mp = append(*mp, pol.Spec.Network.MatchProtocols...)
		}
		policies[i].Metadata["name"] = pol.Metadata["name"]
	}

	policies = mergeFromSource(policies)

	// merging and sorting all the rules at MatchPaths, MatchDirs, MatchProtocols level
	// sorting is needed so that the rules are placed consistently in the
	// same order everytime the policy is generated
	for _, pol := range policies {
		if len(pol.Spec.File.MatchPaths) > 0 {
			mp := &pol.Spec.File.MatchPaths
			sort.Slice(*mp, func(x, y int) bool {
				return cmpPaths((*mp)[x], (*mp)[y])
			})
		}
		if len(pol.Spec.File.MatchDirectories) > 0 {
			mp := &pol.Spec.File.MatchDirectories
			sort.Slice(*mp, func(x, y int) bool {
				return cmpDirs((*mp)[x], (*mp)[y])
			})
		}
		if len(pol.Spec.Process.MatchPaths) > 0 {
			mp := &pol.Spec.Process.MatchPaths
			sort.Slice(*mp, func(x, y int) bool {
				return cmpPaths((*mp)[x], (*mp)[y])
			})
		}
		if len(pol.Spec.Process.MatchDirectories) > 0 {
			mp := &pol.Spec.Process.MatchDirectories
			sort.Slice(*mp, func(x, y int) bool {
				return cmpDirs((*mp)[x], (*mp)[y])
			})
		}
		if len(pol.Spec.Network.MatchProtocols) > 0 {
			mp := &pol.Spec.Network.MatchProtocols
			sort.Slice(*mp, func(x, y int) bool {
				return cmpProts((*mp)[x], (*mp)[y])
			})
		}
	}

	// Convert merged policy back into YAML
	mergedPolicyYaml, err := yaml.Marshal(mp)
	if err != nil {
		fmt.Printf("error marshaling merged policy: %v\n", err)
		os.Exit(1)
	}

	// Write merged policy YAML to file
	err = os.WriteFile("merged-policy.yaml", mergedPolicyYaml, 0644)
	if err != nil {
		fmt.Printf("error writing merged policy file: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Merged %d sys policies into %d policies", len(pols), len(policies))
	return policies
}
