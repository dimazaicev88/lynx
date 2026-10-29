// Command lynxc compiles Lynx source files to JavaScript.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"lynx/internal/compiler"
	"lynx/internal/diag"
)

// Version can be overridden at build time with -ldflags "-X main.Version=...".
var Version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lynxc", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var output string
	var checkOnly bool
	var showVersion bool

	flags.StringVar(&output, "o", "", "write JavaScript to this file instead of stdout")
	flags.BoolVar(&checkOnly, "check", false, "type-check only, do not emit JavaScript")
	flags.BoolVar(&showVersion, "version", false, "print the compiler version")

	flags.Usage = func() {
		fmt.Fprintln(stderr, "lynxc - compile Lynx to JavaScript")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  lynxc [options] <file.lynx>")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Options:")
		flags.PrintDefaults()
	}

	if err := flags.Parse(reorderArgs(args)); err != nil {
		return 2
	}
	if showVersion {
		fmt.Fprintln(stdout, "lynxc "+Version)
		return 0
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}

	filename := flags.Arg(0)
	var src []byte
	var err error

	if filename == "-" {
		filename = "<stdin>"
		src, err = io.ReadAll(stdin)
	} else {
		src, err = os.ReadFile(filename)
	}
	if err != nil {
		fmt.Fprintf(stderr, "lynxc: %v\n", err)
		return 1
	}

	js, err := compiler.Compile(filename, src)
	if err != nil {
		fmt.Fprintln(stderr, diag.Format(err, src))
		return 1
	}

	if checkOnly {
		return 0
	}

	if output == "" {
		if _, err := io.WriteString(stdout, js); err != nil {
			fmt.Fprintf(stderr, "lynxc: %v\n", err)
			return 1
		}
		return 0
	}

	if err := os.WriteFile(output, []byte(js), 0o644); err != nil {
		fmt.Fprintf(stderr, "lynxc: %v\n", err)
		return 1
	}
	return 0
}

// reorderArgs moves options before positional arguments so that callers may
// write "lynxc file.lynx -o out.js" as well as "lynxc -o out.js file.lynx".
func reorderArgs(args []string) []string {
	options := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case arg == "-":
			positional = append(positional, arg)
		case arg == "-o" || arg == "--o":
			options = append(options, arg)
			if i+1 < len(args) {
				i++
				options = append(options, args[i])
			}
		case strings.HasPrefix(arg, "-o=") || strings.HasPrefix(arg, "--o="):
			options = append(options, arg)
		case strings.HasPrefix(arg, "-"):
			options = append(options, arg)
		default:
			positional = append(positional, arg)
		}
	}

	return append(options, positional...)
}
