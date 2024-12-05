// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package genericpolicies

import (
	"archive/zip"
	"context"
	_ "embed" // need for embedding
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/clarketm/json"

	"github.com/google/go-github/github"
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
	"github.com/kubearmor/kubearmor-client/recommend/common"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// CurrentVersion stores the current version of policy-template
var CurrentVersion string

// LatestVersion stores the latest version of policy-template
var LatestVersion string

func isLatest() bool {
	LatestVersion = latestRelease()
	CurrentVersion = CurrentRelease()
	if LatestVersion == "" {
		// error while fetching latest release tag
		// assume the current release is the latest one
		return true
	}
	return (CurrentVersion == LatestVersion)
}

func latestRelease() string {
	latestRelease, _, err := github.NewClient(nil).Repositories.GetLatestRelease(context.Background(), org, repo)
	if err != nil {
		log.WithError(err)
		return ""
	}
	return *latestRelease.TagName
}

// CurrentRelease gets the current release of policy-templates
func CurrentRelease() string {
	CurrentVersion := ""
	path, err := os.ReadFile(fmt.Sprintf("%s%s", getCachePath(), "rules.yaml"))
	if err != nil {
		CurrentVersion = strings.Trim(updateRulesYAML([]byte{}), "\"")
	} else {
		CurrentVersion = strings.Trim(updateRulesYAML(path), "\"")
	}
	return CurrentVersion
}

func getCachePath() string {
	cache := fmt.Sprintf("%s/%s", common.UserHome(), cache)
	return cache
}

//go:embed yaml/rules.yaml

var policyRulesYAML []byte

var policyRules []common.MatchSpec

func updateRulesYAML(yamlFile []byte) string {
	policyRules = []common.MatchSpec{}
	if len(yamlFile) < 30 {
		yamlFile = policyRulesYAML
	}
	policyRulesJSON, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		log.WithError(err).Fatal("failed to convert policy rules yaml to json")
	}
	var jsonRaw map[string]json.RawMessage
	err = json.Unmarshal(policyRulesJSON, &jsonRaw)
	if err != nil {
		log.WithError(err).Fatal("failed to unmarshal policy rules json")
	}
	err = json.Unmarshal(jsonRaw["policyRules"], &policyRules)
	if err != nil {
		log.WithError(err).Fatal("failed to unmarshal policy rules")
	}
	return string(jsonRaw["version"])
}

func removeData(file string) error {
	err := os.RemoveAll(file)
	return err
}

func downloadZip(url string, destination string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	out, err := os.Create(filepath.Clean(destination))
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			kg.Warnf("Error closing os file %s\n", err)
		}
	}()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// DownloadAndUnzipRelease downloads the latest version of policy-templates
func DownloadAndUnzipRelease() (string, error) {
	latestVersion := latestRelease()
	currentVersion := CurrentRelease()

	if isLatest() {
		return latestVersion, nil
	}

	log.WithFields(log.Fields{
		"Current Version": currentVersion,
	}).Info("Found outdated version of policy-templates")
	log.Info("Downloading latest version [", latestVersion, "]")

	err := removeData(getCachePath())
	if err != nil {
		log.WithError(err).Error("failed to remove cache files")
	}
	err = os.MkdirAll(filepath.Dir(getCachePath()), 0o750)
	if err != nil {
		return "", err
	}

	downloadURL := fmt.Sprintf("%s%s.zip", url, latestVersion)
	zipPath := getCachePath() + ".zip"
	err = downloadZip(downloadURL, zipPath)
	if err != nil {
		err = removeData(getCachePath())
		if err != nil {
			log.WithError(err).Error("failed to remove cache files")
		}
		return "", err
	}

	err = unZip(zipPath, getCachePath())
	if err != nil {
		return "", err
	}
	err = removeData(zipPath)
	if err != nil {
		log.WithError(err).Error("failed to remove cache files")
	}
	err = updatePolicyRules(strings.TrimSuffix(zipPath, ".zip"))
	if err != nil {
		log.WithError(err).Error("failed to update policy rules")
	}
	return latestVersion, nil
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}

func unZip(source, dest string) error {
	read, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer read.Close()
	for _, file := range read.File {
		if file.Mode().IsDir() {
			continue
		}
		open, err := file.Open()
		if err != nil {
			return err
		}
		name, err := sanitizeArchivePath(dest, file.Name)
		if err != nil {
			return err
		}
		err = os.MkdirAll(path.Dir(name), 0o750)
		if err != nil {
			log.WithError(err).Error("failed to create directory")
		}
		create, err := os.Create(filepath.Clean(name))
		if err != nil {
			return err
		}
		_, err = create.ReadFrom(open)
		if err != nil {
			return err
		}
		if err = create.Close(); err != nil {
			return err
		}
		defer func() {
			if err := open.Close(); err != nil {
				kg.Warnf("Error closing io stream %s\n", err)
			}
		}()
	}
	return nil
}

func getNextRule(idx *int) (common.MatchSpec, error) {
	if *idx < 0 {
		(*idx)++
	}
	if *idx >= len(policyRules) {
		return common.MatchSpec{}, errors.New("no rule at idx")
	}
	r := policyRules[*idx]
	(*idx)++
	return r, nil
}

func updatePolicyRules(filePath string) error {
	var files []string
	err := filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "metadata.yaml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	rulesYamlPath := filepath.Join(getCachePath(), "rules.yaml")
	f, err := os.Create(filepath.Clean(rulesYamlPath))
	if err != nil {
		log.WithError(err).Errorf("Failed to create %s", rulesYamlPath)
	}

	var yamlFile []byte
	var completePolicy []common.MatchSpec
	var version string

	for _, file := range files {
		idx := 0
		yamlFile, err = os.ReadFile(filepath.Clean(file))
		if err != nil {
			return err
		}
		version = updateRulesYAML(yamlFile)
		ms, err := getNextRule(&idx)
		for ; err == nil; ms, err = getNextRule(&idx) {
			if ms.Yaml != "" {
				var policy map[string]interface{}
				newYaml, err := os.ReadFile(filepath.Clean(fmt.Sprintf("%s%s", strings.TrimSuffix(file, "metadata.yaml"), ms.Yaml)))
				if err != nil {
					newYaml, _ = os.ReadFile(filepath.Clean(fmt.Sprintf("%s/%s", filePath, ms.Yaml)))
				}
				err = yaml.Unmarshal(newYaml, &policy)
				if err != nil {
					return err
				}
				apiVersion := policy["apiVersion"].(string)
				if strings.Contains(apiVersion, "kubearmor") {
					var kubeArmorPolicy pol.KubeArmorPolicy
					err = yaml.Unmarshal(newYaml, &kubeArmorPolicy)
					if err != nil {
						return err
					}
					ms.Spec = kubeArmorPolicy.Spec
				} else {
					continue
				}
				ms.Yaml = ""
			}
			completePolicy = append(completePolicy, ms)
		}
	}
	policyRules = completePolicy
	yamlFile, err = yaml.Marshal(completePolicy)
	if err != nil {
		return err
	}
	version = strings.Trim(version, "\"")
	yamlFile = []byte(fmt.Sprintf("version: %s\npolicyRules:\n%s", version, yamlFile))
	if _, err := f.WriteString(string(yamlFile)); err != nil {
		log.WithError(err).Error("WriteString failed")
	}
	if err := f.Sync(); err != nil {
		log.WithError(err).Error("file sync failed")
	}
	if err := f.Close(); err != nil {
		log.WithError(err).Error("file close failed")
	}

	return nil
}
