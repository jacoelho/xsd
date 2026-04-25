package compiler

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/schemair"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// Root identifies one schema root document.
type Root struct {
	FS       fs.FS
	Resolver SchemaResolver
	Location string
}

// LoadConfig configures schema load and normalization.
type LoadConfig struct {
	Roots                       []Root
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// BuildConfig configures runtime compilation.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// Prepared stores normalized artifacts used to build immutable runtime schemas.
type Prepared struct {
	ir *schemair.Schema
}
