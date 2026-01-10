package grammar

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// CompiledSchema is the fully-resolved, validated schema ready for schemacheck.
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
	// ConstraintDeclsByQName maps element QNames to the elements that declare
	// identity constraints affecting that QName (including substitution groups).
	ConstraintDeclsByQName map[types.QName][]*CompiledElement

	// Schema-level defaults
	ElementFormDefault   parser.Form
	AttributeFormDefault parser.Form
	BlockDefault         types.DerivationSet
	FinalDefault         types.DerivationSet
}
