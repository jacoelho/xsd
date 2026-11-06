package schema

import "github.com/jacoelho/xsd/internal/value"

const invalidID = ^uint32(0)

// namespaceID indexes a namespace URI in a runtime name table.
type namespaceID uint32

// localNameID indexes a local name in a runtime name table.
type localNameID uint32

// QName identifies an expanded XML name through runtime name-table IDs.
type QName struct {
	Namespace namespaceID
	Local     localNameID
}

// noQName returns the absent QName sentinel used in runtime identity paths.
func noQName() QName {
	return QName{Namespace: namespaceID(invalidID), Local: localNameID(invalidID)}
}

// emptyNamespaceID is the name-table ID for the empty namespace URI.
const emptyNamespaceID namespaceID = 0

// SimpleTypeID is the canonical value-program type ID. Schema declarations
// use the same ID space so compiler, value validation, and published reads do
// not need unchecked numeric conversions.
type SimpleTypeID = value.TypeID

// ComplexTypeID indexes a complex type in a runtime schema.
type ComplexTypeID uint32

// ElementID indexes an element declaration in a runtime schema.
type ElementID uint32

// AttributeID indexes an attribute declaration in a runtime schema.
type AttributeID uint32

// contentModelID indexes a content model in a runtime schema.
type contentModelID uint32

// attributeUseSetID indexes an attribute-use set in a runtime schema.
type attributeUseSetID uint32

// WildcardID indexes a wildcard in a runtime schema.
type WildcardID uint32

// IdentityConstraintID indexes an identity constraint in a runtime schema.
type IdentityConstraintID uint32

// IdentityKind identifies identity-constraint table semantics.
type IdentityKind uint8

const (
	// IdentityUnique records optional unique tuples.
	IdentityUnique IdentityKind = iota
	// IdentityKey records required key tuples.
	IdentityKey
	// IdentityKeyRef records key references resolved against another key.
	IdentityKeyRef
)

// DerivationMask stores XSD block/final derivation set bits.
type DerivationMask uint8

const (
	// DerivationExtension blocks or records derivation by extension.
	DerivationExtension DerivationMask = 1 << iota
	// DerivationRestriction blocks or records derivation by restriction.
	DerivationRestriction
	// derivationSubstitution blocks element substitution.
	derivationSubstitution
	// derivationList blocks derivation by list.
	derivationList
	// derivationUnion blocks derivation by union.
	derivationUnion
)

const (
	// derivationComplexMask is the derivation set allowed for complex-type block/final.
	derivationComplexMask = DerivationExtension | DerivationRestriction
	// derivationBlockDefaultMask is the derivation set allowed for schema blockDefault.
	derivationBlockDefaultMask = DerivationExtension | DerivationRestriction | derivationSubstitution
	// derivationFinalDefaultMask is the derivation set allowed for schema finalDefault.
	derivationFinalDefaultMask = DerivationExtension | DerivationRestriction | derivationList | derivationUnion
	// derivationSimpleFinalMask is the derivation set allowed for simple-type final.
	derivationSimpleFinalMask = DerivationRestriction | derivationList | derivationUnion
)

// validElementBlockMask reports whether mask is valid for an element block.
func validElementBlockMask(mask DerivationMask) bool {
	return mask&^derivationBlockDefaultMask == 0
}

// validElementFinalMask reports whether mask is valid for an element final.
func validElementFinalMask(mask DerivationMask) bool {
	return mask&^derivationComplexMask == 0
}

// validComplexBlockMask reports whether mask is valid for a complex-type block.
func validComplexBlockMask(mask DerivationMask) bool {
	return mask&^derivationComplexMask == 0
}

// validComplexFinalMask reports whether mask is valid for a complex-type final.
func validComplexFinalMask(mask DerivationMask) bool {
	return mask&^derivationComplexMask == 0
}

// validSimpleFinalMask reports whether mask is valid for a simple-type final.
func validSimpleFinalMask(mask DerivationMask) bool {
	return mask&^derivationSimpleFinalMask == 0
}

const (
	// NoSimpleType is the absent simple-type sentinel.
	NoSimpleType = value.NoType
	// noComplexType is the absent complex-type sentinel.
	noComplexType ComplexTypeID = ComplexTypeID(invalidID)
	// NoElement is the absent element-declaration sentinel.
	NoElement ElementID = ElementID(invalidID)
	// noContentModel is the absent content-model sentinel.
	noContentModel contentModelID = contentModelID(invalidID)
	// noAttributeUseSet is the absent attribute-use-set sentinel.
	noAttributeUseSet attributeUseSetID = attributeUseSetID(invalidID)
	// NoWildcard is the absent wildcard sentinel.
	NoWildcard WildcardID = WildcardID(invalidID)
	// NoIdentityConstraint is the absent identity-constraint sentinel.
	NoIdentityConstraint IdentityConstraintID = IdentityConstraintID(invalidID)
)

// newUint32Index returns n as a uint32 index when it is representable.
func newUint32Index(n int) (uint32, bool) {
	if n < 0 || uint64(n) > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(n), true
}

func newRuntimeID(n int) (uint32, bool) {
	if n < 0 || uint64(n) >= uint64(invalidID) {
		return 0, false
	}
	return uint32(n), true
}

func validRuntimeID(id uint32, n int) bool {
	return id != invalidID && validUint32Index(id, n)
}

func validRuntimeIDTableLength(n int) bool {
	return n >= 0 && uint64(n) <= uint64(invalidID)
}

type typeKind uint8

const (
	typeNone typeKind = iota
	typeSimple
	typeComplex
)

// TypeID identifies either a simple type or complex type in a runtime schema.
type TypeID struct {
	kind typeKind
	id   uint32
}

// SimpleRef returns a runtime reference to a simple type.
func SimpleRef(id SimpleTypeID) TypeID {
	return TypeID{kind: typeSimple, id: uint32(id)}
}

// ComplexRef returns a runtime reference to a complex type.
func ComplexRef(id ComplexTypeID) TypeID {
	return TypeID{kind: typeComplex, id: uint32(id)}
}

// IsSimple reports whether t references the simple-type table.
func (t TypeID) IsSimple() bool {
	return t.kind == typeSimple
}

// IsComplex reports whether t references the complex-type table.
func (t TypeID) IsComplex() bool {
	return t.kind == typeComplex
}

// Simple returns the simple-type ID if t references a simple type.
func (t TypeID) Simple() (SimpleTypeID, bool) {
	if !t.IsSimple() {
		return NoSimpleType, false
	}
	return SimpleTypeID(t.id), true
}

// Complex returns the complex-type ID if t references a complex type.
func (t TypeID) Complex() (ComplexTypeID, bool) {
	if !t.IsComplex() {
		return noComplexType, false
	}
	return ComplexTypeID(t.id), true
}

func validTypeID(typ TypeID, simpleCount, complexCount int) bool {
	if id, ok := typ.Simple(); ok {
		return validSimpleTypeID(id, simpleCount)
	}
	if id, ok := typ.Complex(); ok {
		return validComplexTypeID(id, complexCount)
	}
	return false
}

// validUint32Index reports whether id is a valid index into a slice of length n.
func validUint32Index(id uint32, n int) bool {
	if n < 0 {
		return false
	}
	return uint64(id) < uint64(n)
}

// validSimpleTypeID reports whether id indexes a simple-type table of length n.
func validSimpleTypeID(id SimpleTypeID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validComplexTypeID reports whether id indexes a complex-type table of length n.
func validComplexTypeID(id ComplexTypeID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validElementID reports whether id indexes an element-declaration table of length n.
func validElementID(id ElementID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validAttributeID reports whether id indexes an attribute-declaration table of length n.
func validAttributeID(id AttributeID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validContentModelID reports whether id indexes a content-model table of length n.
func validContentModelID(id contentModelID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validAttributeUseSetID reports whether id indexes an attribute-use-set table of length n.
func validAttributeUseSetID(id attributeUseSetID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validWildcardID reports whether id indexes a wildcard table of length n.
func validWildcardID(id WildcardID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// validIdentityConstraintID reports whether id indexes an identity-constraint table of length n.
func validIdentityConstraintID(id IdentityConstraintID, n int) bool {
	return validRuntimeID(uint32(id), n)
}

// RuntimeName is an XML name resolved against the runtime name table when known.
type RuntimeName struct {
	NS    string
	Local string
	Name  QName
	Known bool
}

// Label formats the runtime name for diagnostics.
func (n RuntimeName) Label() string {
	if n.Known || n.NS == "" {
		return n.Local
	}
	return formatExpandedName(n.NS, n.Local)
}

// formatExpandedName formats a namespace URI and local name as an expanded XML name.
func formatExpandedName(ns, local string) string {
	if ns == "" {
		return local
	}
	return "{" + ns + "}" + local
}

// identityConstraintSpec is canonical construction metadata for an XSD identity constraint.
type identityConstraintSpec struct {
	Selector []identityPath
	Fields   []identityField
	Name     QName
	Refer    IdentityConstraintID
	Kind     IdentityKind
}

// identityPath is one parsed selector XPath branch.
type identityPath struct {
	Steps      []IdentityStep
	Descendant bool
}

// IdentityStep is one parsed identity XPath name test.
type IdentityStep struct {
	Name         QName
	Wildcard     bool
	NamespaceSet bool
	Namespace    namespaceID
}

// identityField is one identity field with all parsed XPath alternatives.
type identityField struct {
	Paths []identityFieldPath
}

// identityFieldPath is one parsed field XPath branch.
type identityFieldPath struct {
	Steps            []IdentityStep
	Attribute        QName
	AttrNamespace    namespaceID
	Descendant       bool
	Attr             bool
	AttrWildcard     bool
	AttrNamespaceSet bool
}
