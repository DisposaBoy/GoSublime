package margo_pkg

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	mEnvVars = defaultEnv()
)

type mEnv struct {
	List   []string
	Gopath string
}

func mEnvGetEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		v = mEnvVars[k]
	}
	return v
}

func (m *mEnv) Call() (interface{}, string) {
	env := map[string]string{}
	addLibPath := false

	if len(m.List) == 0 {
		addLibPath = true

		for k, v := range mEnvVars {
			env[k] = v
		}

		for _, s := range os.Environ() {
			p := strings.SplitN(s, "=", 2)
			if len(p) == 2 {
				env[p[0]] = p[1]
			} else {
				env[p[0]] = ""
			}
		}
	} else {
		for _, k := range m.List {
			if k == "GOSUBLIME_LIBPATH" {
				addLibPath = true
			} else {
				env[k] = mEnvGetEnv(k)
			}
		}
	}

	if addLibPath {
		p := []string{}
		sep := string(os.PathListSeparator)
		osArch := runtime.GOOS + "_" + runtime.GOARCH
		gpath := m.Gopath
		if gpath == "" {
			gpath = mEnvGetEnv("GOPATH")
		}
		for _, s := range strings.Split(gpath, sep) {
			p = append(p, filepath.Join(s, "pkg", osArch))
		}
		env["GOSUBLIME_LIBPATH"] = strings.Join(p, sep)
	}

	return env, ""
}

func init() {
	registry.Register("env", func(_ *Broker) Caller {
		return &mEnv{}
	})
}
