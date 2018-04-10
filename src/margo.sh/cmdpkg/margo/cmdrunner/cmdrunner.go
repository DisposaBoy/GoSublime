package cmdrunner

import (
	"fmt"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"strings"
)

type Cmd struct {
	Name     string
	Args     []string
	Env      map[string]string
	OutToErr bool
}

func (c Cmd) Run() error {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	if c.OutToErr {
		cmd.Stdout = cmd.Stderr
	} else {
		cmd.Stdout = os.Stdout
	}

	if len(c.Env) != 0 {
		environ := os.Environ()
		cmd.Env = make([]string, 0, len(environ)+1)
		// I don't remember the rules about duplicate env vars...
		for _, s := range os.Environ() {
			k := strings.Split(s, "=")[0]
			if _, exists := c.Env[k]; !exists {
				cmd.Env = append(cmd.Env, s)
			}
		}
		for k, v := range c.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	a := append([]string{c.Name}, c.Args...)
	for i, s := range a {
		a[i] = mgutil.QuoteCmdArg(s)
	}
	fmt.Fprintf(os.Stderr, "``` %s ```\n", strings.Join(a, " "))

	return cmd.Run()
}
