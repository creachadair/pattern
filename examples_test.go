package pattern_test

import (
	"fmt"
	"log"

	"github.com/creachadair/pattern"
)

func ExampleParse() {
	// A pattern consists of a template string containing pattern words, and a
	// set of bindings that give regular expressions that each word must match.
	p, err := pattern.Parse(`Grade: ${grade}`, pattern.Binds{
		{Name: "grade", Expr: `([ABCD][-+]?|[EF])`},
	})
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}

	// The string representation of a pattern is the original template string
	// from which it was parsed.
	fmt.Println(p)

	// Output:
	// Grade: ${grade}
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

func ExampleP_Search() {
	p := pattern.MustParse("${word}:", pattern.Binds{
		{Name: "word", Expr: "\\w+"},
	})

	const text = `
Do: a deer, a female deer, Re: a drop of golden sun.
Mi: a name I call myself, Fa: a long long way to run.
`
	if err := p.Search(text, func(i, j int, m pattern.Binds) error {
		fmt.Printf("At %d: %q\n", i, m.First("word"))
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	// Output:
	// At 1: "Do"
	// At 28: "Re"
	// At 54: "Mi"
	// At 80: "Fa"
}
