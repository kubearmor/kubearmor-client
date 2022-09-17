// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package recommend package
package recommend

import (
	_ "embed" // need for embedding
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// HTMLReport Report in HTML format
type HTMLReport struct {
	header    *template.Template
	section   *template.Template
	sectend   *template.Template
	record    *template.Template
	footer    *template.Template
	outString *strings.Builder
	RecordCnt *int
}

//go:embed html/record.html
var recordHTML string

//go:embed html/header.html
var headerHTML string

//go:embed html/footer.html
var footerHTML string

//go:embed html/section.html
var sectionHTML string

//go:embed html/sectend.html
var sectendHTML string

// Col column of the table
type Col struct {
	Name string
}

// Info key val pair of the image info
type Info struct {
	Key string
	Val string
}

// HeaderInfo HTML Header Info
type HeaderInfo struct {
	ReportTitle string
	DateTime    string
}

// SectionInfo Section information
type SectionInfo struct {
	HdrCols []Col
	ImgInfo []Info
}

// NewHTMLReport instantiation on new html report
func NewHTMLReport() HTMLReport {
	str := &strings.Builder{}
	header, err := template.New("headertmpl").Parse(headerHTML)
	if err != nil {
		log.WithError(err).Fatal("failed parsing html header template")
	}
	record, err := template.New("recordtmpl").Parse(recordHTML)
	if err != nil {
		log.WithError(err).Fatal("failed parsing html record template")
	}
	footer, err := template.New("footertmpl").Parse(footerHTML)
	if err != nil {
		log.WithError(err).Fatal("failed parsing html footer template")
	}
	section, err := template.New("sectiontmpl").Parse(sectionHTML)
	if err != nil {
		log.WithError(err).Fatal("failed parsing html section template")
	}
	sectend, err := template.New("sectendtmpl").Parse(sectendHTML)
	if err != nil {
		log.WithError(err).Fatal("failed parsing html sectend template")
	}
	hdri := HeaderInfo{
		ReportTitle: "KubeArmor Security Report",
		DateTime:    time.Now().Format("02-Jan-2006 15:04:05"),
	}
	_ = header.Execute(str, hdri)
	recordcnt := 0
	return HTMLReport{
		header:    header,
		section:   section,
		sectend:   sectend,
		record:    record,
		footer:    footer,
		outString: str,
		RecordCnt: &recordcnt,
	}
}

// Start of HTML report section
func (r HTMLReport) Start(img *ImageInfo) error {
	seci := SectionInfo{
		HdrCols: []Col{
			{Name: "Policy"},
			{Name: "Description"},
			{Name: "Severity"},
			{Name: "Action"},
			{Name: "Tags"},
		},
		ImgInfo: []Info{
			{Key: "Container", Val: img.RepoTags[0]},
			{Key: "OS/Arch/Distro", Val: img.OS + "/" + img.Arch + "/" + img.Distro},
		},
	}
	_ = r.section.Execute(r.outString, seci)
	return nil
}

// RecordInfo new row information in table
type RecordInfo struct {
	RowID       string
	Rec         []Col
	Policy      string
	Description string
	Refs        []Ref
}

// Record addition of new HTML table row
func (r HTMLReport) Record(ms MatchSpec, policyName string) error {
	*r.RecordCnt = *r.RecordCnt + 1
	policy, err := os.ReadFile(filepath.Clean(policyName))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("failed to read policy %s", policyName))
	}
	reci := RecordInfo{
		RowID: fmt.Sprintf("row%d", *r.RecordCnt),
		Rec: []Col{
			{Name: policyName},
			{Name: ms.Description.Tldr},
			{Name: fmt.Sprintf("%d", ms.Spec.Severity)},
			{Name: string(ms.Spec.Action)},
			{Name: strings.Join(ms.Spec.Tags[:], ",")},
		},
		Policy:      string(policy),
		Description: ms.Description.Detailed,
		Refs:        ms.Description.Refs,
	}
	_ = r.record.Execute(r.outString, reci)
	return nil
}

// SectionEnd end of section of the HTML table
func (r HTMLReport) SectionEnd(img *ImageInfo) error {
	return r.sectend.Execute(r.outString, nil)
}

// Render output the table
func (r HTMLReport) Render(out string) error {
	_ = r.footer.Execute(r.outString, nil)

	if err := os.WriteFile(out, []byte(r.outString.String()), 0600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	return nil
}
