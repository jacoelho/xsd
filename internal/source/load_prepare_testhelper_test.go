package source

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
)

func loadAndPrepare(t *testing.T, loader *SchemaLoader, location string) (*parser.Schema, error) {
	t.Helper()
	sch, err := loader.Load(location)
	if err != nil {
		return nil, err
	}
	validated, err := pipeline.Validate(sch)
	if err != nil {
		return nil, err
	}
	// ensure transform-phase reference checks run for source integration tests.
	if _, err := pipeline.Transform(validated); err != nil {
		return nil, err
	}
	// source tests assert resolved parser-model fields directly.
	// use validated artifact snapshots instead of mutating parse-phase inputs.
	resolved, err := validated.SchemaSnapshot()
	if err != nil {
		return nil, err
	}
	return resolved, nil
}
