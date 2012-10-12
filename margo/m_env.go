package main

import (
	"os"
	"runtime"
)

var (
	mEnvVars = map[string]string{
		"GOROOT": runtime.GOROOT(),
		"GOARCH": runtime.GOARCH,
		"GOOS":   runtime.GOOS,
	}
)

type mEnv struct {
	List []string
}

func (m *mEnv) Arg() interface{} {
	return m
}

func (m *mEnv) Call() (interface{}, string) {
	env := map[string]string{}
	for _, k := range m.List {
		v := os.Getenv(k)
		if v == "" {
			v = mEnvVars[k]
		}
		env[k] = v
	}
	return env, ""
}

func init() {
	registry.Register("env", func() Caller {
		return &mEnv{}
	})
}
