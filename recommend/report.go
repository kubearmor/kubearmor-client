// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"errors"
	"strings"
)

/*
ReportInit()
for every image {
	ReportStart()
	for every policy {
		ReportRecord()
	}
	for every dynamic_admission_controller_policy {
		ReportAdmissionControllerRecord()
	}
	ReportSectEnd()
}
if recommend_generic_admission_controller_policies {
	ReportStartGenericAdmissionControllerPolicies()
	for every generic_admission_controller_policy {
		ReportAdmissionControllerRecord()
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
func ReportStart(img *ImageInfo) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Start(img)
	case TextReport:
		return v.Start(img)
	}
	return errors.New("unknown reporter type")
}

// ReportStartGenericAdmissionControllerPolicies called once per generic admission controller policy at the start
func ReportStartGenericAdmissionControllerPolicies() error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.StartGenericAdmissionControllerPolicies()
	case TextReport:
		return v.StartGenericAdmissionControllerPolicies()
	}
	return errors.New("unknown reporter type")
}

// ReportRecord called once per policy
func ReportRecord(ms MatchSpec, policyName string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.Record(ms, policyName)
	case TextReport:
		return v.Record(ms, policyName)
	}
	return errors.New("unknown reporter type")
}

// ReportAdmissionControllerRecord called once per admission controller policy
func ReportAdmissionControllerRecord(policyFilePath, action string, annotations map[string]string) error {
	switch v := Handler.(type) {
	case HTMLReport:
		return v.RecordAdmissionController(policyFilePath, action, annotations)
	case TextReport:
		return v.RecordAdmissionController(policyFilePath, action, annotations)
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
