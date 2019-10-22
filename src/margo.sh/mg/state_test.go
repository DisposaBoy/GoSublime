package mg

import (
	"reflect"
	"testing"
	"time"
)

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
	ag := NewTestingAgent(nil, nil, nil)
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
