package mg

import (
	"margo.sh/mg/actions"
)

var (
	_ actions.ClientAction = CmdOutput{}
	_ actions.ClientAction = Activate{}
	_ actions.ClientAction = Restart{}
	_ actions.ClientAction = Shutdown{}
)

type clientActionSupport struct{ ReducerType }

func (cas *clientActionSupport) Reduce(mx *Ctx) *State {
	if act, ok := mx.Action.(actions.ClientAction); ok {
		switch act := act.(type) {
		case Activate:
			mx.Log.Printf("client action Activate(%s:%d:%d) dispatched\n", act.Path, act.Row, act.Col)
		case Restart, Shutdown:
			mx.Log.Printf("client action %s dispatched\n", act.ClientAction().Name)
		}
		return mx.addClientActions(act)
	}
	return mx.State
}
