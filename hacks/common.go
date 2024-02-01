// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package hacks close the file
package hacks

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CloseCheckErr close file
func CloseCheckErr(f *os.File, fname string) {
	err := f.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"file": fname,
		}).Error("close file failed")
	}
}

// GetImageDetails gets image details
func GetImageDetails(image string) (registry string, name string, tag string, hash string) {
	tag = "latest"
	name = image
	registry = "docker.io"
	hashExist := strings.Index(image, "@")

	if hashExist != -1 {
		tag = ""
		hash = image[hashExist+1:]
		name = image[:hashExist]

	} else {
		versionExist := -1
		for i := len(image) - 1; i >= 0; i-- {
			if image[i] == ':' {
				versionExist = i
				break
			}
			if image[i] == '/' {
				break
			}
		}

		if versionExist != -1 {
			tag = image[versionExist+1:]
			name = image[:versionExist]
		}
	}

	if strings.ContainsAny(name, ".:") || strings.HasPrefix(name, "localhost/") {
		tmp := strings.Split(name, "/")
		name = strings.Join(tmp[1:], "/")
		registry = tmp[0]
	}
	return registry, name, tag, hash
}
