package main

import (
	"encoding/json"
	"go/build"
	"os"
	"regexp"
	"runtime"
	"strings"
)

func main() {
	reVer := regexp.MustCompile(`(\S+(?:\s*[+]\S+)?)`)
	reClean := regexp.MustCompile(`[^\w._]+`)
	rawVer := runtime.Version()
	m := reVer.FindStringSubmatch(rawVer)
	ver := reClean.ReplaceAllString(m[1], "..")
	env := map[string]string{
		"GOROOT":      build.Default.GOROOT,
		"GOPATH":      build.Default.GOPATH,
		"GOBIN":       os.Getenv("GOBIN"),
		"PATH":        os.Getenv("PATH"),
		"CGO_ENABLED": os.Getenv("CGO_ENABLED"),
	}

	for k, v := range env {
		if strings.TrimSpace(v) == "" {
			delete(env, k)
		}
	}

	os.Stdout.WriteString("\n")
	json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"RawVersion": rawVer,
		"Version":    ver,
		"Env":        env,
	})
	os.Stdout.WriteString("\n")
}
