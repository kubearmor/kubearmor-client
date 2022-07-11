package simulate

type Action = int
type EventType int

const (
	Allow Action = iota
	Audit
	Block
)

const (
	PermissionAllowed = "Allow"
	PermissionDenied  = "Permission Denied"
	PermissionAudit   = "Audit"
)

const (
	Process EventType = iota
	File
	Network
)

// matchRule matches both path and dirs. If it is matchDirectory then isDir flag is set to true.
type matchRule struct {
	path        string
	fromSource  []string
	isownerOnly bool
	isDir       bool
	recursive   bool
}

type matchProtocol struct {
	protocol string
	action   Action
}

type processRules struct {
	rules  []matchRule
	action Action
}

type fileRules struct {
	rules  []matchRule
	action Action
}

type networkRules struct {
	rules  []matchProtocol
	action Action
}

type policy struct {
	pr []processRules
	fr []fileRules
	nr []networkRules
}

type ProcessEvent struct {
	process       string
	parentProcess string
	user          string
}

type Event struct {
	et EventType
	pe ProcessEvent
	// fe FileEvent
	// ne NetworkEvent
}

type SimulationOutput struct {
	Policy    string
	Severity  int
	Type      string
	Source    string
	Operation string
	Resource  string
	Data      string
	Action    string
	Result    string
}

func (pr *processRules) GenerateTelemetry(policyName string, userAction string) []SimulationOutput {
	out := []SimulationOutput{}
	for _, rule := range pr.rules {
		so := SimulationOutput{}
		if rule.path == userAction {
			so.Policy = policyName
			so.Type = "MatchedPolicy"
			so.Source = userAction
			so.Resource = rule.path
			so.Operation = "Process"
			so.Data = "SYS_EXECVE"
			switch pr.action {
			case Allow:
				so.Action = PermissionAllowed
			case Block:
				so.Action = PermissionDenied
			case Audit:
				so.Action = PermissionAudit
				// if the action does not match any of these cases use default posture?
			}
		}

	}
	return out
}
