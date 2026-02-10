package parser

import "github.com/jacoelho/xsd/internal/model"

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
	Name model.QName
	Kind GlobalDeclKind
}

// Schema represents a compiled XSD schema
type Schema struct {
	ImportContexts        map[string]ImportContext
	Groups                map[model.QName]*model.ModelGroup
	ElementOrigins        map[model.QName]string
	TypeDefs              map[model.QName]model.Type
	TypeOrigins           map[model.QName]string
	AttributeDecls        map[model.QName]*model.AttributeDecl
	SubstitutionGroups    map[model.QName][]model.QName
	AttributeGroups       map[model.QName]*model.AttributeGroup
	AttributeGroupOrigins map[model.QName]string
	ImportedNamespaces    map[model.NamespaceURI]map[model.NamespaceURI]bool
	ElementDecls          map[model.QName]*model.ElementDecl
	GroupOrigins          map[model.QName]string
	AttributeOrigins      map[model.QName]string
	NotationOrigins       map[model.QName]string
	NotationDecls         map[model.QName]*model.NotationDecl
	IDAttributes          map[string]string
	NamespaceDecls        map[string]string
	Location              string
	TargetNamespace       model.NamespaceURI
	GlobalDecls           []GlobalDecl
	FinalDefault          model.DerivationSet
	AttributeFormDefault  Form
	ElementFormDefault    Form
	BlockDefault          model.DerivationSet
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		ElementDecls:          make(map[model.QName]*model.ElementDecl),
		ElementOrigins:        make(map[model.QName]string),
		TypeDefs:              make(map[model.QName]model.Type),
		TypeOrigins:           make(map[model.QName]string),
		AttributeDecls:        make(map[model.QName]*model.AttributeDecl),
		AttributeOrigins:      make(map[model.QName]string),
		AttributeGroups:       make(map[model.QName]*model.AttributeGroup),
		AttributeGroupOrigins: make(map[model.QName]string),
		Groups:                make(map[model.QName]*model.ModelGroup),
		GroupOrigins:          make(map[model.QName]string),
		SubstitutionGroups:    make(map[model.QName][]model.QName),
		NotationDecls:         make(map[model.QName]*model.NotationDecl),
		NotationOrigins:       make(map[model.QName]string),
		NamespaceDecls:        make(map[string]string),
		IDAttributes:          make(map[string]string),
		ImportedNamespaces:    make(map[model.NamespaceURI]map[model.NamespaceURI]bool),
		ImportContexts:        make(map[string]ImportContext),
		GlobalDecls:           []GlobalDecl{},
	}
}

func (s *Schema) addGlobalDecl(kind GlobalDeclKind, name model.QName) {
	s.GlobalDecls = append(s.GlobalDecls, GlobalDecl{Kind: kind, Name: name})
}

// ImportContext tracks import namespaces for a specific schema document.
type ImportContext struct {
	Imports         map[model.NamespaceURI]bool
	TargetNamespace model.NamespaceURI
}
