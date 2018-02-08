package margo

import (
	"disposa.blue/margo/golang"
	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
	"time"
)

func Margo(ma mg.Args) {
	sublConf := sublime.DefaultConfig.
		// enable GoSublime to trigger events when the view is activared, saved, etc.
		// for the moment, this isn't enabled by default to avoid breaking users
		EnableEvents().

		// we could also write the function ourselves but this is IMO less boilerplate
		Config

	ma.Store.
		// modify the existing default config
		EditorConfig(sublConf).

		// add our reducers (plugins)
		// reducers are run in the specified order
		Use(
			DayTimeStatus,

			// both GoFmt and GoImports will automaticallydisable the GoSublime version
			// use the new gofmt integration instead of the old GoSublime version
			golang.GoFmt,
			// or
			// golang.GoImports,
		)
}

// DayTimeStatus adds the current day and time to the status bar
var DayTimeStatus = mg.Reduce(func(mx *mg.Ctx) *mg.State {
	if _, ok := mx.Action.(mg.Started); ok {
		dispatch := mx.Store.Dispatch
		// kick off the ticker when we start
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				// we don't have any specific action to trigger
				// we just want to schedule a re-render
				dispatch(nil)
			}
		}()
	}

	// we always want to render the time
	// otherwise the it will sometimes disappear from the status bar
	now := time.Now()
	format := "Mon, 15:04"
	if now.Second()%2 == 0 {
		format = "Mon, 15 04"
	}
	return mx.AddStatus(now.Format(format))
})
