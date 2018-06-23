package pattern

import (
	"errors"
	"fmt"
)

// A T represents a reversible transformation between two templates, L and R.
// A reversible transformation has the property that its forward and reverse
// applications are inverses of each other, meaning that if
//
//    a, err := t.Forward(x)  // and err == nil
//
// then
//
//    b, err := t.Reverse(a)  // gives err == nil
//
// succeeds with a == b.
type T struct {
	lhs, rhs *P
}

// ErrNotReversible is returned by Transformer if its template arguments do not
// produce a reversible transformation.
var ErrNotReversible = errors.New("transformation is not reversible")

// Transformer constructs a new reversible transformation from the template
// strings lhs and rhs, and the bindings shared by both templates.  If the
// resulting transformation is not reversible, it returns ErrNotReversible.
func Transformer(lhs, rhs string, binds Binds) (*T, error) {
	lp, err := Parse(lhs, binds)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %v", lhs, err)
	}
	rp, err := lp.Derive(rhs)
	if err != nil {
		if _, ok := err.(parseError); ok {
			return nil, fmt.Errorf("parsing %q: %v", rhs, err)
		}
		return nil, ErrNotReversible
	}
	if !reversible(lp.Binds(), rp.Binds()) {
		return nil, ErrNotReversible
	}
	return &T{lhs: lp, rhs: rp}, nil
}

// Forward matches needle against the left pattern of t, and if it matches
// applies the result to the right pattern of t.
func (t *T) Forward(needle string) (string, error) {
	return matchApply(t.lhs, t.rhs, needle)
}

// Reverse matches needle against the right pattern of t, and if it matches
// applies the result to the left pattern of t.
func (t *T) Reverse(needle string) (string, error) {
	return matchApply(t.rhs, t.lhs, needle)
}

// reversible reports whether two sets of bindings are mutually saturating,
// meaning that each contains at least as many values for each binding as the
// other requires. This check does not reflect permutations of order within
// bindings of the same name (since it doesn't examine values).
func reversible(a, b Binds) bool {
	na := make(map[string]int)
	for _, bind := range a {
		na[bind.Name]++
	}
	for _, bind := range b {
		if _, ok := na[bind.Name]; !ok {
			return false // a does not bind this name at all
		}
		na[bind.Name]--
		if na[bind.Name] < 0 {
			return false // a does not bind this name often enough
		}
	}
	for _, v := range na {
		if v != 0 {
			return false // b does not bind this name often enough
		}
	}
	return true
}

func matchApply(p, q *P, needle string) (string, error) {
	ms, err := p.Match(needle)
	if err != nil {
		return "", err
	}
	return q.Apply(ms)
}
