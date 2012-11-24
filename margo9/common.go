package main

import (
	"os"
)

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func envSlice(envMap map[string]string) []string {
	env := []string{}
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	if len(env) == 0 {
		env = os.Environ()
	}
	return env
}
