package transform

import (
	"strings"
	"testing"

	"bitbucket.org/creachadair/pattern"
)

func TestReversible(t *testing.T) {
	tests := []struct {
		desc     string
		lhs, rhs []string
		want     bool
	}{
		{"Both empty ", nil, nil, true},
		{"LHS nonempty", []string{"a"}, nil, false},
		{"RHS nonempty", nil, []string{"b"}, false},
		{"Exact match",
			[]string{"a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"Permuted match",
			[]string{"c", "a", "b"}, []string{"a", "b", "c"}, true},
		{"Repeated value 1",
			[]string{"foo", "foo"}, []string{"foo", "foo"}, true},
		{"Repeated value 2",
			[]string{"a", "a", "b"}, []string{"a", "b", "a"}, true},
		{"Unbalanced left side",
			[]string{"a", "x", "a", "y"}, []string{"x", "a", "a"}, false},
		{"Unbalanced right side",
			[]string{"a", "x", "x"}, []string{"x", "a", "x", "y"}, false},
		{"Unbalanced both sides",
			[]string{"b", "x", "b"}, []string{"x", "b", "x"}, false},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Logf("LHS %+q", test.lhs)
			lhs := makeBinds(test.lhs)
			t.Logf("RHS %+q", test.rhs)
			rhs := makeBinds(test.rhs)
			if got := reversible(lhs, rhs); got != test.want {
				t.Errorf("reversible: got %v, want %v", got, test.want)
			}
		})
	}
}

func TestReversibleApply(t *testing.T) {
	tests := []struct {
		name     string
		lhs, rhs string
		binds    pattern.Binds
		input    string
	}{
		{"empty", "", "", nil, ""},

		{"static", "x", "y", nil, "x"},

		{"simple", "x${0}", "${0}y", pattern.Binds{{Name: "0", Expr: "\\d+"}}, "x22"},

		{"multi-single-ordered",
			"${1} or ${2} things",
			"{${1}, ${2}}",
			pattern.Binds{{Name: "1", Expr: "\\d+"}, {Name: "2", Expr: "\\d+"}},
			"5 or 6 things",
		},

		{"multi-single-unordered",
			"all your ${x} are belong to ${y}",
			"give ${y} your ${x}",
			pattern.Binds{{Name: "x", Expr: "base"}, {Name: "y", Expr: "us"}},
			"all your base are belong to us",
		},

		{"multi-repeated-unordered",
			"a ${adj} ${adj} ${noun} came by",
			"I want a ^${adj} ^${noun} that is ^${adj}",
			pattern.Binds{{Name: "adj", Expr: "(little|blue)"}, {Name: "noun", Expr: "car"}},
			"a little blue car came by",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Verify that forward | reverse is the identity transformation.
			t.Run("FR", func(t *testing.T) {
				tut, err := Reversible(New(test.lhs, test.rhs, test.binds))
				if err != nil {
					t.Fatalf("NewReversible(%q, %q, ...) failed: %v", test.lhs, test.rhs, err)
				}

				a, err := tut.Apply(test.input)
				if err != nil {
					t.Fatalf("Forward(%q) failed: %v", test.input, err)
				}
				t.Logf("Forward(%q) = %q", test.input, a)

				b, err := tut.Reverse().Apply(a)
				if err != nil {
					t.Fatalf("Reverse(%q) failed: %v", a, err)
				}
				t.Logf("Reverse(%q) = %q", a, b)

				if b != test.input {
					t.Errorf("FR transform: got %q, want %q", b, test.input)
				}
			})

			// Verify that reverse | forward is the identity transformation.
			// Note that the LHS and RHS are swapped here.
			t.Run("RF", func(t *testing.T) {
				tut, err := Reversible(New(test.rhs, test.lhs, test.binds))
				if err != nil {
					t.Fatalf("NewReversible(%q, %q, ...) failed: %v", test.rhs, test.lhs, err)
				}

				b, err := tut.Reverse().Apply(test.input)
				if err != nil {
					t.Fatalf("Reverse(%q) failed: %v", test.input, err)
				}
				t.Logf("Reverse(%q) = %q", test.input, b)

				a, err := tut.Apply(b)
				if err != nil {
					t.Fatalf("Forward(%q) failed: %v", b, err)
				}
				t.Logf("Forward(%q) = %q", b, a)

				if a != test.input {
					t.Errorf("RF transform: got %q, want %q", a, test.input)
				}
			})
		})
	}
}

func TestNewErrors(t *testing.T) {
	nonrev := []struct {
		lhs, rhs string
	}{
		{"${a}", "boof"},
		{"beef", "${b}"},
		{"${a},${x},${a},${y}", "${x} + ${a} + ${a}"},
		{"${a},${x},${x}", "${x} + ${a} + ${x} + ${y}"},
		{"${b} + ${x} + ${b}", "${x} + ${b} + ${x}"},
	}
	for _, test := range nonrev {
		tut, err := Reversible(New(test.lhs, test.rhs, nil))
		if err != ErrNotReversible {
			t.Errorf("Reversible(New(%q, %q, _)): got (%v, %v), want: %v",
				test.lhs, test.rhs, tut, err, ErrNotReversible)
		}
	}
	const bogus = "${"
	if tut, err := Reversible(New(bogus, "OK", nil)); err == nil {
		t.Errorf("Reversible(New(%q, OK, _)): got %+v, wanted error", bogus, tut)
	}
	if tut, err := Reversible(New("OK", bogus, nil)); err == nil {
		t.Errorf("Reversible(New(OK, %q, _)): got %+v, wanted error", bogus, tut)
	}
}

func TestSearch(t *testing.T) {
	tut := MustReversible(New("(${n} ${op} ${n})", "${n} ${n} ${op}", pattern.Binds{
		{Name: "n", Expr: "\\d+"}, {Name: "op", Expr: "[-+*/]"},
	}))
	const A = "(5 + 3)\n(2 * 4)\n(6 - 3)\n(9 / 1)"
	const B = "5 3 +\n2 4 *\n6 3 -\n9 1 /"

	var fgot []string
	if err := tut.Search(A, func(i, j int, s string) error {
		t.Logf("Forward Search rewrote [%d:%d] %q to %q", i, j, A[i:j], s)
		fgot = append(fgot, s)
		return nil
	}); err != nil {
		t.Errorf("Search forward failed: %v", err)
	}
	t.Logf("Search forward: found %+q", fgot)
	if got := strings.Join(fgot, "\n"); got != B {
		t.Errorf("Search forward: got %q, want %q", got, B)
	}

	var rgot []string
	if err := tut.Reverse().Search(B, func(i, j int, s string) error {
		t.Logf("Reverse Search rewrote [%d:%d] %q to %q", i, j, B[i:j], s)
		rgot = append(rgot, s)
		return nil
	}); err != nil {
		t.Errorf("Search reverse failed: %v", err)
	}
	t.Logf("Search reverse: found %+q", rgot)
	if got := strings.Join(rgot, "\n"); got != A {
		t.Errorf("Search reverse: got %q, want %q", got, A)
	}
}

func TestReplace(t *testing.T) {
	tut := Must(New("`${text}`", "<tt>${text}</tt>", pattern.Binds{
		{Name: "text", Expr: "([^`]*)"},
	}))
	const input = "calling `f` or `g` with no argument returns `#f`"
	const want = "calling <tt>f</tt> or <tt>g</tt> with no argument returns <tt>#f</tt>"

	got, err := tut.Replace(input)
	if err != nil {
		t.Errorf("Replace %q failed: %v", input, err)
	} else if got != want {
		t.Errorf("Replace %q: got %q, want %q", input, got, want)
	}
}

func makeBinds(ss []string) (bs pattern.Binds) {
	for _, s := range ss {
		bs = append(bs, pattern.Bind{Name: s})
	}
	return
}
