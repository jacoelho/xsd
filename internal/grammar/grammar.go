package grammar

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// CompiledSchema is the fully-resolved, validated schema ready for schemacheck.
// All references are resolved to direct pointers. No QName lookups needed.
type CompiledSchema struct {
	SubstitutionGroups      map[types.QName][]*CompiledElement
	Elements                map[types.QName]*CompiledElement
	Types                   map[types.QName]*CompiledType
	Attributes              map[types.QName]*CompiledAttribute
	NotationDecls           map[types.QName]*types.NotationDecl
	LocalElements           map[types.QName]*CompiledElement
	ConstraintDeclsByQName  map[types.QName][]*CompiledElement
	TargetNamespace         types.NamespaceURI
	ElementsWithConstraints []*CompiledElement
	ElementFormDefault      parser.Form
	AttributeFormDefault    parser.Form
	BlockDefault            types.DerivationSet
	FinalDefault            types.DerivationSet
}
