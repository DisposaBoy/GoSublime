package mg

import (
	"fmt"
)

type EditorProps struct {
	Name    string
	Version string
}

type EditorConfig interface {
	EditorConfig() interface{}
}

type EphemeralState struct {
	Config      EditorConfig
	Status      StrSet
	Errors      StrSet
	Completions []Completion
	Tooltips    []Tooltip
}

type State struct {
	EphemeralState
	View     View
	Obsolete bool
}

func (s State) AddStatus(l ...string) State {
	s.Status = s.Status.Add(l...)
	return s
}

func (s State) Errorf(format string, a ...interface{}) State {
	return s.AddError(fmt.Errorf(format, a...))
}

func (s State) AddError(l ...error) State {
	for _, e := range l {
		if e != nil {
			s.Errors = s.Errors.Add(e.Error())
		}
	}
	return s
}

func (s State) SetConfig(c EditorConfig) State {
	s.Config = c
	return s
}

func (s State) SetSrc(src []byte) State {
	s.View = s.View.SetSrc(src)
	return s
}

func (s State) AddCompletions(l ...Completion) State {
	s.Completions = append(s.Completions[:len(s.Completions):len(s.Completions)], l...)
	return s
}

func (s State) AddTooltips(l ...Tooltip) State {
	s.Tooltips = append(s.Tooltips[:len(s.Tooltips):len(s.Tooltips)], l...)
	return s
}

type clientProps struct {
	Editor EditorProps
	Env    EnvMap
	View   View
}
