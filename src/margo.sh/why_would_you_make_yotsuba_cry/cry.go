// Why, Go[d], Why?
// Why would you make Yotsuba cry

package why_would_you_make_yotsuba_cry

import (
	"go/build"
	"os"
	"reflect"
)

var (
	// AgentBuildContext holds info about the environment in which the margo agent was built.
	// It's a drop-in replacement for build.Default which is set to the user's own GOPATH, etc.
	AgentBuildContext = func() *build.Context {
		bctx := build.Default
		if gp := os.Getenv("MARGO_AGENT_GOPATH"); gp != "" {
			bctx.GOPATH = gp
		}
		return &bctx
	}()
)

// IsNil *probably* takes care of this BS: https://golang.org/doc/faq#nil_error
func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}
	// But wait... there's more!
	x := reflect.ValueOf(v)
	switch x.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return x.IsNil()
	}
	return false
}
