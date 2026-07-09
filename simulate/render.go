// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Report holds aggregate simulation output.
type Report struct {
	PolicyName  string        `json:"policy"`
	Namespace   string        `json:"namespace,omitempty"`
	Window      string        `json:"window"`
	Matched     int           `json:"matched"`
	Blocked     int           `json:"blocked"`
	Audited     int           `json:"audited"`
	Allowed     int           `json:"allowed"`
	Results     []EventResult `json:"results"`
	EventsFile  string        `json:"eventsFile,omitempty"`
	LiveCollect bool          `json:"liveCollect,omitempty"`
}

// RenderOutput prints the simulation report in text or JSON format.
func RenderOutput(w io.Writer, report Report, format string, showAllowed bool) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	default:
		return renderText(w, report, showAllowed)
	}
}

func renderText(w io.Writer, report Report, showAllowed bool) error {
	nsLine := ""
	if report.Namespace != "" {
		nsLine = fmt.Sprintf(" (namespace: %s)", report.Namespace)
	}
	if _, err := fmt.Fprintf(w, "\nSimulating policy: %s\n", report.PolicyName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Matched %d events in the last %s%s\n\n", report.Matched, report.Window, nsLine); err != nil {
		return err
	}

	for _, r := range report.Results {
		if r.Verdict == VerdictAllowed && !showAllowed {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s %s [pid=%d, pod=%s, container=%s]\n",
			r.Verdict, r.DisplayPath, r.PID, r.PodName, r.ContainerName); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "\nSummary: %d events would be blocked, %d would be audited, %d would be allowed.\n",
		report.Blocked, report.Audited, report.Allowed); err != nil {
		return err
	}
	return nil
}

// BuildReport aggregates event results into a report.
func BuildReport(policyName, namespace, window string, results []EventResult) Report {
	report := Report{
		PolicyName: policyName,
		Namespace:  namespace,
		Window:     window,
		Matched:    len(results),
		Results:    results,
	}
	for _, r := range results {
		switch r.Verdict {
		case VerdictWouldBlock:
			report.Blocked++
		case VerdictWouldAudit:
			report.Audited++
		default:
			report.Allowed++
		}
	}
	return report
}

// PrintReport renders to stdout.
func PrintReport(report Report, format string, showAllowed bool) error {
	return RenderOutput(os.Stdout, report, format, showAllowed)
}
