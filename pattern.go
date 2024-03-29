// Package pattern implements template-based string matching and substitution.
//
// A *pattern.P represents a template string containing a number of pattern
// words, named locations where substitution may occur. A pattern may be
// "matched" against a string to produce a set of bindings of names to
// substrings; or it may be "applied" to a set of bindings to produce a
// transformed string.
//
// # Template Grammar
//
// A template is a string that contains zero or more pattern words. A pattern
// word has the general format
//
//	${name}
//
// That is, a single word (allowing letters, digits, "/", ":", "_", "-", "+",
// "=", and "#") enclosed in curly brackets, prefixed by a dollar sign ($). To
// include a literal dollar sign, double it ($$); all other characters are
// interpreted as written.
//
// # Matching
//
// Each pattern word is an anchor to a location in the template string.
// Binding regular expressions to the pattern words allows the the pattern to
// match strings.
//
// To match a pattern against a string, use the Match method.  Match succeeds
// if the string is a full regexp match for the expansion of the template with
// the pattern word bindings. A successful match returns a list of Binds that
// give the text of the submatches.
//
// To find multiple matches of the pattern in the string, use the Search
// method. Search behaves like Match, but invokes a callback for each complete,
// non-overlapping match in sequence.
//
// # Substitution
//
// String values may be substituted into a pattern using the Apply and
// ApplyFunc methods. Apply takes an ordered list of Bind values and
// interpolates them into the template; ApplyFunc invokes a callback to
// generate the strings to interpolate.
package pattern

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
)

// P contains a compiled pattern.
type P struct {
	// Even indexes are literal parts of the pattern, odd indexes are the names
	// of pattern words.
	parts    []string
	template string            // the original template
	rules    map[string]string // :: pattern word → regexp
	re       *regexp.Regexp    // cache of compileRegexp
}

// String returns the original template string from which p was parsed.
func (p *P) String() string { return p.template }

// Binds returns a list of bindings for p, in parsed order, populated with the
// currently-bound expression strings. Modifying the result has no effect on p,
// the caller may use this to generate a list of bindings to fill with values.
func (p *P) Binds() Binds {
	var binds Binds
	for i := 1; i < len(p.parts); i += 2 {
		part := p.parts[i]
		binds = append(binds, Bind{
			Name: part,
			Expr: p.rules[part],
		})
	}
	return binds
}

// Match reports whether needle matches p, and if so returns a list of bindings
// for the pattern words occurring in s.  Because the same pattern word may
// occur multiple times in the pattern, the order of bindings is significant.
//
// If matching fails, Match returns nil, ErrNoMatch.
// If matching succeeds but no bindings are found, Match returns nil, nil.
func (p *P) Match(needle string) (Binds, error) {
	re, err := p.compileRegexp()
	if err != nil {
		return nil, err
	}
	m := re.FindStringSubmatchIndex(needle)
	if m == nil || m[0] != 0 || m[1] != len(needle) {
		return nil, ErrNoMatch
	}
	return bindMatches(re, m, needle), nil
}

// Search scans needle for all non-overlapping matches of p. For each match,
// Search calls f with the starting and ending offsets of the match, along with
// the bindings captured from the match. If f reports an error, the search
// ends.  If the error is ErrStopSearch, Search returns nil. Otherwise Search
// returns the error from f.
func (p *P) Search(needle string, f func(start, end int, binds Binds) error) error {
	re, err := p.compileRegexp()
	if err != nil {
		return err
	}
	for _, m := range re.FindAllStringSubmatchIndex(needle, -1) {
		if err := f(m[0], m[1], bindMatches(re, m, needle)); err != nil {
			if err == ErrStopSearch {
				return nil
			}
			return err
		}
	}
	return nil
}

// ErrStopSearch is a special error value that can be returned by the callback
// to Search to terminate search early without error.
var ErrStopSearch = errors.New("stopped searching")

// ErrNoMatch is reported by Match when the pattern does not match the needle.
var ErrNoMatch = errors.New("string does not match pattern")

// Apply applies a list of bindings to the pattern template to produce a new
// string. It is an error if the bindings do not cover the pattern words in the
// template, meaning binds has at least one binding for each pattern word
// mentioned by the template.
//
// If a pattern word appears in the template more often than in binds, the
// value of the last matching binding is repeated to fill the remaining spots.
func (p *P) Apply(binds []Bind) (string, error) {
	sub := make(map[string][]string)
	for _, bind := range binds {
		sub[bind.Name] = append(sub[bind.Name], bind.Expr)
	}
	var out strings.Builder
	for i, part := range p.parts {
		if i%2 == 0 {
			out.WriteString(part)
		} else if s := sub[part]; len(s) == 0 {
			return "", fmt.Errorf("missing binding for %q", part)
		} else {
			out.WriteString(s[0])
			if len(s) > 1 {
				sub[part] = s[1:]
			}
		}
	}
	return out.String(), nil
}

// A BindFunc synthesizes a value for the nth occurrence (indexed from 1) of a
// pattern word with the given name.
type BindFunc func(name string, n int) (string, error)

// ApplyFunc applies bindings generated by f to the pattern template of p to
// produce a new string.  If f reports an error, application fails.
// ApplyFunc will panic if f == nil.
func (p *P) ApplyFunc(f BindFunc) (string, error) {
	index := make(map[string]int) // :: name → index
	var out strings.Builder
	for i, part := range p.parts {
		if i%2 == 0 {
			out.WriteString(part)
			continue
		}
		n := index[part] + 1
		index[part] = n
		s, err := f(part, n)
		if err != nil {
			return "", fmt.Errorf("binding %q: %v", part, err)
		}
		out.WriteString(s)
	}
	return out.String(), nil
}

// Derive constructs a new compiled pattern, using the same pattern words as p
// but with s as the template instead. It is an error if s refers to a pattern
// word not known to p.
func (p *P) Derive(s string) (*P, error) {
	lit, pat, err := parse(s)
	if err != nil {
		return nil, err
	}
	for _, name := range pat {
		if _, ok := p.rules[name]; !ok {
			return nil, fmt.Errorf("unknown pattern word %q", name)
		}
	}
	out := &P{template: s, rules: make(map[string]string)}
	for i, part := range lit {
		out.parts = append(out.parts, part)
		if i < len(pat) {
			out.parts = append(out.parts, pat[i])
			out.rules[pat[i]] = p.rules[pat[i]]
		}
	}
	return out, nil
}

// compileRegexp assembles and compiles a regexp that matches the complete
// template string with the subexpressions for pattern words injected.
func (p *P) compileRegexp() (*regexp.Regexp, error) {
	if p.re == nil {
		var expr strings.Builder
		for i, part := range p.parts {
			if i%2 == 0 {
				expr.WriteString(regexp.QuoteMeta(part))
				continue
			}
			rule, ok := p.rules[part]
			if !ok {
				return nil, fmt.Errorf("no binding for %q", part)
			}
			s, err := syntax.Parse(rule, syntax.Perl)
			if err != nil {
				return nil, fmt.Errorf("invalid expression for %q: %v", part, err)
			}
			fmt.Fprintf(&expr, `(?P<%s>%s)`, part, stripCaptures(s).String())
		}
		r, err := regexp.Compile(expr.String())
		if err != nil {
			return nil, err
		}
		p.re = r
	}
	return p.re, nil
}

// stripCaptures replaces capturing groups with non-capturing groups in re and
// all its recursive subexpressions.
func stripCaptures(re *syntax.Regexp) *syntax.Regexp {
	if re.Op == syntax.OpCapture {
		return stripCaptures(re.Sub[0])
	}
	for i, sub := range re.Sub {
		re.Sub[i] = stripCaptures(sub)
	}
	return re
}

// A Bind associates a pattern word name with a matching expression.
type Bind struct {
	Name string
	Expr string
}

// Binds is an ordered collection of bindings.
type Binds []Bind

// First returns the first bound value of key in bs, in order of occurrence.
// It returns "" if key is not bound.
func (bs Binds) First(key string) string {
	for _, b := range bs {
		if b.Name == key {
			return b.Expr
		}
	}
	return ""
}

// All returns all the bound values of key in bs, in order of occurrence.
func (bs Binds) All(key string) []string {
	var all []string
	for _, b := range bs {
		if b.Name == key {
			all = append(all, b.Expr)
		}
	}
	return all
}

// Has reports whether key is bound at least once in bs.
func (bs Binds) Has(key string) bool {
	for _, b := range bs {
		if b.Name == key {
			return true
		}
	}
	return false
}

// Parse parses s into a pattern template, and binds the specified pattern
// variables to the corresponding expressions.
func Parse(s string, binds []Bind) (*P, error) {
	lit, pat, err := parse(s)
	if err != nil {
		return nil, err
	}
	var parts []string
	rules := make(map[string]string)
	for i, part := range lit {
		parts = append(parts, part)
		if i < len(pat) {
			parts = append(parts, pat[i])
			rules[pat[i]] = ""
		}
	}
	p := &P{template: s, parts: parts, rules: mergeBinds(rules, binds)}
	return p, nil
}

// Bind returns a copy of p with the specified bindings updated.  Existing
// bindings of p not mentioned in binds are copied intact from p to the result.
func (p *P) Bind(binds Binds) *P {
	return &P{
		template: p.template,
		parts:    p.parts,
		rules:    mergeBinds(p.rules, binds),
	}
}

// MustParse parses s into a pattern template, as Parse, but panics if parsing
// fails. This function exists to support static initialization.
func MustParse(s string, binds []Bind) *P {
	p, err := Parse(s, binds)
	if err != nil {
		panic("pattern: " + err.Error())
	}
	return p
}

func isWordRune(c rune) bool {
	switch {
	case c == '_', c == '-', c == '+', c == '/', c == ':', c == '=', c == '#':
		return true
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	}
	return false
}

// parse verifies the grammar of s, returning a slice of literals and a
// corresponding slice of pattern labels.
func parse(s string) (lit, pat []string, _ error) {
	const (
		free   = iota // in literal text
		dollar        // saw a $, looking for $ or {
		word          // in a pattern word
	)

	start := 0           // start of most recent pattern word ($)
	st := free           // lexer state
	var buf bytes.Buffer // current token
	for i, c := range s {
		switch st {
		case free:
			if c == '$' {
				start = i
				st = dollar
			} else {
				buf.WriteRune(c)
			}

		case dollar:
			if c == '$' {
				buf.WriteRune(c)
				st = free // escaped $
			} else if c == '{' {
				lit = append(lit, buf.String())
				buf.Reset()
				st = word
			} else {
				return nil, nil, perrorf(i, "wanted $ or { but found '%c'", c)
			}

		case word:
			if c == '}' {
				if buf.Len() == 0 {
					return nil, nil, perrorf(start, "empty pattern word")
				}
				pat = append(pat, buf.String())
				buf.Reset()
				st = free
			} else if !isWordRune(c) {
				return nil, nil, perrorf(i, "invalid name letter '%c'", c)
			} else {
				buf.WriteRune(c)
			}
		}
	}
	if buf.Len() > 0 {
		lit = append(lit, buf.String())
	}
	switch st {
	case dollar:
		return nil, nil, perrorf(start, "incomplete $ escape")
	case word:
		return nil, nil, perrorf(start, "incomplete pattern word")
	}
	return lit, pat, nil
}

// bindMatches extracts bindings from needle corresponding to the named capture
// groups of re, given the submatch indices in m.
func bindMatches(re *regexp.Regexp, m []int, needle string) Binds {
	var binds []Bind
	for i, name := range re.SubexpNames() {
		a, b := m[2*i], m[2*i+1]
		if name == "" || a < 0 {
			continue
		}
		binds = append(binds, Bind{
			Name: name,
			Expr: needle[a:b],
		})
	}
	return binds
}

// mergeBinds returns a copy of old into which the given binds are merged.  The
// result has the same keys as old, and the values for keys not mentioned in
// binds are copied from old.
func mergeBinds(old map[string]string, binds Binds) map[string]string {
	rules := make(map[string]string)
	for key, val := range old {
		rules[key] = val
	}
	for _, bind := range binds {
		if _, ok := rules[bind.Name]; ok {
			rules[bind.Name] = bind.Expr
		}
		// ignore bindings that do not apply
	}
	return rules
}

// ParseError is the concrete type of parsing errors.
type ParseError struct {
	Pos     int    // offset where error occurred
	Message string // description of error
}

func (p *ParseError) Error() string { return fmt.Sprintf("at %d: %s", p.Pos, p.Message) }

func perrorf(pos int, msg string, args ...interface{}) *ParseError {
	return &ParseError{pos, fmt.Sprintf(msg, args...)}
}
