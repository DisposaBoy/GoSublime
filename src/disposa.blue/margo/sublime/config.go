package sublime

import (
	"disposa.blue/margo/mg"
)

var (
	DefaultConfig = Config{}
)

type Config struct {
	Values struct {
		Enabled                    bool
		InhibitExplicitCompletions bool
		InhibitWordCompletions     bool
		OverrideSettings           map[string]interface{}
	}
}

func (c Config) EditorConfig() interface{} {
	return c.Values
}

func (c Config) Config() mg.EditorConfig {
	return c
}

func (c Config) EnableEvents() Config {
	c.Values.Enabled = true
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

func (c Config) overrideSetting(k string, v interface{}) Config {
	m := map[string]interface{}{}
	for k, v := range c.Values.OverrideSettings {
		m[k] = v
	}
	m[k] = v
	c.Values.OverrideSettings = m
	return c
}

func (c Config) DisableGsFmt() Config {
	return c.overrideSetting("fmt_enabled", false)
}

func (c Config) DisableGsComplete() Config {
	return c.overrideSetting("gscomplete_enabled", false)
}

func (c Config) DisableGsLint() Config {
	return c.overrideSetting("gslint_enabled", false)
}
