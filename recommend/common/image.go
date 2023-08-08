package common

// Report interface
type Report interface {
	ReportRecord(ms MatchSpec, policyName string) error
}
