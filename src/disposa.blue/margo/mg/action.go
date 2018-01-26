package mg

type ActionType struct{}

func (a ActionType) Type() ActionType {
	return a
}

type Action interface{ Type() ActionType }

// StartAction is dispatched to indicate the start of IPC communication
type StartAction struct{ ActionType }

type HeartbeatAction struct{ ActionType }

func createAction(name string) Action {
	switch name {
	case "", "heartbeat", "ping":
		return HeartbeatAction{}
	default:
		return nil
	}
}
