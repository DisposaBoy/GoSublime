package mg

import (
	"margo.sh/mg/actions"
)

var (
	ActionCreators = (&actions.Registry{}).
		Register("QueryCompletions", QueryCompletions{}).
		Register("QueryCmdCompletions", QueryCmdCompletions{}).
		Register("QueryIssues", QueryIssues{}).
		Register("Restart", Restart{}).
		Register("Shutdown", Shutdown{}).
		Register("ViewActivated", ViewActivated{}).
		Register("ViewFmt", ViewFmt{}).
		Register("DisplayIssues", DisplayIssues{}).
		Register("ViewLoaded", ViewLoaded{}).
		Register("ViewModified", ViewModified{}).
		Register("ViewPosChanged", ViewPosChanged{}).
		Register("ViewPreSave", ViewPreSave{}).
		Register("ViewSaved", ViewSaved{}).
		Register("QueryUserCmds", QueryUserCmds{}).
		Register("QueryTestCmds", QueryTestCmds{}).
		Register("RunCmd", RunCmd{}).
		Register("QueryTooltips", QueryTooltips{})
)

// initAction is dispatched to indicate the start of IPC communication.
// It's the first action that is dispatched.
type initAction struct{ ActionType }

type ActionType = actions.ActionType

type Action = actions.Action

type DisplayIssues struct{ ActionType }

func (di DisplayIssues) ClientAction() actions.ClientData {
	return actions.ClientData{Name: "DisplayIssues", Data: di}
}

type Activate struct {
	ActionType

	Path string
	Name string
	Row  int
	Col  int
}

func (a Activate) ClientAction() actions.ClientData {
	return actions.ClientData{Name: "Activate", Data: a}
}

var Render Action = nil

type QueryCompletions struct{ ActionType }

type QueryCmdCompletions struct {
	ActionType

	Pos  int
	Src  string
	Name string
	Args []string
}

type QueryIssues struct{ ActionType }

// Restart is the action dispatched to initiate a graceful restart of the agent
type Restart struct{ ActionType }

func (r Restart) ClientAction() actions.ClientData {
	return actions.ClientData{Name: "Restart"}
}

// Shutdown is the action dispatched to initiate a graceful shutdown of the agent
type Shutdown struct{ ActionType }

func (s Shutdown) ClientAction() actions.ClientData {
	return actions.ClientData{Name: "Shutdown"}
}

type QueryTooltips struct {
	ActionType

	Row int
	Col int
}

type ViewActivated struct{ ActionType }

type ViewModified struct{ ActionType }

type ViewPosChanged struct{ ActionType }

type ViewFmt struct{ ActionType }

type ViewPreSave struct{ ActionType }

type ViewSaved struct{ ActionType }

type ViewLoaded struct{ ActionType }

type unmount struct{ ActionType }
