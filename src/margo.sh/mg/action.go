package mg

import (
	"github.com/ugorji/go/codec"
)

var (
	actionCreators = map[string]actionCreator{
		"QueryCompletions": func(codec.Handle, agentReqAction) (Action, error) {
			return QueryCompletions{}, nil
		},
		"QueryIssues": func(codec.Handle, agentReqAction) (Action, error) {
			return QueryIssues{}, nil
		},
		"QueryTooltips": func(codec.Handle, agentReqAction) (Action, error) {
			return QueryTooltips{}, nil
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

type actionCreator func(codec.Handle, agentReqAction) (Action, error)

type ActionType struct{}

func (act ActionType) Action() ActionType {
	return act
}

type Action interface {
	Action() ActionType
}

var Render Action = nil

// Started is dispatched to indicate the start of IPC communication.
// It's the first action that is dispatched.
// Reducers may do lazy initialization during this action.
type Started struct{ ActionType }

type QueryCompletions struct{ ActionType }

type QueryIssues struct{ ActionType }

type QueryTooltips struct{ ActionType }

type Restart struct{ ActionType }

// Shutdown is the action dispatched when margo is shutting down.
// Reducers may use it as a signal to do any cleanups with the following caveats:
// * it  might not be dispatched at all
// * it might be dispatched multiple times
// * IPC might not be available so state changes might have no effect
// * logging should, but might not, be available
type Shutdown struct{ ActionType }

type ViewActivated struct{ ActionType }

type ViewModified struct{ ActionType }

type ViewPosChanged struct{ ActionType }

type ViewFmt struct{ ActionType }

type ViewPreSave struct{ ActionType }

type ViewSaved struct{ ActionType }

type ViewLoaded struct{ ActionType }
