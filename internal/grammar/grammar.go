package grammar

import (
	"io/fs"

	internal "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// CompiledSchema is the fully-resolved, validated schema ready for validation.
// All references are resolved to direct pointers. No QName lookups needed.
type CompiledSchema struct {
	TargetNamespace types.NamespaceURI

	// Direct lookup tables (QName -> compiled component)
	Elements      map[types.QName]*CompiledElement
	Types         map[types.QName]*CompiledType
	Attributes    map[types.QName]*CompiledAttribute
	NotationDecls map[types.QName]*types.NotationDecl

	// LocalElements maps QNames to local (non-top-level) element declarations.
	// Precomputed during compilation for XPath evaluation in identity constraints.
	LocalElements map[types.QName]*CompiledElement

	// Substitution groups: head -> all valid substitutes (transitive closure)
	SubstitutionGroups map[types.QName][]*CompiledElement

	// ElementsWithConstraints contains all elements (top-level and local) that have
	// identity constraints. Precomputed during compilation for O(1) access at validation time.
	ElementsWithConstraints []*CompiledElement

	// Schema-level defaults
	ElementFormDefault   internal.Form
	AttributeFormDefault internal.Form
	BlockDefault         types.DerivationSet
	FinalDefault         types.DerivationSet

	// SourceFS provides optional filesystem access for schemaLocation hints.
	SourceFS fs.FS
	// BasePath is an optional base path for resolving schemaLocation entries.
	BasePath string
}
