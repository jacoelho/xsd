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

// Schema represents a compiled XSD schema
type Schema struct {
	NotationDecls           map[types.QName]*types.NotationDecl
	AttributeOrigins        map[types.QName]string
	ElementOrigins          map[types.QName]string
	TypeDefs                map[types.QName]types.Type
	TypeOrigins             map[types.QName]string
	AttributeDecls          map[types.QName]*types.AttributeDecl
	SubstitutionGroups      map[types.QName][]types.QName
	AttributeGroups         map[types.QName]*types.AttributeGroup
	AttributeGroupOrigins   map[types.QName]string
	Groups                  map[types.QName]*types.ModelGroup
	ElementDecls            map[types.QName]*types.ElementDecl
	GroupOrigins            map[types.QName]string
	ImportContexts          map[string]ImportContext
	NotationOrigins         map[types.QName]string
	ImportedNamespaces      map[types.NamespaceURI]map[types.NamespaceURI]bool
	ParticleRestrictionCaps map[*types.ElementDecl]types.Occurs
	IDAttributes            map[string]string
	NamespaceDecls          map[string]string
	TargetNamespace         types.NamespaceURI
	Location                string
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
	}
}

// ImportContext tracks import namespaces for a specific schema document.
type ImportContext struct {
	Imports         map[types.NamespaceURI]bool
	TargetNamespace types.NamespaceURI
}
