package margo

import (
	"disposa.blue/margo/golang"
	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
	"time"
)

func Margo(ma mg.Args) {
	ma.Store.
		// modify the existing default config
		EditorConfig(sublime.DefaultConfig.
			// enable GoSublime to trigger events when the view is activared, saved, etc.
			// for the moment, this isn't enabled by default to aboidbreaking users
			EnableEvents().

			// we could also write the function ourselves but this is IMO less boilerplate
			Config).

		// add our reducers (plugins)
		// reducers are run in the specified order
		Use(
			// reducers don't have direct access to the store
			// and is therefore unable dispatch actions
			timeReducer(ma.Store),

			// use the new gofmt isntegration instead of the old GoSublime version
			golang.GoFmt,
			// or goimports
			// golang.GoImports,
		)
}

// timeReducers add the current time to the status bar
func timeReducer(sto *mg.Store) mg.Reducer {
	return func(st mg.State, act mg.Action) mg.State {
		switch act.(type) {
		case mg.Started:
			// kick off the ticker when we start
			go func() {
				ticker := time.NewTicker(1 * time.Second)
				for range ticker.C {
					// we don't have any specific action to trigger
					// we just want to schedule a re-render
					sto.Dispatch(nil)
				}
			}()
		}

		// we always want to render the time
		// otherwise the it will sometimes disappear from the status bar
		now := time.Now()
		return st.AddStatus(now.Format("Mon")).AddStatus(now.Format("15:04:05"))
	}
}
