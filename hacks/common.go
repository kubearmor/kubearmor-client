// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package hacks close the file
package hacks

import (
	"os"

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
