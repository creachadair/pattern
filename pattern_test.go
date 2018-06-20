package pattern

import (
	"reflect"
	"sort"
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
		rules := got.Names()
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

func TestBinding(t *testing.T) {
	p := MustParse("name: ${name}\nvalue: ${value}\n", Binds{
		{"name", "xyz"},
		{"value", "pdq"},
	})
	tests := []struct {
		name, expr string
		ok         bool
	}{
		{"name", "\\w+", true},
		{"value", "\\d+", true},
		{"", "", false},
		{"dead", "horse", false},
	}
	for _, test := range tests {
		got := p.Bind(test.name, test.expr)
		if got != test.ok {
			t.Errorf("Bind %q: got %v, want %v", test.name, got, test.ok)
		} else if v := p.rules[test.name]; got && v != test.expr {
			t.Errorf("Bind %q: got %q, want %q", test.name, v, test.expr)
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

func TestDerive(t *testing.T) {
	p := MustParse(`A ${x} in the ${y} is worth ${n} in the ${x}`, []Bind{
		{"x", "\\w+"}, {"y", "(hand|pocket|face)"}, {"n", "\\d+"},
	})

	// Derive a new pattern from p that mentions the same bindings.
	q, err := p.Derive("I have ${n} ${x}s in my ${y}")
	if err != nil {
		t.Fatalf("Derive failed: %v", err)
	}

	// Match against the original pattern to get some values.
	m, err := p.Match("A ferret in the pocket is worth 20 in the face")
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	// Apply the values to the derived pattern.
	got, err := q.Apply(m)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	t.Logf("Apply OK, got %q", got)
}

func TestRoundTrip(t *testing.T) {
	// Each test verifies that a string matched against an original pattern
	// with the given bindings, rendered by a derived pattern on those same
	// bindings, and matched against the derived pattern again, yields a set of
	// bindings that render to the original string.
	tests := []struct {
		original string
		derived  string
		input    string
		binds    Binds
	}{
		{"mary ${act}s jane", "${act} like an animal", "mary loves jane",
			Binds{{"act", "\\w+"}}},

		// Even if the derived string drops some of the occurrences, those that
		// remain should follow the rules.
		{"${1} + ${1} = ${1}", "is ${1} ${1}?", "1 + 3 = 3",
			Binds{{"1", "\\d+"}}},
	}
	for _, test := range tests {
		p, err := Parse(test.original, test.binds)
		if err != nil {
			t.Errorf("Parse %q failed: %v", test.original, err)
			continue
		}
		q, err := p.Derive(test.derived)
		if err != nil {
			t.Errorf("Derive %q failed: %v", test.derived, err)
			continue
		}

		t.Logf("Original string:  %q", test.input)
		m1, err := p.Match(test.input)
		if err != nil {
			t.Errorf("Match 1 %q failed: %v", test.input, err)
			continue
		}
		mid, err := q.Apply(m1)
		if err != nil {
			t.Errorf("Apply 1 %+v failed: %v", m1, err)
			continue
		}
		t.Logf("Derived string:   %q", mid)

		m2, err := q.Match(mid)
		if err != nil {
			t.Errorf("Match 2 %q failed: %v", mid, err)
			continue
		}

		got, err := p.Apply(m2)
		if err != nil {
			t.Errorf("Apply 2 %+v failed: %v", m2, err)
			continue
		}
		t.Logf("Reapplied string: %q", got)

		if got != test.input {
			t.Errorf("Round-trip failed: got %q, want %q", got, test.input)
		}
	}
}
