package mg

type StrSet []string

func (s StrSet) Add(l ...string) StrSet {
	res := make(StrSet, 0, len(s)+len(l))
	for _, lst := range [][]string{[]string(s), l} {
		for _, p := range lst {
			if !res.Has(p) {
				res = append(res, p)
			}
		}
	}
	return res
}

func (s StrSet) Has(p string) bool {
	for _, q := range s {
		if p == q {
			return true
		}
	}
	return false
}

type EnvMap map[string]string

func (e EnvMap) Add(k, v string) EnvMap {
	m := make(EnvMap, len(e)+1)
	for k, v := range e {
		m[k] = v
	}
	m[k] = v
	return m
}

func (e EnvMap) Environ() []string {
	l := make([]string, 0, len(e))
	for k, v := range e {
		l = append(l, k+"="+v)
	}
	return l
}

func (e EnvMap) Get(k, def string) string {
	if v := e[k]; v != "" {
		return v
	}
	return def
}
