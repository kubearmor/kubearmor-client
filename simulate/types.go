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

type UserAction struct {
	Operation string
	Path      string
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

func (pr *processRules) GenerateTelemetry(policyName string, userAction UserAction) []SimulationOutput {
	out := []SimulationOutput{}
	if len(pr.rules) > 0 {
		for _, rule := range pr.rules {
			so := SimulationOutput{}
			so.Policy = policyName
			so.Type = "MatchedPolicy"
			so.Operation = "Process"
			so.Data = "SYS_EXECVE"
			so.Resource = rule.path
			if rule.path == userAction.Path {
				so.Source = userAction.Path
				so.Result = GetActionResult(pr.action)
				switch pr.action {
				case Allow:
					so.Action = "Allow"
				case Block:
					so.Action = "Block"
				case Audit:
					so.Action = "Audit"
				}
			}

			if len(rule.fromSource) > 0 {
				if contains(rule.fromSource, userAction.Path) {
					so.Source = userAction.Path
					so.Result = GetActionResult(pr.action)
					switch pr.action {
					case Allow:
						so.Action = "Allow"
					case Block:
						so.Action = "Block"
					case Audit:
						so.Action = "Audit"
					}
				}

			}
			out = append(out, so)
		}
	}
	return out
}

func GetActionResult(action Action) string {
	switch action {
	case Allow:
		return PermissionAllowed
	case Block:
		return PermissionDenied
	case Audit:
		return PermissionAudit
	default:
		return PermissionDenied
	}
}

func ActiontoString(action Action) string {
	switch action {
	case Allow:
		return "Allow"
	case Block:
		return "Block"
	case Audit:
		return "Audit"
	}
	return ""
}
