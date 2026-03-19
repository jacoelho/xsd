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

// SchemaGraph holds the compile-time declaration graph.
type SchemaGraph struct {
	Groups             map[model.QName]*model.ModelGroup
	TypeDefs           map[model.QName]model.Type
	AttributeDecls     map[model.QName]*model.AttributeDecl
	SubstitutionGroups map[model.QName][]model.QName
	AttributeGroups    map[model.QName]*model.AttributeGroup
	ElementDecls       map[model.QName]*model.ElementDecl
	NotationDecls      map[model.QName]*model.NotationDecl
	GlobalDecls        []GlobalDecl
}

// SchemaMeta holds source, namespace, and declaration-origin metadata.
type SchemaMeta struct {
	ImportContexts        map[string]ImportContext
	ElementOrigins        map[model.QName]string
	TypeOrigins           map[model.QName]string
	AttributeOrigins      map[model.QName]string
	AttributeGroupOrigins map[model.QName]string
	ImportedNamespaces    map[model.NamespaceURI]map[model.NamespaceURI]bool
	GroupOrigins          map[model.QName]string
	NotationOrigins       map[model.QName]string
	IDAttributes          map[string]string
	NamespaceDecls        map[string]string
	Location              string
	TargetNamespace       model.NamespaceURI
	FinalDefault          model.DerivationSet
	AttributeFormDefault  Form
	ElementFormDefault    Form
	BlockDefault          model.DerivationSet
}

// Schema represents a compiled XSD schema.
type Schema struct {
	SchemaGraph
	SchemaMeta
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		SchemaGraph: newSchemaGraph(),
		SchemaMeta:  newSchemaMeta(),
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

func newSchemaGraph() SchemaGraph {
	return SchemaGraph{
		ElementDecls:       make(map[model.QName]*model.ElementDecl),
		TypeDefs:           make(map[model.QName]model.Type),
		AttributeDecls:     make(map[model.QName]*model.AttributeDecl),
		AttributeGroups:    make(map[model.QName]*model.AttributeGroup),
		Groups:             make(map[model.QName]*model.ModelGroup),
		SubstitutionGroups: make(map[model.QName][]model.QName),
		NotationDecls:      make(map[model.QName]*model.NotationDecl),
		GlobalDecls:        []GlobalDecl{},
	}
}

func newSchemaMeta() SchemaMeta {
	return SchemaMeta{
		ElementOrigins:        make(map[model.QName]string),
		TypeOrigins:           make(map[model.QName]string),
		AttributeOrigins:      make(map[model.QName]string),
		AttributeGroupOrigins: make(map[model.QName]string),
		GroupOrigins:          make(map[model.QName]string),
		NotationOrigins:       make(map[model.QName]string),
		NamespaceDecls:        make(map[string]string),
		IDAttributes:          make(map[string]string),
		ImportedNamespaces:    make(map[model.NamespaceURI]map[model.NamespaceURI]bool),
		ImportContexts:        make(map[string]ImportContext),
	}
}
