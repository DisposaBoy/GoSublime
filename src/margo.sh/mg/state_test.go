package mg

import (
	"io"
	"margo.sh/mgutil"
	"reflect"
	"testing"
	"time"
)

// NewTestingAgent creates a new agent for testing
//
// The agent config used is equivalent to:
// * Codec: DefaultCodec
// * Stdin: stdin or &mgutil.IOWrapper{} if nil
// * Stdout: stdout or &mgutil.IOWrapper{} if nil
// * Stderr: &mgutil.IOWrapper{}
func NewTestingAgent(stdout io.WriteCloser, stdin io.ReadCloser) *Agent {
	if stdout == nil {
		stdout = &mgutil.IOWrapper{}
	}
	if stdin == nil {
		stdin = &mgutil.IOWrapper{}
	}
	ag, _ := NewAgent(AgentConfig{
		Stdout: stdout,
		Stdin:  stdin,
		Stderr: &mgutil.IOWrapper{},
	})
	return ag
}

// NewTestingStore creates a new Store for testing
// It's equivalent to NewTestingAgent().Store
func NewTestingStore() *Store {
	return NewTestingAgent(nil, nil).Store
}

// NewTestingCtx creates a new Ctx for testing
// It's equivalent to NewTestingStore().NewCtx()
func NewTestingCtx(act Action) *Ctx {
	return NewTestingStore().NewCtx(act)
}

func checkNonNil(v interface{}, handler func(sel string)) {
	checkNonNilVal(reflect.ValueOf(v), "", handler)
}

func checkNonNilVal(v reflect.Value, sel string, handler func(sel string)) {
	switch kind := v.Kind(); {
	case kind == reflect.Ptr && !v.IsNil():
		checkNonNilVal(v.Elem(), sel, handler)
	case kind == reflect.Struct:
		typ := v.Type()
		switch typ {
		case reflect.TypeOf(time.Time{}):
			return
		}

		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			t := typ.Field(i)
			switch f.Kind() {
			case reflect.Struct:
				checkNonNilVal(f, sel+"."+t.Name, handler)
			case reflect.Ptr, reflect.Interface:
				if f.IsNil() && t.Tag.Get(`mg.Nillable`) != "true" {
					handler(sel + "." + t.Name)
				}
			}
		}
	}
}

// TestNonNilFields checks that NewAgent() doesn't return a Agent, Store,
// State, Ctx or View that has nil fields that are not tagged `mg.Nillable:"true"`
func TestNewAgentNillableFields(t *testing.T) {
	ag := NewTestingAgent(nil, nil)
	mx := ag.Store.NewCtx(nil)
	cases := []interface{}{
		ag,
		ag.Store,
		mx,
		mx.State,
		mx.State.View,
	}

	for _, c := range cases {
		name := reflect.TypeOf(c).String()
		t.Run(name, func(t *testing.T) {
			checkNonNil(c, func(sel string) {
				t.Errorf("(%s)%s is nil but is not tagged mg.Nillable", name, sel)
			})
		})
	}
}
