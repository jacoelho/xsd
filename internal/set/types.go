package set

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// PrepareConfig configures schema load and normalization.
type PrepareConfig struct {
	FS                          fs.FS
	Location                    string
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// CompileConfig configures runtime compilation.
type CompileConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// PreparedSchema stores normalized artifacts and lazy build state.
type PreparedSchema struct {
	prepared *compiler.Prepared
}
