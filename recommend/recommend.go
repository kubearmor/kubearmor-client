// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"os"

	"github.com/kubearmor/kubearmor-client/k8s"
	log "github.com/sirupsen/logrus"
)

// Options for karmor recommend
type Options struct {
	Images  []string
	Outfile string
}

var options Options

func unique(s []string) []string {
	inResult := make(map[string]bool)
	var result []string
	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

// Recommend handler for karmor cli tool
func Recommend(c *k8s.Client, o Options) error {
	var err error
	options = o
	tempDir, err = os.MkdirTemp("", "karmor")
	if err != nil {
		log.WithError(err).Fatal("could not create temp dir")
	}
	defer os.RemoveAll(tempDir) // rm -rf tempDir

	o.Images = unique(o.Images)
	for _, img := range o.Images {
		err = imageHandler(img)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"image": img,
			}).Error("could not handle container image")
		}
	}
	return nil
}
