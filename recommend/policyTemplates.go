// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cavaliergopher/grab/v3"
	"github.com/google/go-github/github"
	"github.com/kubearmor/kubearmor-client/selfupdate"
	log "github.com/sirupsen/logrus"
)

const (
	org  = "vishnusomank"
	repo = "policy-templates"
	url  = "https://github.com/vishnusomank/policy-templates/archive/refs/tags/"
)

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

// isTemplates function returns error if policy-template folder not found
func isTemplates() error {
	path := fmt.Sprintf("%s/.cache/karmor/", userHome())
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		err := isLatest()
		if err != nil {
			if selfupdate.ConfirmUserAction("Outdated policy-templates detected. Do you want to update it?") {
				ver, err := downloadAndUnzipRelease()
				if err != nil {
					return err
				}
				log.WithFields(log.Fields{
					"Current Version": ver,
				}).Info("policy-templates update completed")
			}
		}
	} else {
		log.WithFields(log.Fields{
			"Current Version": "nil",
		}).Info("policy-templates not found. Trying to download")
		ver, err := downloadAndUnzipRelease()
		if err != nil {
			return err
		}
		log.WithFields(log.Fields{
			"Current Version": ver,
		}).Info("policy-templates download completed")
	}
	return nil
}

func latestRelease() (*github.RepositoryRelease, error) {
	latestRelease, _, err := github.NewClient(nil).Repositories.GetLatestRelease(context.Background(), org, repo)
	return latestRelease, err
}

func isLatest() error {
	latestRelease, err := latestRelease()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/.cache/karmor/", userHome())
	latestFolderName := fmt.Sprintf("%s%s-%s", path, repo, strings.TrimPrefix(*latestRelease.TagName, "v"))
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentFolderName := fmt.Sprintf("%s%s", path, file.Name())
		if file.IsDir() && currentFolderName != latestFolderName {
			return errors.New("policy-template version is outdate. Please use `karmor recommend --update` to update the policy-template to the latest version ")
		}
	}
	return nil
}

func removeData(file string) error {
	err := os.RemoveAll(file)
	return err
}

func downloadAndUnzipRelease() (string, error) {
	path := fmt.Sprintf("%s/.cache/karmor/", userHome())
	latestRelease, err := latestRelease()
	if err != nil {
		return "", err
	}
	_ = removeData(path)
	err = os.MkdirAll(filepath.Dir(path), 0750)
	if err != nil {
		return "", err
	}
	downloadURL := fmt.Sprintf("%s%s.zip", url, *latestRelease.TagName)
	resp, err := grab.Get(path, downloadURL)
	if err != nil {
		_ = removeData(path)
		return "", err
	}
	err = unZip(resp.Filename, path)
	if err != nil {
		return "", err
	}
	err = removeData(resp.Filename)
	if err != nil {
		return "", err
	}
	_ = updatePolicyRules(strings.TrimSuffix(resp.Filename, ".zip"))
	return *latestRelease.TagName, nil
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
	f, err := os.Create(filepath.Clean(fmt.Sprintf("%s/.cache/karmor/rules.yaml", userHome())))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", fmt.Sprintf("%s/.cache/karmor/rules.yaml", userHome())))
	}

	idx := 0
	for _, file := range files {
		yamlFile, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return err
		}
		updateRulesYAML(yamlFile)
		ms, err := getNextRule(&idx)
		tempYaml := yamlFile
		for ; err == nil; ms, err = getNextRule(&idx) {
			if ms.Yaml != "" {
				newYaml, err := os.ReadFile(filepath.Clean(fmt.Sprintf("%s%s", strings.TrimSuffix(file, "metadata.yaml"), ms.Yaml)))
				if err != nil {
					return err
				}
				skipCount := len(ms.Yaml) + 6
				tempYaml = tempYaml[strings.Index(string(tempYaml), "yaml:")+skipCount:]
				dataVal := strings.TrimSpace(string(tempYaml))
				yamlFile = yamlFile[:strings.Index(string(yamlFile), "yaml:")-1]
				for _, eachLine := range strings.Split(string(newYaml[strings.Index(string(newYaml), "spec:"):]), "\n") {
					yamlFile = append(yamlFile, []byte(fmt.Sprintf(" %s\n", eachLine))...)
				}
				yamlFile = []byte(strings.TrimSpace(string(yamlFile)))
				yamlFile = append(yamlFile, []byte(fmt.Sprintf("\n%s", dataVal))...)
			}
		}
		if _, err := f.WriteString(string(yamlFile)); err != nil {
			log.WithError(err).Error("WriteString failed")
		}
		if err := f.Sync(); err != nil {
			log.WithError(err).Error("file sync failed")
		}
	}
	if err := f.Close(); err != nil {
		log.WithError(err).Error("file close failed")
	}
	return nil
}
