package pattern

import (
	"strings"
	"testing"
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

func TestTransformApply(t *testing.T) {
	tests := []struct {
		name     string
		lhs, rhs string
		binds    Binds
		input    string
	}{
		{"empty", "", "", nil, ""},

		{"static", "x", "y", nil, "x"},

		{"simple", "x${0}", "${0}y", Binds{{"0", "\\d+"}}, "x22"},

		{"multi-single-ordered",
			"${1} or ${2} things",
			"{${1}, ${2}}",
			Binds{{"1", "\\d+"}, {"2", "\\d+"}},
			"5 or 6 things",
		},

		{"multi-single-unordered",
			"all your ${x} are belong to ${y}",
			"give ${y} your ${x}",
			Binds{{"x", "base"}, {"y", "us"}},
			"all your base are belong to us",
		},

		{"multi-repeated-unordered",
			"a ${adj} ${adj} ${noun} came by",
			"I want a ^${adj} ^${noun} that is ^${adj}",
			Binds{{"adj", "(little|blue)"}, {"noun", "car"}},
			"a little blue car came by",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Verify that forward | reverse is the identity transformation.
			t.Run("FR", func(t *testing.T) {
				tut, err := NewTransform(test.lhs, test.rhs, test.binds)
				if err != nil {
					t.Fatalf("NewTransform(%q, %q, ...) failed: %v", test.lhs, test.rhs, err)
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
				tut, err := NewTransform(test.rhs, test.lhs, test.binds)
				if err != nil {
					t.Fatalf("NewTransform(%q, %q, ...) failed: %v", test.rhs, test.lhs, err)
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

func TestNewTransformErrors(t *testing.T) {
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
		tut, err := NewTransform(test.lhs, test.rhs, nil)
		if err != ErrNotReversible {
			t.Errorf("NewTransform(%q, %q, _): got (%v, %v), want: %v",
				test.lhs, test.rhs, tut, err, ErrNotReversible)
		}
	}
	const bogus = "${"
	if tut, err := NewTransform(bogus, "OK", nil); err == nil {
		t.Errorf("NewTransform(%q, OK, _): got %+v, wanted error", bogus, tut)
	}
	if tut, err := NewTransform("OK", bogus, nil); err == nil {
		t.Errorf("NewTransform(OK, %q, _): got %+v, wanted error", bogus, tut)
	}
}

func TestTransformSearch(t *testing.T) {
	tut := MustTransform("(${n} ${op} ${n})", "${n} ${n} ${op}", Binds{
		{"n", "\\d+"}, {"op", "[-+*/]"},
	})
	const A = "(5 + 3)\n(2 * 4)\n(6 - 3)\n(9 / 1)"
	const B = "5 3 +\n2 4 *\n6 3 -\n9 1 /"

	var fgot []string
	if err := tut.Search(A, func(s string) error {
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
	if err := tut.Reverse().Search(B, func(s string) error {
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

func makeBinds(ss []string) (bs Binds) {
	for _, s := range ss {
		bs = append(bs, Bind{Name: s})
	}
	return
}
