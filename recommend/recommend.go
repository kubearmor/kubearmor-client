// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	log "github.com/sirupsen/logrus"
)

// Options for karmor recommend
type Options struct {
	Images       []string
	UseLabels    []string
	Tags         []string
	UseNamespace string
	OutDir       string
	ReportFile   string
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
	var err error

	if err = createOutDir(o.OutDir); err != nil {
		return err
	}

	if o.ReportFile != "" {
		ReportInit(o.ReportFile)
	}

	o.Images = unique(o.Images)
	o.Tags = unique(o.Tags)
	options = o
	for _, img := range o.Images {
		tempDir, err = os.MkdirTemp("", "karmor")
		if err != nil {
			log.WithError(err).Fatal("could not create temp dir")
		}
		err = imageHandler(img)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"image": img,
			}).Error("could not handle container image")
		}
		_ = os.RemoveAll(tempDir) // rm -rf tempDir
	}

	finalReport()
	return nil
}
