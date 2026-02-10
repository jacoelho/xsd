package source

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/schemaprep"
)

func loadAndPrepare(t *testing.T, loader *SchemaLoader, location string) (*parser.Schema, error) {
	t.Helper()
	sch, err := loader.Load(location)
	if err != nil {
		return nil, err
	}
	if _, err := pipeline.Prepare(sch); err != nil {
		return nil, err
	}
	// source tests assert resolved parser-model fields directly.
	// prepare() returns runtime artifacts, so build a resolved schema snapshot for assertions.
	resolved, err := schemaprep.ResolveAndValidate(sch)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}
