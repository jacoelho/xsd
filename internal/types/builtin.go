package types

import (
	"strings"
	"unicode/utf8"
)

// XSDNamespace is the XML Schema namespace URI.
const XSDNamespace NamespaceURI = "http://www.w3.org/2001/XMLSchema"

// TypeValidator validates a string value according to a type's rules.
// It is used to implement validation logic for built-in XSD types.
// Returns an error if the value is invalid, nil otherwise.
type TypeValidator func(value string) error

// BuiltinType represents a built-in XSD type
type BuiltinType struct {
	primitiveTypeCache     Type
	validator              TypeValidator
	fundamentalFacetsCache *FundamentalFacets
	simpleWrapper          *SimpleType
	qname                  QName
	name                   string
	whiteSpace             WhiteSpace
	ordered                bool
}

type orderingFlag bool

const (
	unordered orderingFlag = false
	ordered   orderingFlag = true
)

var builtinRegistry = map[string]*BuiltinType{
	// built-in complex type
	string(TypeNameAnyType): newBuiltin(TypeNameAnyType, validateAnyType, WhiteSpacePreserve, unordered),

	// base simple type (base of all simple types, must be registered before primitives)
	string(TypeNameAnySimpleType): newBuiltin(TypeNameAnySimpleType, validateAnySimpleType, WhiteSpacePreserve, unordered),

	// primitive types (19 total)
	string(TypeNameString):       newBuiltin(TypeNameString, validateString, WhiteSpacePreserve, unordered),
	string(TypeNameBoolean):      newBuiltin(TypeNameBoolean, validateBoolean, WhiteSpaceCollapse, unordered),
	string(TypeNameDecimal):      newBuiltin(TypeNameDecimal, validateDecimal, WhiteSpaceCollapse, ordered),
	string(TypeNameFloat):        newBuiltin(TypeNameFloat, validateFloat, WhiteSpaceCollapse, ordered),
	string(TypeNameDouble):       newBuiltin(TypeNameDouble, validateDouble, WhiteSpaceCollapse, ordered),
	string(TypeNameDuration):     newBuiltin(TypeNameDuration, validateDuration, WhiteSpaceCollapse, unordered),
	string(TypeNameDateTime):     newBuiltin(TypeNameDateTime, validateDateTime, WhiteSpaceCollapse, ordered),
	string(TypeNameTime):         newBuiltin(TypeNameTime, validateTime, WhiteSpaceCollapse, ordered),
	string(TypeNameDate):         newBuiltin(TypeNameDate, validateDate, WhiteSpaceCollapse, ordered),
	string(TypeNameGYearMonth):   newBuiltin(TypeNameGYearMonth, validateGYearMonth, WhiteSpaceCollapse, ordered),
	string(TypeNameGYear):        newBuiltin(TypeNameGYear, validateGYear, WhiteSpaceCollapse, ordered),
	string(TypeNameGMonthDay):    newBuiltin(TypeNameGMonthDay, validateGMonthDay, WhiteSpaceCollapse, ordered),
	string(TypeNameGDay):         newBuiltin(TypeNameGDay, validateGDay, WhiteSpaceCollapse, ordered),
	string(TypeNameGMonth):       newBuiltin(TypeNameGMonth, validateGMonth, WhiteSpaceCollapse, ordered),
	string(TypeNameHexBinary):    newBuiltin(TypeNameHexBinary, validateHexBinary, WhiteSpaceCollapse, unordered),
	string(TypeNameBase64Binary): newBuiltin(TypeNameBase64Binary, validateBase64Binary, WhiteSpaceCollapse, unordered),
	string(TypeNameAnyURI):       newBuiltin(TypeNameAnyURI, validateAnyURI, WhiteSpaceCollapse, unordered),
	string(TypeNameQName):        newBuiltin(TypeNameQName, validateQName, WhiteSpaceCollapse, unordered),
	string(TypeNameNOTATION):     newBuiltin(TypeNameNOTATION, validateNOTATION, WhiteSpaceCollapse, unordered),

	// derived string types
	string(TypeNameNormalizedString): newBuiltin(TypeNameNormalizedString, validateNormalizedString, WhiteSpaceReplace, unordered),
	string(TypeNameToken):            newBuiltin(TypeNameToken, validateToken, WhiteSpaceCollapse, unordered),
	string(TypeNameLanguage):         newBuiltin(TypeNameLanguage, validateLanguage, WhiteSpaceCollapse, unordered),
	string(TypeNameName):             newBuiltin(TypeNameName, validateName, WhiteSpaceCollapse, unordered),
	string(TypeNameNCName):           newBuiltin(TypeNameNCName, validateNCName, WhiteSpaceCollapse, unordered),
	string(TypeNameID):               newBuiltin(TypeNameID, validateID, WhiteSpaceCollapse, unordered),
	string(TypeNameIDREF):            newBuiltin(TypeNameIDREF, validateIDREF, WhiteSpaceCollapse, unordered),
	string(TypeNameIDREFS):           newBuiltin(TypeNameIDREFS, validateIDREFS, WhiteSpaceCollapse, unordered),
	string(TypeNameENTITY):           newBuiltin(TypeNameENTITY, validateENTITY, WhiteSpaceCollapse, unordered),
	string(TypeNameENTITIES):         newBuiltin(TypeNameENTITIES, validateENTITIES, WhiteSpaceCollapse, unordered),
	string(TypeNameNMTOKEN):          newBuiltin(TypeNameNMTOKEN, validateNMTOKEN, WhiteSpaceCollapse, unordered),
	string(TypeNameNMTOKENS):         newBuiltin(TypeNameNMTOKENS, validateNMTOKENS, WhiteSpaceCollapse, unordered),

	// derived numeric types
	string(TypeNameInteger):            newBuiltin(TypeNameInteger, validateInteger, WhiteSpaceCollapse, ordered),
	string(TypeNameLong):               newBuiltin(TypeNameLong, validateLong, WhiteSpaceCollapse, ordered),
	string(TypeNameInt):                newBuiltin(TypeNameInt, validateInt, WhiteSpaceCollapse, ordered),
	string(TypeNameShort):              newBuiltin(TypeNameShort, validateShort, WhiteSpaceCollapse, ordered),
	string(TypeNameByte):               newBuiltin(TypeNameByte, validateByte, WhiteSpaceCollapse, ordered),
	string(TypeNameNonNegativeInteger): newBuiltin(TypeNameNonNegativeInteger, validateNonNegativeInteger, WhiteSpaceCollapse, ordered),
	string(TypeNamePositiveInteger):    newBuiltin(TypeNamePositiveInteger, validatePositiveInteger, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedLong):       newBuiltin(TypeNameUnsignedLong, validateUnsignedLong, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedInt):        newBuiltin(TypeNameUnsignedInt, validateUnsignedInt, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedShort):      newBuiltin(TypeNameUnsignedShort, validateUnsignedShort, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedByte):       newBuiltin(TypeNameUnsignedByte, validateUnsignedByte, WhiteSpaceCollapse, ordered),
	string(TypeNameNonPositiveInteger): newBuiltin(TypeNameNonPositiveInteger, validateNonPositiveInteger, WhiteSpaceCollapse, ordered),
	string(TypeNameNegativeInteger):    newBuiltin(TypeNameNegativeInteger, validateNegativeInteger, WhiteSpaceCollapse, ordered),
}

var primitiveTypeNames = map[TypeName]struct{}{
	TypeNameString:       {},
	TypeNameBoolean:      {},
	TypeNameDecimal:      {},
	TypeNameFloat:        {},
	TypeNameDouble:       {},
	TypeNameDuration:     {},
	TypeNameDateTime:     {},
	TypeNameTime:         {},
	TypeNameDate:         {},
	TypeNameGYearMonth:   {},
	TypeNameGYear:        {},
	TypeNameGMonthDay:    {},
	TypeNameGDay:         {},
	TypeNameGMonth:       {},
	TypeNameHexBinary:    {},
	TypeNameBase64Binary: {},
	TypeNameAnyURI:       {},
	TypeNameQName:        {},
	TypeNameNOTATION:     {},
}

var builtinBaseTypes = map[TypeName]TypeName{
	// Derived from string
	TypeNameNormalizedString: TypeNameString,
	TypeNameToken:            TypeNameNormalizedString,
	TypeNameLanguage:         TypeNameToken,
	TypeNameName:             TypeNameToken,
	TypeNameNCName:           TypeNameName,
	TypeNameID:               TypeNameNCName,
	TypeNameIDREF:            TypeNameNCName,
	TypeNameIDREFS:           TypeNameIDREF,
	TypeNameENTITY:           TypeNameNCName,
	TypeNameENTITIES:         TypeNameENTITY,
	TypeNameNMTOKEN:          TypeNameToken,
	TypeNameNMTOKENS:         TypeNameNMTOKEN,
	// Derived from decimal
	TypeNameInteger:            TypeNameDecimal,
	TypeNameLong:               TypeNameInteger,
	TypeNameInt:                TypeNameLong,
	TypeNameShort:              TypeNameInt,
	TypeNameByte:               TypeNameShort,
	TypeNameNonNegativeInteger: TypeNameInteger,
	TypeNamePositiveInteger:    TypeNameNonNegativeInteger,
	TypeNameUnsignedLong:       TypeNameNonNegativeInteger,
	TypeNameUnsignedInt:        TypeNameUnsignedLong,
	TypeNameUnsignedShort:      TypeNameUnsignedInt,
	TypeNameUnsignedByte:       TypeNameUnsignedShort,
	TypeNameNonPositiveInteger: TypeNameInteger,
	TypeNameNegativeInteger:    TypeNameNonPositiveInteger,
}

// GetBuiltin returns a built-in type by name (local name only, assumes XSD namespace)
func GetBuiltin(name TypeName) *BuiltinType {
	return builtinRegistry[string(name)]
}

// GetBuiltinNS returns a built-in type by namespace and local name
func GetBuiltinNS(namespace NamespaceURI, local string) *BuiltinType {
	if namespace != XSDNamespace {
		return nil
	}
	return builtinRegistry[local]
}

func newBuiltin(name TypeName, validator TypeValidator, ws WhiteSpace, ordering orderingFlag) *BuiltinType {
	nameStr := string(name)
	builtin := &BuiltinType{
		name:       nameStr,
		qname:      QName{Namespace: XSDNamespace, Local: nameStr},
		validator:  validator,
		whiteSpace: ws,
		ordered:    bool(ordering),
	}
	simple := &SimpleType{
		QName:   builtin.qname,
		variety: AtomicVariety,
		builtin: true,
	}
	builtin.simpleWrapper = simple
	return builtin
}

// Compile-time check that BuiltinType implements Type interface
var _ Type = (*BuiltinType)(nil)

// Compile-time check that BuiltinType implements DerivedType
var _ DerivedType = (*BuiltinType)(nil)

// Compile-time check that BuiltinType implements LengthMeasurable
var _ LengthMeasurable = (*BuiltinType)(nil)

// Name returns the type name as a QName
func (b *BuiltinType) Name() QName {
	return b.qname
}

// IsBuiltin returns true for built-in types
func (b *BuiltinType) IsBuiltin() bool {
	return true
}

// IsQNameOrNotationType reports whether this built-in type is QName or NOTATION.
func (b *BuiltinType) IsQNameOrNotationType() bool {
	if b == nil {
		return false
	}
	return IsQNameOrNotation(b.Name())
}

// Validate validates a value against this type
func (b *BuiltinType) Validate(value string) error {
	return b.validator(value)
}

// ParseValue converts a lexical value to a TypedValue
func (b *BuiltinType) ParseValue(lexical string) (TypedValue, error) {
	normalized, err := NormalizeValue(lexical, b)
	if err != nil {
		return nil, err
	}

	if err := b.Validate(normalized); err != nil {
		return nil, err
	}

	typeName := TypeName(b.name)
	result, err := ParseValueForType(normalized, typeName, b)
	if err == nil {
		return result, nil
	}

	// fallback for types not in registry: create a string value
	return NewStringValue(NewParsedValue(normalized, normalized), b.simpleWrapper), nil
}

// Variety returns the simple type variety (all built-in types are atomic)
func (b *BuiltinType) Variety() SimpleTypeVariety {
	return AtomicVariety
}

// WhiteSpace returns the whitespace normalization for this type
func (b *BuiltinType) WhiteSpace() WhiteSpace {
	return b.whiteSpace
}

// Ordered returns whether this type has an order relation
func (b *BuiltinType) Ordered() bool {
	return b.ordered
}

// MeasureLength returns length in type-appropriate units (octets, items, or characters).
// Implements LengthMeasurable interface.
func (b *BuiltinType) MeasureLength(value string) int {
	name := b.Name().Local

	// check if it's a built-in list type (NMTOKENS, IDREFS, ENTITIES)
	if isBuiltinListType(name) {
		// list type: length is number of items (space-separated)
		if len(strings.TrimSpace(value)) == 0 {
			return 0
		}
		count := 0
		for range strings.FieldsSeq(value) {
			count++
		}
		return count
	}

	primitiveType := b.PrimitiveType()
	if primitiveType != nil {
		return measureLengthForPrimitive(value, TypeName(primitiveType.Name().Local))
	}

	// fallback: character count
	return utf8.RuneCountInString(value)
}

// FundamentalFacets returns the fundamental facets for this built-in type
func (b *BuiltinType) FundamentalFacets() *FundamentalFacets {
	if b.fundamentalFacetsCache != nil {
		return b.fundamentalFacetsCache
	}

	typeName := TypeName(b.name)

	// for primitive types, compute directly
	if isPrimitiveName(typeName) {
		return b.setFundamentalFacets(ComputeFundamentalFacets(typeName))
	}

	// for derived types, get facets from primitive type
	primitive := b.PrimitiveType()
	if primitive == nil {
		// fallback: try computing from name (may return nil for unknown types)
		return b.setFundamentalFacets(ComputeFundamentalFacets(typeName))
	}

	if bt, ok := as[*BuiltinType](primitive); ok {
		return b.setFundamentalFacets(bt.FundamentalFacets())
	}

	// if primitive is not BuiltinType, try to get facets from it
	return b.setFundamentalFacets(primitive.FundamentalFacets())
}

func (b *BuiltinType) setFundamentalFacets(facets *FundamentalFacets) *FundamentalFacets {
	b.fundamentalFacetsCache = facets
	return facets
}

// BaseType returns the base type for this built-in type
func (b *BuiltinType) BaseType() Type {
	// anyType has no base type (it is the root)
	if b.name == string(TypeNameAnyType) {
		return nil
	}
	// anySimpleType derives from anyType
	if b.name == string(TypeNameAnySimpleType) {
		return GetBuiltin(TypeNameAnyType)
	}

	// primitive types have anySimpleType as base
	if isPrimitiveName(TypeName(b.name)) {
		return GetBuiltin(TypeNameAnySimpleType)
	}

	// for derived types, compute base type from type hierarchy
	return computeBaseType(b.name)
}

// ResolvedBaseType returns the resolved base type, or nil if at root.
// Implements DerivedType interface.
func (b *BuiltinType) ResolvedBaseType() Type {
	return b.BaseType()
}

func isPrimitiveName(name TypeName) bool {
	_, ok := primitiveTypeNames[name]
	return ok
}

// isPrimitive checks if a type name is one of the 19 primitive types
func isPrimitive(name string) bool {
	return isPrimitiveName(TypeName(name))
}

// computeBaseType computes the base type for a derived built-in type
func computeBaseType(name string) Type {
	// map derived types to their bases according to XSD 1.0 type hierarchy
	if baseName, ok := builtinBaseTypes[TypeName(name)]; ok {
		return GetBuiltin(baseName)
	}
	// if not found in map, return anySimpleType as fallback (base of all simple types)
	return GetBuiltin(TypeNameAnySimpleType)
}

// PrimitiveType returns the primitive type for this built-in type
func (b *BuiltinType) PrimitiveType() Type {
	// return cached value if available
	if b.primitiveTypeCache != nil {
		return b.primitiveTypeCache
	}

	// anySimpleType and anyType have no primitive type (they are abstract roots)
	if b.name == string(TypeNameAnySimpleType) || b.name == string(TypeNameAnyType) {
		return nil
	}

	// for primitive types, return self
	if isPrimitive(b.name) {
		b.primitiveTypeCache = b
		return b
	}

	// for derived types, follow base type chain
	base := b.BaseType()
	if base == nil {
		return nil
	}
	primitive := base.PrimitiveType()
	b.primitiveTypeCache = primitive
	return primitive
}
