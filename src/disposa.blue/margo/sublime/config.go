package sublime

import (
	"disposa.blue/margo/mg"
)

var (
	DefaultConfig = Config{}
)

type Config struct {
	Values struct {
		TriggerEvents              bool
		InhibitExplicitCompletions bool
		InhibitWordCompletions     bool
	}
}

func (c Config) EditorConfig() interface{} {
	return c.Values
}

func (c Config) TriggerEvents() Config {
	c.Values.TriggerEvents = true
	return c
}

func (c Config) InhibitExplicitCompletions() Config {
	c.Values.InhibitExplicitCompletions = true
	return c
}

func (c Config) InhibitWordCompletions() Config {
	c.Values.InhibitWordCompletions = true
	return c
}

func EditorConfig() mg.EditorConfig {
	return DefaultConfig
}

func ConfigReducer(f func(c Config) Config) mg.Reducer {
	return func(st mg.State, act mg.Action) mg.State {
		switch c := st.Config.(type) {
		case Config:
			return st.SetConfig(f(c))
		}
		return st
	}
}
