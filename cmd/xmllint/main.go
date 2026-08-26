// Package main implements an xmllint-style validation CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type config struct {
	schema             string
	doc                string
	maxErrors          int
	maxIdentityEntries int
	maxBytes           int64
}

const usage = "Usage: xmllint --schema PATH [--max-errors N] [--max-identity-entries N] [--max-instance-bytes N] XML\n"

func main() {
	code := runWithOpen(os.Args[1:], os.Stdout, os.Stderr, openDocument)
	os.Exit(code)
}

func openDocument(path string) (io.ReadCloser, error) {
	f, err := os.Open(path) //nolint:gosec // xmllint intentionally validates caller-provided document paths.
	if err != nil {
		return nil, err
	}
	return f, nil
}

func runWithOpen(args []string, stdout, stderr io.Writer, openDoc func(string) (io.ReadCloser, error)) int {
	cfg, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		return writeStatus(stdout, 0, usage)
	}
	if err != nil {
		return writeStatus(stderr, 2, "%v\n", err)
	}
	engine, err := xsd.Compile(xsd.File(cfg.schema))
	if err != nil {
		return writeStatus(stderr, 1, "%s fails to compile\n%v\n", cfg.schema, err)
	}
	f, err := openDoc(cfg.doc)
	if isNilReadCloser(f) {
		if err == nil {
			err = errors.New("document opener returned nil reader")
		}
		return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, err)
	}
	if err != nil {
		err = errors.Join(err, f.Close())
		return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, err)
	}
	validationErr := engine.ValidateWithOptions(f, xsd.ValidateOptions{
		MaxErrors:          cfg.maxErrors,
		MaxIdentityEntries: cfg.maxIdentityEntries,
		MaxInstanceBytes:   cfg.maxBytes,
	})
	closeErr := f.Close()
	if validationErr != nil {
		if writeErr := printValidationErrors(stderr, validationErr); writeErr != nil {
			return 1
		}
		if closeErr != nil {
			return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, closeErr)
		}
		return writeStatus(stderr, 1, "%s fails to validate\n", cfg.doc)
	}
	if closeErr != nil {
		return writeStatus(stderr, 1, "%s fails to validate\n%v\n", cfg.doc, closeErr)
	}
	return 0
}

func isNilReadCloser(r io.ReadCloser) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func writeStatus(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return 1
	}
	return code
}

func parseArgs(args []string) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("xmllint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&cfg.maxErrors, "max-errors", 0, "maximum validation errors to collect")
	fs.IntVar(&cfg.maxIdentityEntries, "max-identity-entries", 0, "maximum retained identity entries")
	fs.Int64Var(&cfg.maxBytes, "max-instance-bytes", 0, "maximum raw XML bytes to read")
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
	if cfg.maxIdentityEntries < 0 {
		return cfg, errors.New("--max-identity-entries cannot be negative")
	}
	if cfg.maxBytes < 0 {
		return cfg, errors.New("--max-instance-bytes cannot be negative")
	}
	if fs.NArg() != 1 {
		return cfg, errors.New("one XML document path is required")
	}
	cfg.doc = fs.Arg(0)
	return cfg, nil
}

func printValidationErrors(w io.Writer, err error) error {
	for _, child := range xsderrors.Flatten(err) {
		if _, writeErr := fmt.Fprintln(w, child); writeErr != nil {
			return writeErr
		}
	}
	return nil
}
