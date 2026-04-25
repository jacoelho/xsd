package schemair

import (
	"fmt"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

// ResolveConfig configures AST-to-IR resolution.
type ResolveConfig struct{}

func Resolve(docs *ast.DocumentSet, _ ResolveConfig) (*Schema, error) {
	if docs == nil {
		return nil, fmt.Errorf("schema ir: document set is nil")
	}
	return resolveDocuments(docs)
}
