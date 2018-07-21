// Why, Go[d], Why?
// Why would you make Yotsuba cry

package why_would_you_make_yotsuba_cry

import (
	"reflect"
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
