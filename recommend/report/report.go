// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package report

import (
	"errors"
	"strings"

	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
)

/*
ReportInit()
for every image {
	ReportStart()
	for every policy {
		ReportRecord()
	}
	ReportSectEnd()
}
ReportRender()
*/

// Handler interface
var Handler interface{}

// ReportInit called once per execution
func ReportInit(fname string) {
	if Handler != nil {
		return
	}
	if strings.Contains(fname, ".html") {
		Handler = NewHTMLReport()
	} else {
		Handler = NewTextReport()
	}
}

// ReportStart called once per container image at the start
func ReportStart(img *image.ImageInfo, options common.Options, currentVersion string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Start(img, options.OutDir, currentVersion)
	case TextReport:
		return v.Start(img, options.OutDir, currentVersion)
	}
	return errors.New("unknown reporter type")
}

type Report struct{}

// ReportRecord called once per policy
func (r *Report) ReportRecord(ms common.MatchSpec, policyName string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Record(ms, policyName)
	case TextReport:
		return v.Record(ms, policyName)
	}
	return errors.New("unknown reporter type")
}

// ReportSectEnd called once per container image at the end
func ReportSectEnd() error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.SectionEnd()
	case TextReport:
		return v.SectionEnd()
	}
	return errors.New("unknown reporter type")
}

// ReportRender called finaly to render the report
func ReportRender(out string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Render(out)
	case TextReport:
		return v.Render(out)
	}
	return errors.New("unknown reporter type")
}
