package mg

var (
	clientRestart  = clientActionType{Name: "restart"}
	clientShutdown = clientActionType{Name: "shutdown"}
)

type clientAction interface {
	clientAction() clientActionType
}

type clientActionType struct {
	Name string
	Data interface{}
}

func (t clientActionType) clientAction() clientActionType {
	return t
}
