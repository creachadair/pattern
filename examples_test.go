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

func ExampleP_Apply() {
	p := pattern.MustParse(`type ${name} struct {
  ${lhs} int
  ${rhs} int
}`, nil)

	s, err := p.Apply(pattern.Binds{
		{Name: "name", Expr: "binop"},
		{Name: "lhs", Expr: "X"},
		{Name: "rhs", Expr: "Y"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(s)
	// Output:
	// type binop struct {
	//   X int
	//   Y int
	// }
}

func ExampleP_ApplyFunc() {
	p := pattern.MustParse(`type ${name} struct {
  ${arg} string `+"`json:\"${arg},omitempty\"`"+`
}`, nil)

	s, err := p.ApplyFunc(func(name string, n int) (string, error) {
		if name == "name" {
			return "Argument", nil
		} else if n == 1 {
			return "X", nil
		}
		return "value", nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(s)
	// Output:
	// type Argument struct {
	//   X string `json:"value,omitempty"`
	// }
}

func ExampleP_Match() {
	p := pattern.MustParse(`[${text}](${link})`, pattern.Binds{
		{Name: "text", Expr: ".+"},
		{Name: "link", Expr: "\\S+"},
	})

	m, err := p.Match(`[docs](http://godoc.org/net/url)`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("link text: %s\n", m.First("text"))
	fmt.Printf("URL: %s\n", m.First("link"))

	// Output:
	// link text: docs
	// URL: http://godoc.org/net/url
}
