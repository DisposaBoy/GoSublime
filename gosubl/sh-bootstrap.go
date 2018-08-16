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
		"GOROOT": build.Default.GOROOT,
		"GOPATH": build.Default.GOPATH,
		"PATH":   os.Getenv("PATH"),
	}
	varPat := regexp.MustCompile(`^((?:MARGO|GO|CGO)\w+)=(.+)$`)
	for _, s := range os.Environ() {
		m := varPat.FindStringSubmatch(s)
		if len(m) == 3 {
			env[m[1]] = m[2]
		}
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
