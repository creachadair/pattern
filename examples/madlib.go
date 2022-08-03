// Program madlib is a lighthearted demonstration program for how to use the
// github.com/creachadair/pattern package.
//
// Usage:
//
//	madlib <input>
//
// The input file is a text template with ${pattern} words imbedded.  After
// reading the file, the program prompts the user to fill in values for each of
// the pattern words, and then the results are substituted into the template to
// populate the lib.
//
// The values for pattern words with a leading capital letter are capitalized.
// The pattern word is its own prompt, but with spaces for underscores.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/pattern"
)

var in = bufio.NewScanner(os.Stdin)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s <libfile>", filepath.Base(os.Args[0]))
	}

	// Read and parse the input template.
	lib, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("Reading lib: %v", err)
	}
	pat, err := pattern.Parse(string(lib), nil)
	if err != nil {
		log.Fatalf("Parsing lib: %v", err)
	}

	// Query the user for values to fill the bindings.
	binds := pat.Binds()
	for i, bind := range binds {
		req := strings.Join(strings.Split(bind.Name, "_"), " ")
		rsp, err := prompt(fmt.Sprintf("(%d) %s", i+1, req))
		if err != nil {
			log.Fatalf("Input interrupted: %v", err)
		}
		binds[i].Expr = format(bind.Name, rsp)
	}

	filled, err := pat.Apply(binds)
	if err != nil {
		log.Fatalf("Filling lib: %v", err)
	}
	fmt.Println(filled)
}

// prompt prints s as a prompt to stderr and reads a single non-empty line of
// text from stdin.
func prompt(s string) (string, error) {
	for {
		fmt.Fprint(os.Stderr, strings.ToLower(s), ": ")
		if in.Scan() {
			if in.Text() == "" {
				fmt.Fprintln(os.Stderr, "Please enter a non-empty string")
				continue
			}
			return in.Text(), nil
		}
		return "", in.Err()
	}
}

// format renders value, capitalizing its initial letter if name has its
// initial letter capitalized.
func format(name, value string) string {
	if p := name[:1]; p == strings.ToUpper(p) {
		return strings.ToUpper(value[:1]) + value[1:]
	}
	return value
}
