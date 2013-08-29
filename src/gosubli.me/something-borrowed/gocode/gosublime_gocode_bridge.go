package gocode

import (
	"reflect"
)

var (
	gosublimeGocodeDaemon *daemon
)

type GoSublimeGocodeCandidate struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
}

func init() {
	gosublimeGocodeDaemon = &daemon{}
	gosublimeGocodeDaemon.cmd_in = make(chan int, 1)
	gosublimeGocodeDaemon.pkgcache = new_package_cache()
	gosublimeGocodeDaemon.declcache = new_decl_cache(&gocode_env{})
	gosublimeGocodeDaemon.autocomplete = new_auto_complete_context(gosublimeGocodeDaemon.pkgcache, gosublimeGocodeDaemon.declcache)
}

func GoSublimeGocodeComplete(file []byte, filename string, cursor int) []GoSublimeGocodeCandidate {
	list, _ := gosublimeGocodeDaemon.autocomplete.apropos(file, filename, cursor)
	candidates := make([]GoSublimeGocodeCandidate, len(list))
	for i, c := range list {
		candidates[i] = GoSublimeGocodeCandidate{
			Name:  c.Name,
			Type:  c.Type,
			Class: c.Class.String(),
		}
	}
	return candidates
}

func GoSublimeGocodeSet(k, v string) {
	g_config.set_option(k, v)
}

func GoSublimeGocodeOptions() map[string]interface{} {
	m := map[string]interface{}{}
	str, typ := g_config.value_and_type()
	for i := 0; i < str.NumField(); i++ {
		name := typ.Field(i).Tag.Get("json")
		v := str.Field(i)
		switch v.Kind() {
		case reflect.Bool:
			m[name] = v.Bool()
		case reflect.String:
			m[name] = v.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			m[name] = v.Int()
		case reflect.Float32, reflect.Float64:
			m[name] = v.Float()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			m[name] = v.Uint()
		}
	}
	return m
}
