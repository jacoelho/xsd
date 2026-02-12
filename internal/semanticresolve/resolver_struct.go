package semanticresolve

import (
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolveguard"
	model "github.com/jacoelho/xsd/internal/types"
)

// Resolver resolves all QName references in a schema.
// Runs exactly once after parsing. Detects cycles during resolution.
type Resolver struct {
	schema *parser.Schema

	// Cycle detection during resolution (cleared after resolution)
	detector *CycleDetector[model.QName]

	// Pointer-based tracking for anonymous types (which have empty QNames) to
	// avoid false cycle matches while still detecting self-references.
	anonymousTypeGuard *resolveguard.Pointer[model.Type]
}

// NewResolver creates a new resolver for the given schema.
func NewResolver(sch *parser.Schema) *Resolver {
	return &Resolver{
		schema:             sch,
		detector:           NewCycleDetector[model.QName](),
		anonymousTypeGuard: resolveguard.NewPointer[model.Type](),
	}
}
