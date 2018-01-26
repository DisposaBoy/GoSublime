package mg

type EphemeralState struct {
	Status []string
}

type State struct {
	EphemeralState
}

func (s State) AppendStatus(l ...string) State {
	s.Status = append(s.Status[:len(s.Status):len(s.Status)], l...)
	return s
}
