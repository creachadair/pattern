// Package transform implements pattern transformations between pairs of string
// templates, as defined by the bitbucket.org/creachadair/pattern package.
package transform

import (
	"errors"
	"fmt"
	"strings"

	"bitbucket.org/creachadair/pattern"
)

// An R represents a reversible transformation between two templates, L and R.
// A reversible transformation has the property that its forward and reverse
// applications are inverses of each other, meaning that if
//
//    a, err := r.Apply(x)  // and err == nil
//
// then
//
//    b, err := r.Reverse().Apply(a)
//
// succeeds with a == b, and vice versa.
type R struct{ t *T }

// NewReversible constructs a new reversible transformation from the template
// strings lhs and rhs, and the bindings shared by both templates.  If the
// resulting transformation is not reversible, it returns ErrNotReversible.
func NewReversible(lhs, rhs string, binds pattern.Binds) (R, error) {
	t, err := New(lhs, rhs, binds)
	if err != nil {
		if _, ok := err.(*pattern.ParseError); !ok {
			err = ErrNotReversible
		}
		return R{}, err
	} else if !reversible(t.lhs.Binds(), t.rhs.Binds()) {
		return R{}, ErrNotReversible
	}
	return R{t: t}, nil
}

// MustReversible is as NewReversible, but panics if an error is reported. This
// function exists to support static initialization.
func MustReversible(lhs, rhs string, binds pattern.Binds) R {
	r, err := NewReversible(lhs, rhs, binds)
	if err != nil {
		panic("transform: " + err.Error())
	}
	return r
}

// Reverse returns the reverse transformation of R, with its left and right
// templates in the opposite order.
func (r R) Reverse() R { return R{t: &T{lhs: r.t.rhs, rhs: r.t.lhs}} }

// Apply applies the transformation, as (*T).Apply.
func (r R) Apply(needle string) (string, error) { return r.t.Apply(needle) }

// Search performs the search transformation, as (*T).Search.
func (r R) Search(needle string, f func(int, int, string) error) error {
	return r.t.Search(needle, f)
}

// ErrNotReversible is returned by NewReversible if its template arguments do
// not produce a reversible transformation.
var ErrNotReversible = errors.New("transformation is not reversible")

// A T represents a transformation between two templates, L and R.  Applying
// the transformation matches L against the needle, and if the match succeeds
// applies the resulting bindings to R.
type T struct {
	lhs, rhs *pattern.P
}

// New constructs a new transformation from the template strings lhs and rhs,
// and the bindings shared by both templates.
func New(lhs, rhs string, binds pattern.Binds) (*T, error) {
	lp, err := pattern.Parse(lhs, binds)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %v", lhs, err)
	}
	rp, err := lp.Derive(rhs)
	if err != nil {
		return nil, err
	}
	return &T{lhs: lp, rhs: rp}, nil
}

// Must is as New, but panics if an error is reported. This function exists to
// support static initialization.
func Must(lhs, rhs string, binds pattern.Binds) *T {
	t, err := New(lhs, rhs, binds)
	if err != nil {
		panic("transform: " + err.Error())
	}
	return t
}

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
// and calls f with the starting and ending offsets of the original match,
// along with the transformed string. If f reports an error, the search ends.
// If the error is ErrStopSearch, Search returns nil. Otherwise Search returns
// the error from f.
func (t *T) Search(needle string, f func(start, end int, match string) error) error {
	return t.lhs.Search(needle, func(start, end int, binds pattern.Binds) error {
		out, err := t.rhs.Apply(binds)
		if err != nil {
			return err
		}
		return f(start, end, out)
	})
}

// Replace replaces all non-overlapping matches of the left pattern of t with
// the results of applying the right pattern of t.
func (t *T) Replace(needle string) (string, error) {
	var out strings.Builder
	cur := 0
	if err := t.Search(needle, func(start, end int, match string) error {
		out.WriteString(needle[cur:start])
		out.WriteString(match)
		cur = end
		return nil
	}); err != nil {
		return "", err
	}
	return out.String(), nil
}

// reversible reports whether two sets of bindings are mutually saturating,
// meaning that each contains at least as many values for each binding as the
// other requires. This check does not reflect permutations of order within
// bindings of the same name (since it doesn't examine values).
func reversible(a, b pattern.Binds) bool {
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
