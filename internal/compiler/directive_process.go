package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// Status reports the outcome of loading one directive target.
type Status uint8

const (
	StatusLoaded Status = iota
	StatusDeferred
	StatusSkippedMissing
)

// LoadResult carries the schema and target key produced by one directive load.
type LoadResult[K comparable] struct {
	Schema *parser.Schema
	Target K
	Status Status
}

// ImportConfig describes the root-owned callbacks needed to process one import.
type ImportConfig[K comparable] struct {
	Load                 func(parser.ImportInfo) (LoadResult[K], error)
	Merge                func(*parser.Schema, K) error
	Info                 parser.ImportInfo
	AllowMissingLocation bool
}

// IncludeConfig describes the root-owned callbacks needed to process one include.
type IncludeConfig[K comparable] struct {
	Load  func(parser.IncludeInfo) (LoadResult[K], error)
	Merge func(*parser.Schema, K) error
	Info  parser.IncludeInfo
}

// Process dispatches directives in document order.
func Process(
	directives []parser.Directive,
	onInclude func(parser.IncludeInfo) error,
	onImport func(parser.ImportInfo) error,
) error {
	for _, directive := range directives {
		switch directive.Kind {
		case parser.DirectiveInclude:
			if err := onInclude(directive.Include); err != nil {
				return err
			}
		case parser.DirectiveImport:
			if err := onImport(directive.Import); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected directive kind: %d", directive.Kind)
		}
	}
	return nil
}

// ProcessImport handles the generic control flow for one import directive.
func ProcessImport[K comparable](cfg ImportConfig[K]) error {
	if cfg.Info.SchemaLocation == "" {
		if cfg.AllowMissingLocation {
			return nil
		}
		return fmt.Errorf("import missing schemaLocation")
	}

	result, err := cfg.Load(cfg.Info)
	if err != nil {
		return fmt.Errorf("load imported schema %s: %w", cfg.Info.SchemaLocation, err)
	}
	switch result.Status {
	case StatusDeferred, StatusSkippedMissing:
		return nil
	case StatusLoaded:
		return cfg.Merge(result.Schema, result.Target)
	default:
		return fmt.Errorf("unexpected import load status: %d", result.Status)
	}
}

// ProcessInclude handles the generic control flow for one include directive.
func ProcessInclude[K comparable](cfg IncludeConfig[K]) error {
	result, err := cfg.Load(cfg.Info)
	if err != nil {
		return fmt.Errorf("load included schema %s: %w", cfg.Info.SchemaLocation, err)
	}
	switch result.Status {
	case StatusDeferred:
		return nil
	case StatusSkippedMissing:
		return fmt.Errorf("included schema %s not found", cfg.Info.SchemaLocation)
	case StatusLoaded:
		return cfg.Merge(result.Schema, result.Target)
	default:
		return fmt.Errorf("unexpected include load status: %d", result.Status)
	}
}
