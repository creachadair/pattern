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

func Example_transform() {
	const lhs = `git@${host}:${user}/${repo}.git`
	const rhs = `http://${host}/${user}/${repo}`

	t := pattern.MustTransform(lhs, rhs, pattern.Binds{
		{Name: "host", Expr: `\w+(\.\w+)*`},
		{Name: "user", Expr: `\w+`},
		{Name: "repo", Expr: `\w+`},
	})

	const input = `git@bitbucket.org:creachadair/stringset.git`
	fmt.Println("input:", input)

	// Forward transform the input to get output.
	output, err := t.Apply(input)
	if err != nil {
		log.Fatalf("Forward: %v", err)
	}
	fmt.Println("output:", output)

	// Reverse transform the output to recover the input.
	check, err := t.Reverse().Apply(output)
	if err != nil {
		log.Fatalf("Reverse: %v", err)
	}
	fmt.Printf("check == input: %v", check == input)

	// Output:
	// input: git@bitbucket.org:creachadair/stringset.git
	// output: http://bitbucket.org/creachadair/stringset
	// check == input: true
}
