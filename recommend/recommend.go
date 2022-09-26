// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	log "github.com/sirupsen/logrus"
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

	yamlFile, _ := os.ReadFile(filepath.Join(getCachePath(), "rules.yaml"))
	CurrentVersion = strings.Trim(updateRulesYAML(yamlFile), "\"")
	if !isLatest() {
		log.Warn("\033[1;33mpolicy-templates ", LatestVersion, " is available. Use `karmor recommend update` to get recommendations based on the latest policy-templates.\033[0m")
	}

	if err = createOutDir(o.OutDir); err != nil {
		return err
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
		err = imageHandler(dp.Namespace, dp.Name, dp.Labels, img)
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
