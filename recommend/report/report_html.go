// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package report package
package report

import (
	_ "embed" // need for embedding
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
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

//go:embed html/css/main.css
var mainCSS []byte

//go:embed html/images/v38_6837.png
var imageV38_6837 []byte

//go:embed html/images/v38_7029.png
var imageV38_7029 []byte

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
		ReportTitle: "Security Report",
		DateTime:    time.Now().Format("02-Jan-2006 15:04:05"),
	}
	err = header.Execute(str, hdri)
	if err != nil {
		log.WithError(err).Error("failed to execute report header")
	}
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
func (r HTMLReport) Start(img *image.Info, outDir string, currentVersion string) error {
	seci := SectionInfo{
		HdrCols: []Col{
			{Name: "POLICY"},
			{Name: "DESCRIPTION"},
			{Name: "SEVERITY"},
			{Name: "ACTION"},
			{Name: "TAGS"},
		},
		ImgInfo: []Info{
			{Key: "Container", Val: img.RepoTags[0]},
			{Key: "OS/Arch/Distro", Val: img.OS + "/" + img.Arch + "/" + img.Distro},
			{Key: "Output Directory", Val: img.GetPolicyDir(outDir)},
			{Key: "policy-template version", Val: currentVersion},
		},
	}
	err := r.section.Execute(r.outString, seci)
	if err != nil {
		log.WithError(err)
	}
	return nil
}

// RecordInfo new row information in table
type RecordInfo struct {
	RowID       string
	Rec         []Col
	Policy      string
	Description string
	PolicyType  string
	Refs        []common.Ref
}

// Record addition of new HTML table row
func (r HTMLReport) Record(ms common.MatchSpec, policyName string) error {
	*r.RecordCnt = *r.RecordCnt + 1
	policy, err := os.ReadFile(filepath.Clean(policyName))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("failed to read policy %s", policyName))
	}
	policyName = policyName[strings.LastIndex(policyName, "/")+1:]
	reci := RecordInfo{
		RowID: fmt.Sprintf("row%d", *r.RecordCnt),
		Rec: []Col{
			{Name: policyName},
			{Name: ms.Description.Tldr},
			{Name: fmt.Sprintf("%d", ms.Spec.Severity)},
			{Name: string(ms.Spec.Action)},
			{Name: strings.Join(ms.Spec.Tags[:], "\n")},
		},
		Policy:      string(policy),
		PolicyType:  "Kubearmor Security Policy",
		Description: ms.Description.Detailed,
		Refs:        ms.Description.Refs,
	}
	err = r.record.Execute(r.outString, reci)
	if err != nil {
		log.WithError(err)
	}
	return nil
}

// SectionEnd end of section of the HTML table
func (r HTMLReport) SectionEnd() error {
	return r.sectend.Execute(r.outString, nil)
}

// Render output the table
func (r HTMLReport) Render(out string) error {
	err := r.footer.Execute(r.outString, nil)
	if err != nil {
		log.WithError(err)
	}

	outPath := strings.Join(strings.Split(out, "/")[:len(strings.Split(out, "/"))-1], "/")

	outPath = outPath + "/.static/"

	err = os.MkdirAll(outPath, 0o740)
	if err != nil {
		log.WithError(err)
	}

	if err := os.WriteFile(outPath+"main.css", []byte(mainCSS), 0o600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	if err := os.WriteFile(outPath+"v38_6837.png", []byte(imageV38_6837), 0o600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	if err := os.WriteFile(outPath+"v38_7029.png", []byte(imageV38_7029), 0o600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	if err := os.WriteFile(out, []byte(r.outString.String()), 0o600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	return nil
}
