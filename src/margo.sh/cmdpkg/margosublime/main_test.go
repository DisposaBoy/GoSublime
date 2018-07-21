package margosublime

import (
	"margo.sh/mg"
	"margo.sh/mgutil"
	"margo.sh/sublime"
	"testing"
)

func TestConfig(t *testing.T) {
	ac := agentConfig
	ac.Stdin = &mgutil.IOWrapper{}
	ac.Stdout = &mgutil.IOWrapper{}
	ac.Stderr = &mgutil.IOWrapper{}
	ag, err := mg.NewAgent(ac)
	if err != nil {
		t.Fatalf("agent creation failed: %v", err)
	}

	ag.Store.SetBaseConfig(sublime.DefaultConfig)
	mxc := make(chan *mg.Ctx)
	ag.Store.Subscribe(func(mx *mg.Ctx) {
		mxc <- mx
	})
	ag.Store.Dispatch(mg.Render)
	go ag.Run()
	mx := <-mxc

	if _, ok := mx.Config.(sublime.Config); !ok {
		t.Fatalf("mx.Config is %T, not %T", mx.Config, sublime.Config{})
	}

	ec := mx.Config.EditorConfig()
	cv, ok := ec.(sublime.ConfigValues)
	if !ok {
		t.Fatalf("mx.Config.EditorConfig() is %T, not %T", ec, sublime.ConfigValues{})
	}

	if len(cv.EnabledForLangs) == 0 {
		t.Fatal("EditorConfig().Values.EnabledForLangs in empty")
	}
}
