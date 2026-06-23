// Package runtime defines stable runtime vocabulary and metadata helpers.
package runtime

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
)

const invalidID = ^uint32(0)

// NamespaceID indexes a namespace URI in a runtime name table.
type NamespaceID uint32

// LocalNameID indexes a local name in a runtime name table.
type LocalNameID uint32

// QName identifies an expanded XML name through runtime name-table IDs.
type QName struct {
	Namespace NamespaceID
	Local     LocalNameID
}

// NoQName is the absent QName sentinel used in runtime identity paths.
var NoQName = QName{Namespace: NamespaceID(invalidID), Local: LocalNameID(invalidID)}

// EmptyNamespaceID is the name-table ID for the empty namespace URI.
const EmptyNamespaceID NamespaceID = 0

const (
	// EmptyNamespaceURI is the no-namespace URI.
	EmptyNamespaceURI = vocab.EmptyNamespaceURI
	// XSDNamespaceURI is the XML Schema namespace URI.
	XSDNamespaceURI = vocab.XSDNamespaceURI
	// XSINamespaceURI is the XML Schema instance namespace URI.
	XSINamespaceURI = vocab.XSINamespaceURI
	// XMLNamespaceURI is the reserved XML namespace URI.
	XMLNamespaceURI = vocab.XMLNamespaceURI
	// XLinkNamespaceURI is the XLink namespace URI.
	XLinkNamespaceURI = vocab.XLinkNamespaceURI
	// XMLNSNamespaceURI is the reserved xmlns namespace URI.
	XMLNSNamespaceURI = vocab.XMLNSNamespaceURI
)

// SimpleTypeID indexes a simple type in a runtime schema.
type SimpleTypeID uint32

// ComplexTypeID indexes a complex type in a runtime schema.
type ComplexTypeID uint32

// BuiltinIDs stores runtime IDs for built-in schema components.
type BuiltinIDs struct {
	AnyType       ComplexTypeID
	AnySimpleType SimpleTypeID
	String        SimpleTypeID
	Boolean       SimpleTypeID
	Decimal       SimpleTypeID
	Integer       SimpleTypeID
	Int           SimpleTypeID
	Date          SimpleTypeID
	DateTime      SimpleTypeID
	Time          SimpleTypeID
	AnyURI        SimpleTypeID
	QName         SimpleTypeID
	ID            SimpleTypeID
	IDREF         SimpleTypeID
	IDREFS        SimpleTypeID
	NMTOKEN       SimpleTypeID
	NMTOKENS      SimpleTypeID
	ENTITY        SimpleTypeID
	ENTITIES      SimpleTypeID
}

// ElementID indexes an element declaration in a runtime schema.
type ElementID uint32

// AttributeID indexes an attribute declaration in a runtime schema.
type AttributeID uint32

// ContentModelID indexes a content model in a runtime schema.
type ContentModelID uint32

// AttributeUseSetID indexes an attribute-use set in a runtime schema.
type AttributeUseSetID uint32

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
	// DerivationSubstitution blocks element substitution.
	DerivationSubstitution
	// DerivationList blocks derivation by list.
	DerivationList
	// DerivationUnion blocks derivation by union.
	DerivationUnion
)

const (
	// DerivationComplexMask is the derivation set allowed for complex-type block/final.
	DerivationComplexMask = DerivationExtension | DerivationRestriction
	// DerivationBlockDefaultMask is the derivation set allowed for schema blockDefault.
	DerivationBlockDefaultMask = DerivationExtension | DerivationRestriction | DerivationSubstitution
	// DerivationFinalDefaultMask is the derivation set allowed for schema finalDefault.
	DerivationFinalDefaultMask = DerivationExtension | DerivationRestriction | DerivationList | DerivationUnion
	// DerivationSimpleFinalMask is the derivation set allowed for simple-type final.
	DerivationSimpleFinalMask = DerivationRestriction | DerivationList | DerivationUnion
)

const (
	derivationSetAllToken          = "#all"
	derivationSetExtensionToken    = vocab.XSDElemExtension
	derivationSetRestrictionToken  = vocab.XSDElemRestriction
	derivationSetSubstitutionToken = "substitution"
	derivationSetListToken         = vocab.XSDElemList
	derivationSetUnionToken        = vocab.XSDElemUnion
)

// DerivationSetIssueKind classifies derivation-set lexical validation results.
type DerivationSetIssueKind uint8

const (
	// DerivationSetOK reports a valid derivation set.
	DerivationSetOK DerivationSetIssueKind = iota
	// DerivationSetInvalidToken reports an unknown derivation-set token.
	DerivationSetInvalidToken
	// DerivationSetDisallowedToken reports a known token outside the allowed class.
	DerivationSetDisallowedToken
	// DerivationSetAllCombination reports #all combined with another token.
	DerivationSetAllCombination
)

// DerivationSetIssue reports why a derivation set is invalid.
type DerivationSetIssue struct {
	Token string
	Kind  DerivationSetIssueKind
}

// ParseDerivationSet parses an XSD derivation-set lexical value.
func ParseDerivationSet(value string, allowed DerivationMask) (DerivationMask, DerivationSetIssue) {
	var mask DerivationMask
	seenAll := false
	for token := range derivationSetFields(value) {
		if token == derivationSetAllToken {
			if seenAll || mask != 0 {
				return 0, DerivationSetIssue{Kind: DerivationSetAllCombination, Token: token}
			}
			seenAll = true
			continue
		}
		if seenAll {
			return 0, DerivationSetIssue{Kind: DerivationSetAllCombination, Token: token}
		}
		bit, ok := derivationSetTokenMask(token)
		if !ok {
			return 0, DerivationSetIssue{Kind: DerivationSetInvalidToken, Token: token}
		}
		if allowed&bit == 0 {
			return 0, DerivationSetIssue{Kind: DerivationSetDisallowedToken, Token: token}
		}
		mask |= bit
	}
	if seenAll {
		return allowed, DerivationSetIssue{}
	}
	return mask, DerivationSetIssue{}
}

func derivationSetTokenMask(token string) (DerivationMask, bool) {
	switch token {
	case derivationSetExtensionToken:
		return DerivationExtension, true
	case derivationSetRestrictionToken:
		return DerivationRestriction, true
	case derivationSetSubstitutionToken:
		return DerivationSubstitution, true
	case derivationSetListToken:
		return DerivationList, true
	case derivationSetUnionToken:
		return DerivationUnion, true
	default:
		return 0, false
	}
}

func derivationSetFields(value string) func(func(string) bool) {
	return func(yield func(string) bool) {
		start := -1
		for i := range len(value) {
			if lex.IsXMLWhitespaceByte(value[i]) {
				if start >= 0 {
					if !yield(value[start:i]) {
						return
					}
					start = -1
				}
				continue
			}
			if start < 0 {
				start = i
			}
		}
		if start >= 0 {
			yield(value[start:])
		}
	}
}

// ValidElementBlockMask reports whether mask is valid for an element block.
func ValidElementBlockMask(mask DerivationMask) bool {
	return mask&^DerivationBlockDefaultMask == 0
}

// ValidElementFinalMask reports whether mask is valid for an element final.
func ValidElementFinalMask(mask DerivationMask) bool {
	return mask&^DerivationComplexMask == 0
}

// ValidComplexBlockMask reports whether mask is valid for a complex-type block.
func ValidComplexBlockMask(mask DerivationMask) bool {
	return mask&^DerivationComplexMask == 0
}

// ValidComplexFinalMask reports whether mask is valid for a complex-type final.
func ValidComplexFinalMask(mask DerivationMask) bool {
	return mask&^DerivationComplexMask == 0
}

// ValidSimpleFinalMask reports whether mask is valid for a simple-type final.
func ValidSimpleFinalMask(mask DerivationMask) bool {
	return mask&^DerivationSimpleFinalMask == 0
}

const (
	// NoSimpleType is the absent simple-type sentinel.
	NoSimpleType SimpleTypeID = SimpleTypeID(invalidID)
	// NoComplexType is the absent complex-type sentinel.
	NoComplexType ComplexTypeID = ComplexTypeID(invalidID)
	// NoElement is the absent element-declaration sentinel.
	NoElement ElementID = ElementID(invalidID)
	// NoContentModel is the absent content-model sentinel.
	NoContentModel ContentModelID = ContentModelID(invalidID)
	// NoAttributeUseSet is the absent attribute-use-set sentinel.
	NoAttributeUseSet AttributeUseSetID = AttributeUseSetID(invalidID)
	// NoWildcard is the absent wildcard sentinel.
	NoWildcard WildcardID = WildcardID(invalidID)
	// NoIdentityConstraint is the absent identity-constraint sentinel.
	NoIdentityConstraint IdentityConstraintID = IdentityConstraintID(invalidID)
)

// NewUint32Index returns n as a uint32 index when it is representable.
func NewUint32Index(n int) (uint32, bool) {
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

// NextSimpleTypeID returns the next simple-type ID for a table of length n.
func NextSimpleTypeID(n int) (SimpleTypeID, bool) {
	id, ok := newRuntimeID(n)
	return SimpleTypeID(id), ok
}

// NextComplexTypeID returns the next complex-type ID for a table of length n.
func NextComplexTypeID(n int) (ComplexTypeID, bool) {
	id, ok := newRuntimeID(n)
	return ComplexTypeID(id), ok
}

// NextElementID returns the next element declaration ID for a table of length n.
func NextElementID(n int) (ElementID, bool) {
	id, ok := newRuntimeID(n)
	return ElementID(id), ok
}

// NextAttributeID returns the next attribute declaration ID for a table of length n.
func NextAttributeID(n int) (AttributeID, bool) {
	id, ok := newRuntimeID(n)
	return AttributeID(id), ok
}

// NextContentModelID returns the next content-model ID for a table of length n.
func NextContentModelID(n int) (ContentModelID, bool) {
	id, ok := newRuntimeID(n)
	return ContentModelID(id), ok
}

// NextAttributeUseSetID returns the next attribute-use-set ID for a table of length n.
func NextAttributeUseSetID(n int) (AttributeUseSetID, bool) {
	id, ok := newRuntimeID(n)
	return AttributeUseSetID(id), ok
}

// NextWildcardID returns the next wildcard ID for a table of length n.
func NextWildcardID(n int) (WildcardID, bool) {
	id, ok := newRuntimeID(n)
	return WildcardID(id), ok
}

// NextIdentityConstraintID returns the next identity-constraint ID for a table
// of length n.
func NextIdentityConstraintID(n int) (IdentityConstraintID, bool) {
	id, ok := newRuntimeID(n)
	return IdentityConstraintID(id), ok
}

// TypeKind identifies the component table referenced by a TypeID.
type TypeKind uint8

const (
	// TypeNone is the zero kind and does not reference a real type.
	TypeNone TypeKind = iota
	// TypeSimple references a simple type.
	TypeSimple
	// TypeComplex references a complex type.
	TypeComplex
)

// TypeID identifies either a simple type or complex type in a runtime schema.
type TypeID struct {
	Kind TypeKind
	ID   uint32
}

// SimpleRef returns a runtime reference to a simple type.
func SimpleRef(id SimpleTypeID) TypeID {
	return TypeID{Kind: TypeSimple, ID: uint32(id)}
}

// ComplexRef returns a runtime reference to a complex type.
func ComplexRef(id ComplexTypeID) TypeID {
	return TypeID{Kind: TypeComplex, ID: uint32(id)}
}

// Simple returns the simple-type ID if t references a simple type.
func (t TypeID) Simple() (SimpleTypeID, bool) {
	if t.Kind != TypeSimple {
		return NoSimpleType, false
	}
	return SimpleTypeID(t.ID), true
}

// Complex returns the complex-type ID if t references a complex type.
func (t TypeID) Complex() (ComplexTypeID, bool) {
	if t.Kind != TypeComplex {
		return NoComplexType, false
	}
	return ComplexTypeID(t.ID), true
}

// ValidTypeID reports whether typ indexes one of the runtime type tables.
func ValidTypeID(typ TypeID, simpleCount, complexCount int) bool {
	switch typ.Kind {
	case TypeSimple:
		return ValidUint32Index(typ.ID, simpleCount)
	case TypeComplex:
		return ValidUint32Index(typ.ID, complexCount)
	default:
		return false
	}
}

// ValidUint32Index reports whether id is a valid index into a slice of length n.
func ValidUint32Index(id uint32, n int) bool {
	if n < 0 {
		return false
	}
	return uint64(id) < uint64(n)
}

// ValidSimpleTypeID reports whether id indexes a simple-type table of length n.
func ValidSimpleTypeID(id SimpleTypeID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidComplexTypeID reports whether id indexes a complex-type table of length n.
func ValidComplexTypeID(id ComplexTypeID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidElementID reports whether id indexes an element-declaration table of length n.
func ValidElementID(id ElementID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidAttributeID reports whether id indexes an attribute-declaration table of length n.
func ValidAttributeID(id AttributeID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidContentModelID reports whether id indexes a content-model table of length n.
func ValidContentModelID(id ContentModelID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidAttributeUseSetID reports whether id indexes an attribute-use-set table of length n.
func ValidAttributeUseSetID(id AttributeUseSetID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidWildcardID reports whether id indexes a wildcard table of length n.
func ValidWildcardID(id WildcardID, n int) bool {
	return ValidUint32Index(uint32(id), n)
}

// ValidIdentityConstraintID reports whether id indexes an identity-constraint table of length n.
func ValidIdentityConstraintID(id IdentityConstraintID, n int) bool {
	return ValidUint32Index(uint32(id), n)
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
	return FormatExpandedName(n.NS, n.Local)
}

// FormatExpandedName formats a namespace URI and local name as an expanded XML name.
func FormatExpandedName(ns, local string) string {
	if ns == "" {
		return local
	}
	return "{" + ns + "}" + local
}

// IdentityConstraint is the runtime metadata for an XSD identity constraint.
type IdentityConstraint struct {
	Selector                []IdentityPath
	Fields                  []IdentityField
	ElementFields           []CompiledIdentityField
	AttributeFields         map[QName][]CompiledIdentityField
	AttributeWildcardFields []CompiledIdentityField
	Name                    QName
	Refer                   IdentityConstraintID
	Kind                    IdentityKind
}

// CompiledIdentityField groups paths for one field after lookup compilation.
type CompiledIdentityField struct {
	Paths []IdentityFieldPath
	Field int
}

// IdentityPath is one parsed selector XPath branch.
type IdentityPath struct {
	Steps      []IdentityStep
	Descendant bool
	Self       bool
}

// IdentityStep is one parsed identity XPath name test.
type IdentityStep struct {
	Name         QName
	Wildcard     bool
	NamespaceSet bool
	Namespace    NamespaceID
}

// IdentityField is one identity field with all parsed XPath alternatives.
type IdentityField struct {
	Paths []IdentityFieldPath
}

// IdentityFieldPath is one parsed field XPath branch.
type IdentityFieldPath struct {
	Steps            []IdentityStep
	Attribute        QName
	AttrNamespace    NamespaceID
	Descendant       bool
	Self             bool
	Attr             bool
	AttrWildcard     bool
	AttrNamespaceSet bool
}

// BuildIdentityFieldLookup partitions identity fields by element, exact
// attribute, and wildcard attribute lookup.
func BuildIdentityFieldLookup(fields []IdentityField) ([]CompiledIdentityField, map[QName][]CompiledIdentityField, []CompiledIdentityField) {
	var elementFields []CompiledIdentityField
	var attrFields map[QName][]CompiledIdentityField
	var attrWildcardFields []CompiledIdentityField
	for fieldIndex := range fields {
		var elementPaths []IdentityFieldPath
		var wildcardAttrPaths []IdentityFieldPath
		var exactAttrPaths map[QName][]IdentityFieldPath
		for _, path := range fields[fieldIndex].Paths {
			path = cloneIdentityFieldPath(path)
			if !path.Attr {
				elementPaths = append(elementPaths, path)
				continue
			}
			if path.AttrWildcard {
				wildcardAttrPaths = append(wildcardAttrPaths, path)
				continue
			}
			if exactAttrPaths == nil {
				exactAttrPaths = make(map[QName][]IdentityFieldPath)
			}
			exactAttrPaths[path.Attribute] = append(exactAttrPaths[path.Attribute], path)
		}
		if len(elementPaths) != 0 {
			elementFields = append(elementFields, CompiledIdentityField{
				Field: fieldIndex,
				Paths: elementPaths,
			})
		}
		if len(wildcardAttrPaths) != 0 {
			attrWildcardFields = append(attrWildcardFields, CompiledIdentityField{
				Field: fieldIndex,
				Paths: wildcardAttrPaths,
			})
		}
		for name, paths := range exactAttrPaths {
			if attrFields == nil {
				attrFields = make(map[QName][]CompiledIdentityField)
			}
			attrFields[name] = append(attrFields[name], CompiledIdentityField{
				Field: fieldIndex,
				Paths: paths,
			})
		}
	}
	return elementFields, attrFields, attrWildcardFields
}
