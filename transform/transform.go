// Package transform implements transformations between pairs of string
// patterns, as defined by the bitbucket.org/creachadair/pattern package.
package transform

import (
	"fmt"
	"strings"

	"bitbucket.org/creachadair/pattern"
)

// A T represents a transformation between two patterns, L and R.  Applying the
// transformation matches L against the needle, and if the match succeeds it
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

// Must acts as New, but panics if an error is reported. This function exists
// to support static initialization.
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

// Reverse returns the reverse of t, with its left and right templates
// exchanged.
func (t *T) Reverse() *T { return &T{lhs: t.rhs, rhs: t.lhs} }

// Reversible reports whether the bindings of t are mutually saturating,
// meaning that each contains at least as many values for each binding as the
// other requires. If this is false, it means applying the transformation
// discards information.
//
// This check does not reflect permutations of order within bindings of the
// same name (since it doesn't examine values).
func (t *T) Reversible() bool { return reversible(t.lhs.Binds(), t.rhs.Binds()) }

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
