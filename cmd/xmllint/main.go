// Package main implements an xmllint-compatible validation CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type config struct {
	schema    string
	doc       string
	maxErrors int
}

func main() {
	os.Exit(runWithOpen(os.Args[1:], os.Stderr, func(path string) (io.ReadCloser, error) {
		return os.Open(path) //nolint:gosec // xmllint intentionally validates caller-provided document paths.
	}))
}

func runWithOpen(args []string, stderr io.Writer, openDoc func(string) (io.ReadCloser, error)) int {
	cfg, err := parseArgs(args)
	if err != nil {
		return writeStatus(stderr, 2, "%v\n", err)
	}
	engine, err := xsd.Compile(xsd.File(cfg.schema))
	if err != nil {
		return writeStatus(stderr, 1, "%s fails to compile\n%v\n", cfg.schema, err)
	}
	f, err := openDoc(cfg.doc)
	if err != nil {
		return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, err)
	}
	validationErr := engine.ValidateWithOptions(f, xsd.ValidateOptions{MaxErrors: cfg.maxErrors})
	closeErr := f.Close()
	if validationErr != nil {
		if writeErr := printValidationErrors(stderr, validationErr); writeErr != nil {
			return 2
		}
		if closeErr != nil {
			return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, closeErr)
		}
		return writeStatus(stderr, 1, "%s fails to validate\n", cfg.doc)
	}
	if closeErr != nil {
		return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, closeErr)
	}
	return writeStatus(stderr, 0, "%s validates\n", cfg.doc)
}

func writeStatus(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return 2
	}
	return code
}

func parseArgs(args []string) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("xmllint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("noout", false, "suppress document output")
	fs.Bool("huge", false, "accepted for xmllint compatibility")
	fs.IntVar(&cfg.maxErrors, "max-errors", 0, "maximum validation errors to collect")
	fs.StringVar(&cfg.schema, "schema", "", "schema path")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.schema == "" {
		return cfg, errors.New("--schema is required")
	}
	if cfg.maxErrors < 0 {
		return cfg, errors.New("--max-errors cannot be negative")
	}
	if fs.NArg() != 1 {
		return cfg, errors.New("one XML document path is required")
	}
	cfg.doc = fs.Arg(0)
	return cfg, nil
}

func printValidationErrors(w io.Writer, err error) error {
	var errs xsderrors.Errors
	if errors.As(err, &errs) {
		for _, child := range errs {
			if _, writeErr := fmt.Fprintln(w, child); writeErr != nil {
				return writeErr
			}
		}
		return nil
	}
	_, writeErr := fmt.Fprintln(w, err)
	return writeErr
}
