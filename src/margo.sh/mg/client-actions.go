package mg

var (
	_ clientAction = CmdOutput{}
	_ clientAction = Activate{}
	_ clientAction = Restart{}
	_ clientAction = Shutdown{}
)

type clientAction interface {
	clientAction() clientActionType
}

type clientActionType struct {
	Name string
	Data interface{}
}

type clientActionSupport struct{ ReducerType }

func (cas *clientActionSupport) Reduce(mx *Ctx) *State {
	if act, ok := mx.Action.(clientAction); ok {
		switch act := act.(type) {
		case Activate:
			mx.Log.Printf("client action Activate(%s:%d:%d) dispatched\n", act.Path, act.Row, act.Col)
		case Restart, Shutdown:
			mx.Log.Printf("client action %s dispatched\n", act.clientAction().Name)
		}
		return mx.addClientActions(act)
	}
	return mx.State
}
