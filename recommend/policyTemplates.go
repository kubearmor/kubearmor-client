// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cavaliergopher/grab/v3"
	"github.com/google/go-github/github"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

const (
	org   = "kubearmor"
	repo  = "policy-templates"
	url   = "https://github.com/kubearmor/policy-templates/archive/refs/tags/"
	cache = ".cache/karmor/"
)

// CurrentVersion stores the current version of policy-template
var CurrentVersion string

// LatestVersion stores the latest version of policy-template
var LatestVersion string

func getCachePath() string {
	cache := fmt.Sprintf("%s/%s", userHome(), cache)
	return cache

}

// userHome function returns users home directory
func userHome() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
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

	path, err := os.ReadFile(fmt.Sprintf("%s%s", getCachePath(), "rules.yaml"))
	if err != nil {
		CurrentVersion = strings.Trim(updateRulesYAML([]byte{}), "\"")
	} else {

		CurrentVersion = strings.Trim(updateRulesYAML(path), "\"")
	}

	return CurrentVersion
}

func isLatest() bool {
	LatestVersion = latestRelease()

	if LatestVersion == "" {
		// error while fetching latest release tag
		// assume the current release is the latest one
		return true
	}
	return (CurrentVersion == LatestVersion)
}

func removeData(file string) error {
	err := os.RemoveAll(file)
	return err
}

func init() {
	CurrentVersion = CurrentRelease()
}

// DownloadAndUnzipRelease downloads the latest version of policy-templates
func DownloadAndUnzipRelease() (string, error) {

	LatestVersion = latestRelease()

	_ = removeData(getCachePath())
	err := os.MkdirAll(filepath.Dir(getCachePath()), 0750)
	if err != nil {
		return "", err
	}
	downloadURL := fmt.Sprintf("%s%s.zip", url, LatestVersion)
	resp, err := grab.Get(getCachePath(), downloadURL)
	if err != nil {
		_ = removeData(getCachePath())
		return "", err
	}
	err = unZip(resp.Filename, getCachePath())
	if err != nil {
		return "", err
	}
	err = removeData(resp.Filename)
	if err != nil {
		return "", err
	}
	_ = updatePolicyRules(strings.TrimSuffix(resp.Filename, ".zip"))
	CurrentVersion = CurrentRelease()
	return LatestVersion, nil
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
		_ = os.MkdirAll(path.Dir(name), 0750)
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
		defer open.Close()
	}
	return nil
}

func updatePolicyRules(filePath string) error {
	var files, tagsFile []string
	err := filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "metadata.yaml" {
			files = append(files, path)
		}
		if !info.IsDir() && info.Name() == "tags.yaml" {
			tagsFile = append(tagsFile, path)
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
	var completePolicy []MatchSpec
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
				newPolicyFile := pol.KubeArmorPolicy{}
				newYaml, err := os.ReadFile(filepath.Clean(fmt.Sprintf("%s%s", strings.TrimSuffix(file, "metadata.yaml"), ms.Yaml)))
				if err != nil {
					newYaml, _ = os.ReadFile(filepath.Clean(fmt.Sprintf("%s/%s", filePath, ms.Yaml)))
				}
				err = yaml.Unmarshal(newYaml, &newPolicyFile)
				if err != nil {
					return err
				}
				ms.Yaml = ""
				ms.Spec = newPolicyFile.Spec
			}
			newTags := []string{}
			for _, tagFile := range tagsFile {
				tagsData, err := os.ReadFile(filepath.Clean(tagFile))
				if err != nil {
					return err
				}
				tagData := updateTagsData(tagsData)
				for _, tags := range tagData {
					for _, nextTags := range ms.Spec.Tags {
						if tags == nextTags {
							newTags = append(newTags, tags)
						}
					}
				}
			}
			ms.Spec.Tags = newTags
			completePolicy = append(completePolicy, ms)
		}
	}
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
