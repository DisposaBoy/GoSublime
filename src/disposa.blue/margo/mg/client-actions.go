package mg

var (
	clientRestart  = clientActionType{Name: "restart"}
	clientShutdown = clientActionType{Name: "shutdown"}
)

type clientAction interface {
	Type() clientActionType
}

type clientActionType struct {
	Name string
	Data interface{}
}

func (t clientActionType) Type() clientActionType {
	return t
}
