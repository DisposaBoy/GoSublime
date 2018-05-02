package mg

import (
	"github.com/ugorji/go/codec"
)

var (
	_ Action = actionType{}

	actionCreators = map[string]actionCreator{
		"QueryCompletions": func(codec.Handle, agentReqAction) (Action, error) {
			return QueryCompletions{}, nil
		},
		"QueryIssues": func(codec.Handle, agentReqAction) (Action, error) {
			return QueryIssues{}, nil
		},
		"Restart": func(codec.Handle, agentReqAction) (Action, error) {
			return Restart{}, nil
		},
		"Shutdown": func(codec.Handle, agentReqAction) (Action, error) {
			return Shutdown{}, nil
		},
		"ViewActivated": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewActivated{}, nil
		},
		"ViewFmt": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewFmt{}, nil
		},
		"ViewLoaded": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewLoaded{}, nil
		},
		"ViewModified": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewModified{}, nil
		},
		"ViewPosChanged": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewPosChanged{}, nil
		},
		"ViewPreSave": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewPreSave{}, nil
		},
		"ViewSaved": func(codec.Handle, agentReqAction) (Action, error) {
			return ViewSaved{}, nil
		},
		"RunCmd": func(h codec.Handle, a agentReqAction) (Action, error) {
			act := RunCmd{}
			err := codec.NewDecoderBytes(a.Data, h).Decode(&act)
			return act, err
		},
		"QueryUserCmds": func(h codec.Handle, a agentReqAction) (Action, error) {
			return QueryUserCmds{}, nil
		},
	}
)

// Started is dispatched to indicate the start of IPC communication.
// It's the first action that is dispatched.
type Started struct{ ActionType }

type actionCreator func(codec.Handle, agentReqAction) (Action, error)

type actionType struct{ ActionType }

type ActionType struct{}

func (act ActionType) actionType() actionType { return actionType{} }

func (act ActionType) ActionLabel() string { return "" }

type Action interface {
	actionType() actionType

	ActionLabel() string
}

var Render Action = nil

type QueryCompletions struct{ ActionType }

type QueryIssues struct{ ActionType }

// Restart is the action dispatched to initiate a graceful restart of the agent
type Restart struct{ ActionType }

// Shutdown is the action dispatched to initiate a graceful shutdown of the agent
type Shutdown struct{ ActionType }

type ViewActivated struct{ ActionType }

type ViewModified struct{ ActionType }

type ViewPosChanged struct{ ActionType }

type ViewFmt struct{ ActionType }

type ViewPreSave struct{ ActionType }

type ViewSaved struct{ ActionType }

type ViewLoaded struct{ ActionType }

type unmount struct{ ActionType }
