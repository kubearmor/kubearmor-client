// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package report

import (
	"errors"
	"strings"

	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
)

/*
Init()
for every image {
	Start()
	for every policy {
		Record()
	}
	SectEnd()
}
Render()
*/

// Handler interface
var Handler interface{}

// Init called once per execution
func Init(fname string) {
	if Handler != nil {
		return
	}
	if strings.Contains(fname, ".html") {
		Handler = NewHTMLReport()
	} else {
		Handler = NewTextReport()
	}
}

// Start called once per container image at the start
func Start(img *image.Info, options common.Options, currentVersion string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Start(img, options.OutDir, currentVersion)
	case TextReport:
		return v.Start(img, options.OutDir, currentVersion)
	}
	return errors.New("unknown reporter type")
}

// Record called once per policy
func Record(in interface{}, policyName string) error {
	ms := in.(common.MatchSpec)
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Record(ms, policyName)
	case TextReport:
		return v.Record(ms, policyName)
	}
	return errors.New("unknown reporter type")
}

// SectEnd called once per container image at the end
func SectEnd() error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.SectionEnd()
	case TextReport:
		return v.SectionEnd()
	}
	return errors.New("unknown reporter type")
}

// Render called finaly to render the report
func Render(out string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Render(out)
	case TextReport:
		return v.Render(out)
	}
	return errors.New("unknown reporter type")
}
