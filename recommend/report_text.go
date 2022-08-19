// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding
	"os"
	"strconv"
	"strings"

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

func (r TextReport) writeImageSummary(img *ImageInfo) {
	t := tablewriter.NewWriter(r.outString)
	t.SetBorder(false)
	t.Append([]string{"Container", img.RepoTags[0]})
	t.Append([]string{"OS", img.OS})
	t.Append([]string{"Arch", img.Arch})
	t.Append([]string{"Distro", img.Distro})
	t.Render()
}

// Start Start of the section of the text report
func (r TextReport) Start(img *ImageInfo) error {
	r.writeImageSummary(img)
	r.table.SetHeader([]string{"Policy", "Short Desc", "Severity", "Action", "Tags"})
	r.table.SetAlignment(tablewriter.ALIGN_LEFT)
	return nil
}

// SectionEnd end of section of the text table
func (r TextReport) SectionEnd(img *ImageInfo) error {
	r.table.Render()
	r.table.ClearRows()
	r.outString.WriteString("\n")
	return nil
}

// Record addition of new text table row
func (r TextReport) Record(ms MatchSpec, policyName string) error {
	var rec []string

	rec = append(rec, policyName)
	rec = append(rec, ms.Description.Tldr)
	rec = append(rec, strconv.Itoa(ms.OnEvent.Severity))
	rec = append(rec, ms.OnEvent.Action)
	rec = append(rec, strings.Join(ms.OnEvent.Tags[:], ","))
	r.table.Append(rec)
	return nil
}

// Render output the table
func (r TextReport) Render(out string) error {

	if err := os.WriteFile(out, []byte(r.outString.String()), 0600); err != nil {
		log.WithError(err).Error("failed to write file")
	}
	return nil
}
