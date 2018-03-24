package sublime

import (
	"disposa.blue/margo/mg"
)

var (
	DefaultConfig = Config{}

	_ mg.EditorConfig = DefaultConfig
)

type Config struct {
	Values struct {
		EnabledForLangs            []string
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

func (c Config) EnabledForLangs(langs ...string) mg.EditorConfig {
	c.Values.EnabledForLangs = langs
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
