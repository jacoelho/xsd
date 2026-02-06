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
	if _, err := pipeline.Prepare(sch); err != nil {
		return nil, err
	}
	return sch, nil
}
