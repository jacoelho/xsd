package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func includeInsertIndex(entry *schemaEntry, include parser.IncludeInfo, currentDecls int) (int, error) {
	if entry == nil {
		return 0, fmt.Errorf("include tracking entry is nil")
	}
	if include.IncludeIndex < 0 || include.IncludeIndex >= len(entry.includeInserted) {
		return 0, fmt.Errorf("include index %d out of range", include.IncludeIndex)
	}
	if include.DeclIndex < 0 {
		return 0, fmt.Errorf("include decl index %d out of range", include.DeclIndex)
	}
	offset := 0
	for i := 0; i < include.IncludeIndex; i++ {
		offset += entry.includeInserted[i]
	}
	insertAt := include.DeclIndex + offset
	if insertAt > currentDecls {
		return 0, fmt.Errorf("include position %d out of range (decls=%d)", insertAt, currentDecls)
	}
	return insertAt, nil
}

func recordIncludeInserted(entry *schemaEntry, includeIndex, count int) error {
	if entry == nil {
		return fmt.Errorf("include tracking entry is nil")
	}
	if includeIndex < 0 || includeIndex >= len(entry.includeInserted) {
		return fmt.Errorf("include index %d out of range", includeIndex)
	}
	if count < 0 {
		return fmt.Errorf("include inserted count %d out of range", count)
	}
	entry.includeInserted[includeIndex] += count
	return nil
}
