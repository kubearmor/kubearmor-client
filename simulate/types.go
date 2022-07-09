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
	fromSource  string
	isownerOnly bool
	isDir       bool
	recursive   bool
}

type matchProtocol struct {
	protocol string
	Action   Action
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
