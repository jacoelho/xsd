package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/schemair"
)

// Prepare resolves parse-only schema documents into immutable IR.
func Prepare(docs *schemaast.DocumentSet) (*Prepared, error) {
	ir, err := schemair.Resolve(docs, schemair.ResolveConfig{})
	if err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	return &Prepared{ir: ir}, nil
}
