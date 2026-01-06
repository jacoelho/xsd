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
	name                   string
	qname                  QName // Cached QName for performance
	validator              TypeValidator
	whiteSpace             WhiteSpace
	ordered                bool
	primitiveTypeCache     Type
	fundamentalFacetsCache *FundamentalFacets
}

var builtinRegistry = make(map[string]*BuiltinType)

var primitiveTypeNames = map[TypeName]struct{}{
	TypeNameString: {}, TypeNameBoolean: {}, TypeNameDecimal: {}, TypeNameFloat: {}, TypeNameDouble: {},
	TypeNameDuration: {}, TypeNameDateTime: {}, TypeNameTime: {}, TypeNameDate: {},
	TypeNameGYearMonth: {}, TypeNameGYear: {}, TypeNameGMonthDay: {}, TypeNameGDay: {}, TypeNameGMonth: {},
	TypeNameHexBinary: {}, TypeNameBase64Binary: {}, TypeNameAnyURI: {}, TypeNameQName: {}, TypeNameNOTATION: {},
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

func registerBuiltin(name TypeName, validator TypeValidator, ws WhiteSpace, ordered bool) {
	nameStr := string(name)
	builtinRegistry[nameStr] = &BuiltinType{
		name:       nameStr,
		qname:      QName{Namespace: XSDNamespace, Local: nameStr},
		validator:  validator,
		whiteSpace: ws,
		ordered:    ordered,
	}
}

// Compile-time check that BuiltinType implements Type interface
var _ Type = (*BuiltinType)(nil)

// Compile-time check that BuiltinType implements SimpleTypeDefinition
var _ SimpleTypeDefinition = (*BuiltinType)(nil)

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

	// Fallback for types not in registry: create a string value
	st := &SimpleType{
		QName:   b.qname,
		variety: AtomicVariety,
	}
	st.MarkBuiltin()
	return NewStringValue(NewParsedValue(normalized, normalized), st), nil
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

	// Check if it's a built-in list type (NMTOKENS, IDREFS, ENTITIES)
	if isBuiltinListType(name) {
		// List type: length is number of items (space-separated)
		if len(strings.TrimSpace(value)) == 0 {
			return 0
		}
		return len(strings.Fields(value))
	}

	primitiveType := b.PrimitiveType()
	if primitiveType != nil {
		return measureLengthForPrimitive(value, TypeName(primitiveType.Name().Local))
	}

	// Fallback: character count
	return utf8.RuneCountInString(value)
}

// FundamentalFacets returns the fundamental facets for this built-in type
func (b *BuiltinType) FundamentalFacets() *FundamentalFacets {
	if b.fundamentalFacetsCache != nil {
		return b.fundamentalFacetsCache
	}

	// For primitive types, compute directly
	if isPrimitiveName(TypeName(b.name)) {
		b.fundamentalFacetsCache = ComputeFundamentalFacets(TypeName(b.name))
		return b.fundamentalFacetsCache
	}

	// For derived types, get facets from primitive type
	primitive := b.PrimitiveType()
	if primitive == nil {
		// Fallback: try computing from name (may return nil for unknown types)
		b.fundamentalFacetsCache = ComputeFundamentalFacets(TypeName(b.name))
		return b.fundamentalFacetsCache
	}

	if bt, ok := as[*BuiltinType](primitive); ok {
		b.fundamentalFacetsCache = bt.FundamentalFacets()
		return b.fundamentalFacetsCache
	}

	// If primitive is not BuiltinType, try to get facets from it
	facets := primitive.FundamentalFacets()
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

	// Primitive types have anySimpleType as base
	if isPrimitiveName(TypeName(b.name)) {
		return GetBuiltin(TypeNameAnySimpleType)
	}

	// For derived types, compute base type from type hierarchy
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
	// Map derived types to their bases according to XSD 1.0 type hierarchy
	if baseName, ok := builtinBaseTypes[TypeName(name)]; ok {
		return GetBuiltin(baseName)
	}
	// If not found in map, return anySimpleType as fallback (base of all simple types)
	return GetBuiltin(TypeNameAnySimpleType)
}

// PrimitiveType returns the primitive type for this built-in type
func (b *BuiltinType) PrimitiveType() Type {
	// Return cached value if available
	if b.primitiveTypeCache != nil {
		return b.primitiveTypeCache
	}

	// anySimpleType and anyType have no primitive type (they are abstract roots)
	if b.name == string(TypeNameAnySimpleType) || b.name == string(TypeNameAnyType) {
		return nil
	}

	// For primitive types, return self
	if isPrimitive(b.name) {
		b.primitiveTypeCache = b
		return b
	}

	// For derived types, follow base type chain
	base := b.BaseType()
	if base == nil {
		return nil
	}
	primitive := base.PrimitiveType()
	b.primitiveTypeCache = primitive
	return primitive
}

func init() {
	// Built-in complex type
	registerBuiltin(TypeNameAnyType, validateAnyType, WhiteSpacePreserve, false)

	// Base simple type (base of all simple types, must be registered before primitives)
	registerBuiltin(TypeNameAnySimpleType, validateAnySimpleType, WhiteSpacePreserve, false)

	// Primitive types (19 total)
	registerBuiltin(TypeNameString, validateString, WhiteSpacePreserve, false)
	registerBuiltin(TypeNameBoolean, validateBoolean, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameDecimal, validateDecimal, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameFloat, validateFloat, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameDouble, validateDouble, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameDuration, validateDuration, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameDateTime, validateDateTime, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameTime, validateTime, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameDate, validateDate, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameGYearMonth, validateGYearMonth, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameGYear, validateGYear, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameGMonthDay, validateGMonthDay, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameGDay, validateGDay, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameGMonth, validateGMonth, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameHexBinary, validateHexBinary, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameBase64Binary, validateBase64Binary, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameAnyURI, validateAnyURI, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameQName, validateQName, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameNOTATION, validateNOTATION, WhiteSpaceCollapse, false)

	// Derived string types
	registerBuiltin(TypeNameNormalizedString, validateNormalizedString, WhiteSpaceReplace, false)
	registerBuiltin(TypeNameToken, validateToken, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameLanguage, validateLanguage, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameName, validateName, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameNCName, validateNCName, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameID, validateID, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameIDREF, validateIDREF, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameIDREFS, validateIDREFS, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameENTITY, validateENTITY, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameENTITIES, validateENTITIES, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameNMTOKEN, validateNMTOKEN, WhiteSpaceCollapse, false)
	registerBuiltin(TypeNameNMTOKENS, validateNMTOKENS, WhiteSpaceCollapse, false)

	// Derived numeric types
	registerBuiltin(TypeNameInteger, validateInteger, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameLong, validateLong, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameInt, validateInt, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameShort, validateShort, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameByte, validateByte, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameNonNegativeInteger, validateNonNegativeInteger, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNamePositiveInteger, validatePositiveInteger, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameUnsignedLong, validateUnsignedLong, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameUnsignedInt, validateUnsignedInt, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameUnsignedShort, validateUnsignedShort, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameUnsignedByte, validateUnsignedByte, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameNonPositiveInteger, validateNonPositiveInteger, WhiteSpaceCollapse, true)
	registerBuiltin(TypeNameNegativeInteger, validateNegativeInteger, WhiteSpaceCollapse, true)
}
