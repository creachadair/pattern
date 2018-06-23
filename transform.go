package pattern

import (
	"errors"
	"fmt"
)

// A T represents a reversible transformation between two templates, L and R.
// A reversible transformation has the property that its forward and reverse
// applications are inverses of each other, meaning that if
//
//    a, err := t.Apply(x)  // and err == nil
//
// then
//
//    b, err := t.Reverse().Apply(a)  // gives err == nil
//
// succeeds with a == b, and vice versa.
type T struct {
	lhs, rhs *P
}

// ErrNotReversible is returned by Transformer if its template arguments do not
// produce a reversible transformation.
var ErrNotReversible = errors.New("transformation is not reversible")

// NewTransform constructs a new reversible transformation from the template
// strings lhs and rhs, and the bindings shared by both templates.  If the
// resulting transformation is not reversible, it returns ErrNotReversible.
func NewTransform(lhs, rhs string, binds Binds) (*T, error) {
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

// MustTransform is as NewTransform, but panics if an error is reported. This
// function exists to support static initialization.
func MustTransform(lhs, rhs string, binds Binds) *T {
	t, err := NewTransform(lhs, rhs, binds)
	if err != nil {
		panic("pattern: " + err.Error())
	}
	return t
}

// Reverse returns the reverse transformation of T, with left and right
// templates in opposite order.
func (t *T) Reverse() *T { return &T{lhs: t.rhs, rhs: t.lhs} }

// Apply matches needle against the left pattern of t, and if it matches
// applies the result to the right pattern of t.
func (t *T) Apply(needle string) (string, error) {
	ms, err := t.lhs.Match(needle)
	if err != nil {
		return "", err
	}
	return t.rhs.Apply(ms)
}

// Search scans needle for all non-overlapping matches of the left pattern of
// t. For each match, Search applies the the result to the right pattern of t
// and calls f with the transformed string. If f reports an error, the search
// ends.  If the error is ErrStopSearch, Search returns nil. Otherwise Search
// returns the error from f.
func (t *T) Search(needle string, f func(string) error) error {
	return t.lhs.Search(needle, func(start, end int, binds Binds) error {
		out, err := t.rhs.Apply(binds)
		if err != nil {
			return err
		}
		return f(out)
	})
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
