package parser

import "github.com/jacoelho/xsd/internal/types"

// Form represents qualified/unqualified element/attribute forms
type Form int

const (
	// Unqualified indicates an unqualified element/attribute form.
	Unqualified Form = iota
	// Qualified indicates a qualified element/attribute form.
	Qualified
)

// GlobalDeclKind identifies top-level schema declarations in document order.
type GlobalDeclKind int

const (
	// GlobalDeclElement represents a top-level element declaration.
	GlobalDeclElement GlobalDeclKind = iota
	// GlobalDeclType represents a top-level type declaration (complex or simple).
	GlobalDeclType
	// GlobalDeclAttribute represents a top-level attribute declaration.
	GlobalDeclAttribute
	// GlobalDeclAttributeGroup represents a top-level attributeGroup declaration.
	GlobalDeclAttributeGroup
	// GlobalDeclGroup represents a top-level model group declaration.
	GlobalDeclGroup
	// GlobalDeclNotation represents a top-level notation declaration.
	GlobalDeclNotation
)

// GlobalDecl tracks a top-level declaration in document order.
type GlobalDecl struct {
	Name types.QName
	Kind GlobalDeclKind
}

// Schema represents a compiled XSD schema
type Schema struct {
	ImportContexts          map[string]ImportContext
	Groups                  map[types.QName]*types.ModelGroup
	ElementOrigins          map[types.QName]string
	TypeDefs                map[types.QName]types.Type
	TypeOrigins             map[types.QName]string
	AttributeDecls          map[types.QName]*types.AttributeDecl
	SubstitutionGroups      map[types.QName][]types.QName
	AttributeGroups         map[types.QName]*types.AttributeGroup
	AttributeGroupOrigins   map[types.QName]string
	ImportedNamespaces      map[types.NamespaceURI]map[types.NamespaceURI]bool
	ElementDecls            map[types.QName]*types.ElementDecl
	GroupOrigins            map[types.QName]string
	AttributeOrigins        map[types.QName]string
	NotationOrigins         map[types.QName]string
	NotationDecls           map[types.QName]*types.NotationDecl
	ParticleRestrictionCaps map[*types.ElementDecl]types.Occurs
	IDAttributes            map[string]string
	NamespaceDecls          map[string]string
	Location                string
	TargetNamespace         types.NamespaceURI
	GlobalDecls             []GlobalDecl
	FinalDefault            types.DerivationSet
	AttributeFormDefault    Form
	ElementFormDefault      Form
	BlockDefault            types.DerivationSet
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		ElementDecls:            make(map[types.QName]*types.ElementDecl),
		ElementOrigins:          make(map[types.QName]string),
		TypeDefs:                make(map[types.QName]types.Type),
		TypeOrigins:             make(map[types.QName]string),
		AttributeDecls:          make(map[types.QName]*types.AttributeDecl),
		AttributeOrigins:        make(map[types.QName]string),
		AttributeGroups:         make(map[types.QName]*types.AttributeGroup),
		AttributeGroupOrigins:   make(map[types.QName]string),
		Groups:                  make(map[types.QName]*types.ModelGroup),
		GroupOrigins:            make(map[types.QName]string),
		SubstitutionGroups:      make(map[types.QName][]types.QName),
		NotationDecls:           make(map[types.QName]*types.NotationDecl),
		NotationOrigins:         make(map[types.QName]string),
		NamespaceDecls:          make(map[string]string),
		IDAttributes:            make(map[string]string),
		ParticleRestrictionCaps: make(map[*types.ElementDecl]types.Occurs),
		ImportedNamespaces:      make(map[types.NamespaceURI]map[types.NamespaceURI]bool),
		ImportContexts:          make(map[string]ImportContext),
		GlobalDecls:             []GlobalDecl{},
	}
}

func (s *Schema) addGlobalDecl(kind GlobalDeclKind, name types.QName) {
	s.GlobalDecls = append(s.GlobalDecls, GlobalDecl{Kind: kind, Name: name})
}

// ImportContext tracks import namespaces for a specific schema document.
type ImportContext struct {
	Imports         map[types.NamespaceURI]bool
	TargetNamespace types.NamespaceURI
}
