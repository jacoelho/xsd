package xsd

import (
	"github.com/jacoelho/xsd/internal/compiler"
)

func prepareEntries(entries []schemaSetEntry, load resolvedLoadOptions) (*compiler.Prepared, error) {
	roots := make([]compiler.Root, 0, len(entries))
	for _, entry := range entries {
		roots = append(roots, compiler.Root{
			FS:       entry.fsys,
			Location: entry.location,
		})
	}
	return compiler.PrepareRoots(compiler.LoadConfig{
		Roots:                       roots,
		AllowMissingImportLocations: load.allowMissingImportLocations,
		SchemaParseOptions:          load.schemaLimits.options(),
	})
}
