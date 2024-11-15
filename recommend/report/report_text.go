// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package report

import (
	_ "embed" // need for embedding
	"fmt"
	"os"
	"strings"

	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/image"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
)

// TextReport Report in Text format
type TextReport struct {
	table     *tablewriter.Table
	outString *strings.Builder
}

// NewTextReport instantiation of new TextReport
func NewTextReport() TextReport {
	str := &strings.Builder{}
	table := tablewriter.NewWriter(str)
	return TextReport{
		table:     table,
		outString: str,
	}
}

func (r TextReport) writeImageSummary(img *image.Info, outDir string, currentVersion string) {
	t := tablewriter.NewWriter(r.outString)
	t.SetBorder(false)
	if img.Deployment != "" {
		dp := fmt.Sprintf("%s/%s", img.Namespace, img.Deployment)
		t.Append([]string{"Deployment", dp})
	}
	t.Append([]string{"Container", img.RepoTags[0]})
	t.Append([]string{"OS", img.OS})
	t.Append([]string{"Arch", img.Arch})
	t.Append([]string{"Distro", img.Distro})
	t.Append([]string{"Output Directory", img.GetPolicyDir(outDir)})
	t.Append([]string{"policy-template version", currentVersion})
	t.Render()
}

// Start Start of the section of the text report
func (r TextReport) Start(img *image.Info, outDir string, currentVersion string) error {
	r.writeImageSummary(img, outDir, currentVersion)
	r.table.SetHeader([]string{"Policy", "Short Desc", "Severity", "Action", "Tags"})
	r.table.SetAlignment(tablewriter.ALIGN_LEFT)
	r.table.SetRowLine(true)
	return nil
}

// SectionEnd end of section of the text table
func (r TextReport) SectionEnd() error {
	r.table.Render()
	r.table.ClearRows()
	r.outString.WriteString("\n")
	return nil
}

// Record addition of new text table row
func (r TextReport) Record(ms common.MatchSpec, policyName string) error {
	var rec []string
	policyName = policyName[strings.LastIndex(policyName, "/")+1:]
	rec = append(rec, wrapPolicyName(policyName, 35))
	rec = append(rec, ms.Description.Tldr)
	rec = append(rec, fmt.Sprintf("%d", ms.Spec.Severity))
	rec = append(rec, string(ms.Spec.Action))
	rec = append(rec, strings.Join(ms.Spec.Tags[:], "\n"))
	r.table.Append(rec)
	return nil
}

func wrapPolicyName(name string, limit int) string {
	line := ""
	lines := []string{}

	strArr := strings.Split(name, "-")

	strArrLen := len(strArr)
	for i, str := range strArr {
		var newLine string
		if (i + 1) != strArrLen {
			newLine = line + str + "-"
		} else {
			newLine = line + str
		}

		if len(newLine) <= limit {
			line = newLine
		} else {
			lines = append(lines, line)
			line = strings.TrimPrefix(newLine, line)
		}
	}
	lines = append(lines, line)

	return strings.Join(lines, "\n")
}

// Render output the table
func (r TextReport) Render(out string) error {
	if err := os.WriteFile(out, []byte(r.outString.String()), 0o600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	return nil
}
