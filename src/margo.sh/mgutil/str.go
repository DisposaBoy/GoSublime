package mgutil

// StrSet holds a set of strings
type StrSet []string

// NewStrSet returns a new StrSet initialised with the strings in l
func NewStrSet(l ...string) StrSet {
	return StrSet{}.Add(l...)
}

// Add add the list of strings l to the set and returns the new set
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

// Has returns true if p is in the set
func (s StrSet) Has(p string) bool {
	for _, q := range s {
		if p == q {
			return true
		}
	}
	return false
}
