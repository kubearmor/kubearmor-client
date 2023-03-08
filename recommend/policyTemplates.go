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
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
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
	cache := fmt.Sprintf("%s/%s", UserHome(), cache)
	return cache

}

// UserHome function returns users home directory
func UserHome() string {
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
func CurrentRelease() (string, error) {
	fileData, err := os.ReadFile(fmt.Sprintf("%s%s", getCachePath(), "rules.yaml"))
	if err != nil {
		return "", err
	} else {
		var version string
		version, err = updateRulesYAML(fileData, &policyRules)
		if err != nil {
			return "", err
		} else {
			CurrentVersion = strings.Trim(version, "\"")
		}
	}
	return CurrentVersion, nil
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
	CurrentVersion, _ = CurrentRelease()
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
	err = updatePolicyRules(strings.TrimSuffix(resp.Filename, ".zip"), &policyRules)
	if err != nil {
		return "", err
	}
	CurrentVersion, err = CurrentRelease()
	if err != nil {
		return "", err
	}
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
		defer func() {
			if err := open.Close(); err != nil {
				kg.Warnf("Error closing io stream %s\n", err)
			}
		}()
	}
	return nil
}

func updatePolicyRules(filePath string, policyRules *[]MatchSpec) error {
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
	var completePolicy []MatchSpec
	var version string

	for _, file := range files {
		idx := 0
		yamlFile, err = os.ReadFile(filepath.Clean(file))
		if err != nil {
			return err
		}
		version, err = updateRulesYAML(yamlFile, policyRules)
		if err != nil {
			return err
		}
		ms, err := getNextRule(&idx, *policyRules)
		for ; err == nil; ms, err = getNextRule(&idx, *policyRules) {
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
			completePolicy = append(completePolicy, ms)
		}
	}
	yamlFile, err = yaml.Marshal(completePolicy)
	if err != nil {
		return err
	}
	updateYamlRulesFile(version, yamlFile, f)
	return nil
}

func updateYamlRulesFile(version string, yamlFile []byte, f *os.File) {
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
}
