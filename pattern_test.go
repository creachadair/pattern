package pattern

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		parts []string
		rules []string
	}{
		// Unadorned literals.
		{"", nil, nil},
		{"foo", []string{"foo"}, nil},
		{"{foo}", []string{"{foo}"}, nil},
		{"{foo", []string{"{foo"}, nil},
		{"foo}", []string{"foo}"}, nil},

		// Escaping of $.
		{"$$foo", []string{"$foo"}, nil},
		{"foo$$", []string{"foo$"}, nil},
		{"foo$$bar", []string{"foo$bar"}, nil},
		{"foo$${bar", []string{"foo${bar"}, nil},

		// Escaping (or not) of word brackets.
		{"${foo}", []string{"", "foo"}, []string{"foo"}},
		{"$${foo}", []string{"${foo}"}, nil},

		// Interleaving of brackets and non-brackets.
		{"foo${bar}baz", []string{"foo", "bar", "baz"}, []string{"bar"}},
		{"a${b}c${b}d", []string{"a", "b", "c", "b", "d"}, []string{"b"}},
		{"a${b}c${d}e", []string{"a", "b", "c", "d", "e"}, []string{"b", "d"}},
		{"${a}b${c}d${e}", []string{"", "a", "b", "c", "d", "e"}, []string{"a", "c", "e"}},
		{"${a}${b}", []string{"", "a", "", "b"}, []string{"a", "b"}},
		{"a${b}${c}d", []string{"a", "b", "", "c", "d"}, []string{"b", "c"}},

		// Content of word names.
		{"${a:b} ${c/d} ${_e_} ${--F} ${+gee} ${#25} ${h=18}",
			[]string{"", "a:b", " ", "c/d", " ", "_e_", " ", "--F", " ", "+gee", " ", "#25", " ", "h=18"},
			[]string{"a:b", "c/d", "_e_", "--F", "+gee", "#25", "h=18"}},
	}
	for _, test := range tests {
		got, err := Parse(test.input, nil)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", test.input, err)
			continue
		}
		if !reflect.DeepEqual(got.parts, test.parts) {
			t.Errorf("Parse(%q) parts\ngot:  %+q\nwant: %+q", test.input, got.parts, test.parts)
		}
		var rules []string
		for name := range got.rules {
			rules = append(rules, name)
		}
		sort.Strings(rules)
		sort.Strings(test.rules)
		if !reflect.DeepEqual(rules, test.rules) {
			t.Errorf("Parse(%q) rules\ngot:  %+q\nwant: %+q", test.input, rules, test.rules)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []string{
		"$",     // incomplete escape
		"a$",    // "
		"$ ",    // "
		"${",    // incomplete pattern word
		"a${bc", // "
		"${}",   // empty pattern word
		"${ }",  // invalid name letter
		"${a^}", // "
	}
	for _, test := range tests {
		got, err := Parse(test, nil)
		if err == nil {
			t.Errorf("Parse(%q): got %+v, wanted error", test, got)
		} else {
			t.Logf("Parse(%q): correctly failed: %v", test, err)
		}
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		binds   Binds
		needle  string
		want    Binds
	}{
		// A plain string should match itself.
		{"alpha", nil, "alpha", nil},

		// Escaped stuff in the pattern should match literally.
		{"35$$", nil, "35$", nil},
		{"$${ok", nil, "${ok", nil},

		// A simple binding.
		{"A#${num}", Binds{{"num", "\\d+"}}, "A#5", []Bind{{"num", "5"}}},

		// Repeated occurrences of the same pattern word.
		{"[ ${x} | ${x} ]", Binds{{"x", "\\d+"}}, "[ 1 | 2 ]", Binds{
			{"x", "1"}, {"x", "2"},
		}},

		// Multiple distinct pattern words.
		{"${a} ${y} ${b}", Binds{
			{"a", "(?i)all"}, {"y", "(?i)your"}, {"b", "(?i)base"},
		}, "ALL YOUR BASE", Binds{
			{"a", "ALL"}, {"y", "YOUR"}, {"b", "BASE"},
		}},

		// Distinct pattern words with repetitions.
		{"${a} and ${b} and ${a} again${c}", Binds{
			{"a", "\\w+"}, {"b", "\\d+"}, {"c", "[.?]"},
		}, "red and 25 and blue again?", Binds{
			{"a", "red"}, {"b", "25"}, {"a", "blue"}, {"c", "?"},
		}},
	}
	for _, test := range tests {
		p, err := Parse(test.pattern, test.binds)
		if err != nil {
			t.Errorf("Parse %q failed: %v", test.pattern, err)
			continue
		}

		m, err := p.Match(test.needle)
		if err != nil {
			t.Errorf("Match %q failed: %v", test.needle, err)
			continue
		}

		if !reflect.DeepEqual(m, test.want) {
			t.Errorf("Match %q:\ngot:  %+v\nwant: %+v", test.needle, m, test.want)
		}
	}
}

func TestMatchErrors(t *testing.T) {
	t.Run("BadCompile", func(t *testing.T) {
		p := MustParse(`arg${vowel}naut`, []Bind{{"vowel", "[bad"}})
		m, err := p.Match("it got better")
		if err == nil {
			t.Errorf("Match: got %+v, wanted error", m)
		} else {
			t.Logf("Match: correctly failed: %v", err)
		}
	})
	t.Run("ErrNoMatch", func(t *testing.T) {
		p := MustParse(`arg${vowel}naut`, []Bind{{"vowel", "(?i)[aeiou]"}})
		for _, test := range []string{
			"",           // no match
			"argo",       // incomplete match
			"naut",       // "
			" argonaut ", // match does not consume the whole string
		} {
			m, err := p.Match(test)
			if err == nil {
				t.Errorf("Match %q: got %+v, wanted error", test, m)
			} else {
				t.Logf("Match %q: correctly failed: %v", test, err)
			}
		}
	})
	t.Run("NoBinding", func(t *testing.T) {
		p := MustParse(`arg${o}naut`, nil)
		m, err := p.Match("argonaut")
		if err == nil {
			t.Errorf("Match: got %+v, wanted error", m)
		} else {
			t.Logf("Match correctly failed: %v", err)
		}
	})
}

func TestSearch(t *testing.T) {
	//                          1   1   2   2   2   3
	//              0   4   8   2   6   0   4   8   2
	const needle = `A1, B2, C3, D4, E5, F6, G7, H8, I9`
	//              ^^              ^^              ^^
	p := MustParse(`${x}${0}`, Binds{
		{Name: "x", Expr: "[AEIOU]"}, {Name: "0", Expr: "[0-9]"},
	})

	t.Run("All", func(t *testing.T) {
		want := map[string]int{"A1": 0, "E5": 16, "I9": 32}
		got := make(map[string]int)

		if err := p.Search(needle, func(i, j int, binds Binds) error {
			// Check that the bound values are what the range contains.
			a := binds.First("x") + binds.First("0")
			b := needle[i:j]

			if a != b {
				t.Errorf("Search [%d:%d] bound %q ≠ indexed %q", i, j, a, b)
			} else {
				t.Logf("Search [%d:%d] bound=%q indexed=%q", i, j, a, b)
			}
			got[a] = i
			return nil
		}); err != nil {
			t.Errorf("Search %q failed: %v", needle, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Search %q:\n got: %+v\nwant: %+v", needle, got, want)
		}
	})

	// Verify that a stop error is propagated as nil.
	t.Run("StopEarly", func(t *testing.T) {
		var found string
		if err := p.Search(needle, func(i, j int, binds Binds) error {
			found = needle[i:j]
			return ErrStopSearch
		}); err != nil {
			t.Errorf("Search %q failed: %v", needle, err)
		} else if found != "A1" {
			t.Errorf("Search %q did not find A1", needle)
		}
	})

	// Verify that other errors generated by f get propagated.
	t.Run("Errors", func(t *testing.T) {
		want := errors.New("minions of bogosity")
		got := p.Search(needle, func(i, j int, binds Binds) error {
			t.Logf("Search [%d:%d] %q", i, j, needle[i:j])
			if binds.First("x") == "E" {
				return want
			}
			return nil
		})
		if got != want {
			t.Errorf("Search %q: got error %v, want %v", needle, got, want)
		}
	})
}

func TestApply(t *testing.T) {
	p := MustParse(`${thing} is as ${thing} ${verb}`, nil)
	tests := []struct {
		binds []Bind
		want  string
	}{
		// Everything required is present.
		{[]Bind{{"thing", "value"}, {"verb", "pays"}, {"thing", "customer"}},
			"value is as customer pays"},

		// Multiple uses pad out with the last value.
		{[]Bind{{"thing", "handsome"}, {"verb", "does"}},
			"handsome is as handsome does"},

		// Unnecessary bindings are ignored.
		{[]Bind{{"thing", "Apple"}, {"thing", "orange"}, {"verb", "compares"},
			{"foo", "bar"}, {"frob", "quux"}}, // unnecessary values
			"Apple is as orange compares"},

		// Extra values for useful bindings are ignored (in order).
		{[]Bind{{"verb", "screws up"}, {"thing", "A screw-up"}, {"thing", "a screw-up"},
			{"verb", "nobody cares"}, {"thing", "whatever, man"}}, // superfluous values
			"A screw-up is as a screw-up screws up"},
	}
	for _, test := range tests {
		got, err := p.Apply(test.binds)
		t.Logf("Apply: %q, %v", got, err)
		if err != nil {
			t.Errorf("Apply %+v:\n  unexpected error: %v", test.binds, err)
		} else if got != test.want {
			t.Errorf("Apply %+v:\n  got %q, want %q", test.binds, got, test.want)
		}
	}

	if got, err := p.Apply(nil); err == nil {
		t.Errorf("Apply(nil): got %q, wanted error", got)
	} else {
		t.Logf("Apply(nil) correctly failed: %v", err)
	}
}

func TestApplyFunc(t *testing.T) {
	p := MustParse(`${a} ${b} ${a} ${a} ${b} ${_c} f`, nil)

	// Apply a custom value filter.
	val := map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}
	got, err := p.ApplyFunc(func(name string, i int) (string, error) {
		if trim := strings.TrimPrefix(name, "_"); trim != name {
			return val[trim], nil
		}
		// Verify that the index reflects the correct ordering.
		return fmt.Sprintf("%s-%d", val[name], i), nil
	})
	if err != nil {
		t.Fatalf("ApplyFunc failed: %v", err)
	}
	t.Logf("ApplyFunc: %q", got)

	const want = `alpha-1 bravo-1 alpha-2 alpha-3 bravo-2 charlie f`
	if got != want {
		t.Errorf("ApplyFunc: got %q, want %q", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	// Verify that the bindings from a match can be applied to recover the
	// original string.

	// Verify the string from applying bindings can be matched to recover the
	// original bindings.

	tests := []struct {
		template string
		input    string
		binds    Binds
	}{
		{"mary ${act}s jane", "mary loves jane",
			Binds{{"act", "\\w+"}},
		},

		{"${1} + ${2} = ${3}", "3 + 7 = 11",
			Binds{{"1", "\\d+"}, {"2", "\\d+"}, {"3", "\\d+"}},
		},
	}
	for _, test := range tests {
		p := MustParse(test.template, test.binds)
		t.Logf("Input: %q", test.input)

		t.Run("Match-Apply", func(t *testing.T) {
			m, err := p.Match(test.input)
			if err != nil {
				t.Fatalf("Match %q failed: %v", test.input, err)
			}
			got, err := p.Apply(m)
			if err != nil {
				t.Errorf("Apply %+v failed: %v", m, err)
			} else if got != test.input {
				t.Errorf("Apply %+v: got %q, want %q", m, got, test.input)
			} else {
				t.Logf("Apply 1: %q", got)
			}
		})

		t.Run("Apply-Match", func(t *testing.T) {
			binds := p.Binds()
			for i := range binds {
				binds[i].Expr = strconv.Itoa(10 * (i + 1))
			}

			s, err := p.Apply(binds)
			if err != nil {
				t.Fatalf("Apply %+v failed: %v", binds, err)
			}
			t.Logf("Apply 2: %q", s)

			got, err := p.Match(s)
			if err != nil {
				t.Errorf("Match %q failed: %v", s, err)
			} else if !reflect.DeepEqual(got, binds) {
				t.Errorf("Match:\n got:  %+v\n want: %+v", got, binds)
			}
		})
	}
}
