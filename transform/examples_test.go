package transform_test

import (
	"fmt"
	"log"

	"bitbucket.org/creachadair/pattern"
	"bitbucket.org/creachadair/pattern/transform"
)

func Example() {
	const lhs = `git@${host}:${user}/${repo}.git`
	const rhs = `http://${host}/${user}/${repo}`

	t := transform.MustReversible(transform.New(lhs, rhs, pattern.Binds{
		{Name: "host", Expr: `\w+(\.\w+)*`},
		{Name: "user", Expr: `\w+`},
		{Name: "repo", Expr: `\w+`},
	}))

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
