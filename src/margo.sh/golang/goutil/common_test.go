package goutil

import (
	"strings"
	"testing"
)

var (
	escTbSp   = strings.NewReplacer("\t", "<tab>", " ", "<space>")
	unescTbSp = strings.NewReplacer("<tab>", "\t", "<space>", " ")
)

func TestDedent(t *testing.T) {
	cases := []struct{ src, want string }{
		{
			`

	 // empty_lines_at_the_start

	type_T_struct_{
		    //_space_alignment
		S_string
	}

 //_space_before

		//_line_with_extra_indentation

//_line_with_tab_at_the_end<tab>
//_line_with_space_at_the_end<space>

	//_empty_lines_after


`,
			`

 // empty_lines_at_the_start

type_T_struct_{
	    //_space_alignment
	S_string
}

 //_space_before

	//_line_with_extra_indentation

//_line_with_tab_at_the_end<tab>
//_line_with_space_at_the_end<space>

//_empty_lines_after


`,
		},
	}
	for _, c := range cases {
		got := Dedent(unescTbSp.Replace(c.src))
		if got != unescTbSp.Replace(c.want) {
			t.Errorf("got `%s`, want `%s`", escTbSp.Replace(got), escTbSp.Replace(c.want))
		}
	}
}

func TestDedentCompletion(t *testing.T) {
	cases := []struct{ src, want string }{
		{
			`
				 hello world

			`,
			` hello world`,
		},
	}
	for _, c := range cases {
		got := DedentCompletion(c.src)
		if got != c.want {
			t.Errorf("got `%s`, want `%s`", got, c.want)
		}
	}
}
