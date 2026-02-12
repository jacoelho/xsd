package compiler

import (
	"github.com/jacoelho/xsd/internal/normalize"
	"github.com/jacoelho/xsd/internal/parser"
)

// Prepare clones, normalizes, and validates a parsed schema.
func Prepare(parsed *parser.Schema) (*Prepared, error) {
	artifacts, err := normalize.Prepare(parsed)
	if err != nil {
		return nil, err
	}
	return &Prepared{artifacts: artifacts}, nil
}

// PrepareOwned normalizes and validates a parsed schema in place.
func PrepareOwned(parsed *parser.Schema) (*Prepared, error) {
	artifacts, err := normalize.PrepareOwned(parsed)
	if err != nil {
		return nil, err
	}
	return &Prepared{artifacts: artifacts}, nil
}
