// Package main implements an xmllint-compatible validation CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jacoelho/xsd"
)

type config struct {
	schema    string
	doc       string
	noout     bool
	huge      bool
	maxErrors int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	engine, err := xsd.Compile(xsd.File(cfg.schema))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s fails to compile\n%v\n", cfg.schema, err)
		return 1
	}
	f, err := os.Open(cfg.doc)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s fails to validate\n%v\n", cfg.doc, err)
		return 1
	}
	defer func() { _ = f.Close() }()
	if err := engine.ValidateWithOptions(f, xsd.ValidateOptions{MaxErrors: cfg.maxErrors}); err != nil {
		printValidationErrors(stderr, err)
		_, _ = fmt.Fprintf(stderr, "%s fails to validate\n", cfg.doc)
		return 1
	}
	_, _ = fmt.Fprintf(stderr, "%s validates\n", cfg.doc)
	return 0
}

func parseArgs(args []string) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("xmllint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&cfg.noout, "noout", false, "suppress document output")
	fs.BoolVar(&cfg.huge, "huge", false, "accepted for xmllint compatibility")
	fs.IntVar(&cfg.maxErrors, "max-errors", 0, "maximum validation errors to collect")
	fs.StringVar(&cfg.schema, "schema", "", "schema path")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.schema == "" {
		return cfg, errors.New("--schema is required")
	}
	if fs.NArg() != 1 {
		return cfg, errors.New("one XML document path is required")
	}
	cfg.doc = fs.Arg(0)
	return cfg, nil
}

func printValidationErrors(w io.Writer, err error) {
	if errs, ok := err.(xsd.Errors); ok {
		for _, child := range errs {
			_, _ = fmt.Fprintln(w, child)
		}
		return
	}
	_, _ = fmt.Fprintln(w, err)
}
