package pattern_test

import (
	"fmt"
	"log"

	"bitbucket.org/creachadair/pattern"
)

func Example() {
	// A pattern consists of a template string containing pattern words, and a
	// set of bindings that give regular expressions that each word must match.
	p := pattern.MustParse(`Grade: ${grade}`, pattern.Binds{
		{Name: "grade", Expr: `([ABCD][-+]?|[EF])`},
	})

	// Matching verifies that a needle matches the template string with the
	// pattern expressions interpolated. If so, it returns a list of bindings
	// that give the matching substrings of the needle.
	m, err := p.Match("Grade: B+")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Bindings:")
	for _, bind := range m {
		fmt.Println(bind.Name, "=", bind.Expr)
	}

	// In addition, you can substitute bindings back into a pattern to produce
	// a new string. Bindings not mentioned by p are ignored, but all the
	// bindings that p uses must be provided.
	s, err := p.Apply(pattern.Binds{{
		Name: "grade",
		Expr: "A-",
	}})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nApplied:")
	fmt.Println(s)

	// Output:
	// Bindings:
	// grade = B+
	//
	// Applied:
	// Grade: A-
}

func Example_roundTrip() {
	p := pattern.MustParse(`git@${host}:${user}/${repo}.git`, pattern.Binds{
		{Name: "host", Expr: `\w+(\.\w+)*`},
		{Name: "user", Expr: `\w+`},
		{Name: "repo", Expr: `\w+`},
	})

	const input = `git@bitbucket.org:creachadair/stringset.git`
	m, err := p.Match(input)
	if err != nil {
		log.Fatalln("Match:", err)
	}

	// Apply the bindings to a derived pattern.
	d, err := p.Derive(`http://${host}/${user}/${repo}`)
	if err != nil {
		log.Fatalln("Derive:", err)
	}
	do, err := d.Apply(m)
	if err != nil {
		log.Fatalln("Apply:", err)
	}

	// Applying the same bindings to the original pattern should recover the
	// input string.
	oo, err := p.Apply(m)
	if err != nil {
		log.Fatalln("Apply:", err)
	}

	fmt.Println(" derived: ", do)
	fmt.Println(" original:", oo, oo == input)
	// Output:
	//  derived:  http://bitbucket.org/creachadair/stringset
	//  original: git@bitbucket.org:creachadair/stringset.git true
}
