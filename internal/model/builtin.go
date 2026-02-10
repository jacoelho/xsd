package model

import (
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/xmlnames"
)

// XSDNamespace is the XML Schema namespace URI.
const XSDNamespace NamespaceURI = "http://www.w3.org/2001/XMLSchema"

// XMLNamespace is the XML namespace URI.
const XMLNamespace NamespaceURI = NamespaceURI(xmlnames.XMLNamespace)

// TypeValidator validates a string value according to a type's rules.
// It is used to implement validation logic for built-in XSD types.
// Returns an error if the value is invalid, nil otherwise.
type TypeValidator func(value string) error

// TypeValidatorBytes validates a byte slice value according to a type's rules.
// It avoids allocations when values are already in byte form.
type TypeValidatorBytes func(value []byte) error

// BuiltinType represents a built-in XSD type
type BuiltinType struct {
	primitiveType     Type
	validator         TypeValidator
	validatorBytes    TypeValidatorBytes
	fundamentalFacets *FundamentalFacets
	simpleWrapper     *SimpleType
	qname             QName
	name              string
	whiteSpace        WhiteSpace
	ordered           bool
}

type orderingFlag bool

const (
	unordered orderingFlag = false
	ordered   orderingFlag = true
)

var defaultBuiltinRegistry = func() *builtinRegistry {
	registry := newBuiltinRegistry(map[string]*BuiltinType{
		// built-in complex type
		string(TypeNameAnyType): newBuiltin(TypeNameAnyType, validateAnyType, nil, WhiteSpacePreserve, unordered),

		// base simple type (base of all simple types, must be registered before primitives)
		string(TypeNameAnySimpleType): newBuiltin(TypeNameAnySimpleType, validateAnySimpleType, nil, WhiteSpacePreserve, unordered),

		// primitive types (19 total)
		string(TypeNameString):       newBuiltin(TypeNameString, validateString, nil, WhiteSpacePreserve, unordered),
		string(TypeNameBoolean):      newBuiltin(TypeNameBoolean, validateBoolean, byteValidator(validateBoolean), WhiteSpaceCollapse, unordered),
		string(TypeNameDecimal):      newBuiltin(TypeNameDecimal, validateDecimal, byteValidator(validateDecimal), WhiteSpaceCollapse, ordered),
		string(TypeNameFloat):        newBuiltin(TypeNameFloat, validateFloat, byteValidator(validateFloat), WhiteSpaceCollapse, ordered),
		string(TypeNameDouble):       newBuiltin(TypeNameDouble, validateDouble, byteValidator(validateDouble), WhiteSpaceCollapse, ordered),
		string(TypeNameDuration):     newBuiltin(TypeNameDuration, validateDuration, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameDateTime):     newBuiltin(TypeNameDateTime, validateDateTime, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameTime):         newBuiltin(TypeNameTime, validateTime, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameDate):         newBuiltin(TypeNameDate, validateDate, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameGYearMonth):   newBuiltin(TypeNameGYearMonth, validateGYearMonth, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameGYear):        newBuiltin(TypeNameGYear, validateGYear, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameGMonthDay):    newBuiltin(TypeNameGMonthDay, validateGMonthDay, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameGDay):         newBuiltin(TypeNameGDay, validateGDay, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameGMonth):       newBuiltin(TypeNameGMonth, validateGMonth, nil, WhiteSpaceCollapse, ordered),
		string(TypeNameHexBinary):    newBuiltin(TypeNameHexBinary, validateHexBinary, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameBase64Binary): newBuiltin(TypeNameBase64Binary, validateBase64Binary, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameAnyURI):       newBuiltin(TypeNameAnyURI, validateAnyURI, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameQName):        newBuiltin(TypeNameQName, validateQName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameNOTATION):     newBuiltin(TypeNameNOTATION, validateQName, nil, WhiteSpaceCollapse, unordered),

		// derived string types
		string(TypeNameNormalizedString): newBuiltin(TypeNameNormalizedString, validateNormalizedString, nil, WhiteSpaceReplace, unordered),
		string(TypeNameToken):            newBuiltin(TypeNameToken, validateToken, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameLanguage):         newBuiltin(TypeNameLanguage, validateLanguage, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameName):             newBuiltin(TypeNameName, validateName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameNCName):           newBuiltin(TypeNameNCName, validateNCName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameID):               newBuiltin(TypeNameID, validateNCName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameIDREF):            newBuiltin(TypeNameIDREF, validateNCName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameIDREFS):           newBuiltin(TypeNameIDREFS, validateIDREFS, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameENTITY):           newBuiltin(TypeNameENTITY, validateNCName, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameENTITIES):         newBuiltin(TypeNameENTITIES, validateENTITIES, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameNMTOKEN):          newBuiltin(TypeNameNMTOKEN, validateNMTOKEN, nil, WhiteSpaceCollapse, unordered),
		string(TypeNameNMTOKENS):         newBuiltin(TypeNameNMTOKENS, validateNMTOKENS, nil, WhiteSpaceCollapse, unordered),

		// derived numeric types
		string(TypeNameInteger):            newBuiltin(TypeNameInteger, validateInteger, byteValidator(validateInteger), WhiteSpaceCollapse, ordered),
		string(TypeNameLong):               newBuiltin(TypeNameLong, validateLong, byteValidator(validateLong), WhiteSpaceCollapse, ordered),
		string(TypeNameInt):                newBuiltin(TypeNameInt, validateInt, byteValidator(validateInt), WhiteSpaceCollapse, ordered),
		string(TypeNameShort):              newBuiltin(TypeNameShort, validateShort, byteValidator(validateShort), WhiteSpaceCollapse, ordered),
		string(TypeNameByte):               newBuiltin(TypeNameByte, validateByte, byteValidator(validateByte), WhiteSpaceCollapse, ordered),
		string(TypeNameNonNegativeInteger): newBuiltin(TypeNameNonNegativeInteger, validateNonNegativeInteger, byteValidator(validateNonNegativeInteger), WhiteSpaceCollapse, ordered),
		string(TypeNamePositiveInteger):    newBuiltin(TypeNamePositiveInteger, validatePositiveInteger, byteValidator(validatePositiveInteger), WhiteSpaceCollapse, ordered),
		string(TypeNameUnsignedLong):       newBuiltin(TypeNameUnsignedLong, validateUnsignedLong, byteValidator(validateUnsignedLong), WhiteSpaceCollapse, ordered),
		string(TypeNameUnsignedInt):        newBuiltin(TypeNameUnsignedInt, validateUnsignedInt, byteValidator(validateUnsignedInt), WhiteSpaceCollapse, ordered),
		string(TypeNameUnsignedShort):      newBuiltin(TypeNameUnsignedShort, validateUnsignedShort, byteValidator(validateUnsignedShort), WhiteSpaceCollapse, ordered),
		string(TypeNameUnsignedByte):       newBuiltin(TypeNameUnsignedByte, validateUnsignedByte, byteValidator(validateUnsignedByte), WhiteSpaceCollapse, ordered),
		string(TypeNameNonPositiveInteger): newBuiltin(TypeNameNonPositiveInteger, validateNonPositiveInteger, byteValidator(validateNonPositiveInteger), WhiteSpaceCollapse, ordered),
		string(TypeNameNegativeInteger):    newBuiltin(TypeNameNegativeInteger, validateNegativeInteger, byteValidator(validateNegativeInteger), WhiteSpaceCollapse, ordered),
	})
	initializeBuiltinRegistry(registry)
	return registry
}()

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

// GetBuiltin returns a built-in XSD type by local name.
func GetBuiltin(name TypeName) *BuiltinType {
	return defaultBuiltinRegistry.get(name)
}

// GetBuiltinNS returns a built-in XSD type for an expanded name.
func GetBuiltinNS(namespace NamespaceURI, local string) *BuiltinType {
	return defaultBuiltinRegistry.getNS(namespace, local)
}

func newBuiltin(name TypeName, validator TypeValidator, validatorBytes TypeValidatorBytes, ws WhiteSpace, ordering orderingFlag) *BuiltinType {
	nameStr := string(name)
	builtin := &BuiltinType{
		name:           nameStr,
		qname:          QName{Namespace: XSDNamespace, Local: nameStr},
		validator:      validator,
		validatorBytes: validatorBytes,
		whiteSpace:     ws,
		ordered:        bool(ordering),
	}
	if isPrimitiveName(name) {
		builtin.primitiveType = builtin
		builtin.fundamentalFacets = ComputeFundamentalFacets(name)
	}
	builtin.simpleWrapper = newBuiltinSimpleType(builtin)
	return builtin
}

func newBuiltinSimpleType(builtin *BuiltinType) *SimpleType {
	if builtin == nil {
		return nil
	}
	st := &SimpleType{
		QName:           builtin.qname,
		SourceNamespace: XSDNamespace,
		builtin:         true,
		whiteSpace:      builtin.whiteSpace,
	}
	if itemName, ok := builtinListItemTypeName(builtin.name); ok {
		st.List = &ListType{
			ItemType: QName{Namespace: XSDNamespace, Local: string(itemName)},
		}
	}
	return st
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

// Validate validates a value against this type
func (b *BuiltinType) Validate(value string) error {
	if b == nil || b.validator == nil {
		return nil
	}
	return b.validator(value)
}

// ValidateBytes validates a byte slice value when a byte validator exists.
// It returns false when no byte validator is configured.
func (b *BuiltinType) ValidateBytes(value []byte) (bool, error) {
	if b == nil || b.validatorBytes == nil {
		return false, nil
	}
	return true, b.validatorBytes(value)
}

// HasByteValidator reports whether this type provides a byte validator.
func (b *BuiltinType) HasByteValidator() bool {
	return b != nil && b.validatorBytes != nil
}

// ParseValue converts a lexical value to a TypedValue
func (b *BuiltinType) ParseValue(lexical string) (TypedValue, error) {
	normalized, err := normalizeValue(lexical, b)
	if err != nil {
		return nil, err
	}

	err = b.Validate(normalized)
	if err != nil {
		return nil, err
	}

	typeName := TypeName(b.name)
	result, err := parseValueForType(normalized, typeName, b)
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
	if isBuiltinListTypeName(name) {
		// list type: length is number of items (space-separated)
		return countXMLFields(value)
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
	if b == nil {
		return nil
	}
	if b.fundamentalFacets != nil {
		return b.fundamentalFacets
	}
	return b.computeFundamentalFacets()
}

func (b *BuiltinType) computeFundamentalFacets() *FundamentalFacets {
	typeName := TypeName(b.name)

	if isPrimitiveName(typeName) {
		return ComputeFundamentalFacets(typeName)
	}

	primitive := b.computePrimitiveType()
	if primitive == nil {
		return ComputeFundamentalFacets(typeName)
	}

	if bt, ok := as[*BuiltinType](primitive); ok {
		return bt.FundamentalFacets()
	}

	return primitive.FundamentalFacets()
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
	if b == nil {
		return nil
	}
	if b.primitiveType != nil {
		return b.primitiveType
	}
	return b.computePrimitiveType()
}

func (b *BuiltinType) computePrimitiveType() Type {
	// anySimpleType and anyType have no primitive type (they are abstract roots)
	if b.name == string(TypeNameAnySimpleType) || b.name == string(TypeNameAnyType) {
		return nil
	}

	// for primitive types, return self
	if isPrimitiveName(TypeName(b.name)) {
		return b
	}

	// for derived types, follow base type chain
	base := b.BaseType()
	if base == nil {
		return nil
	}
	return base.PrimitiveType()
}

func initializeBuiltinRegistry(registry *builtinRegistry) {
	if registry == nil {
		return
	}
	for _, builtin := range registry.ordered {
		if builtin == nil {
			continue
		}
		if primitive := resolveBuiltinPrimitive(registry, TypeName(builtin.name)); primitive != nil {
			builtin.primitiveType = primitive
		}
	}
	for _, builtin := range registry.ordered {
		if builtin == nil {
			continue
		}
		switch primitive := builtin.primitiveType.(type) {
		case *BuiltinType:
			builtin.fundamentalFacets = ComputeFundamentalFacets(TypeName(primitive.name))
		case nil:
			builtin.fundamentalFacets = nil
		default:
			builtin.fundamentalFacets = primitive.FundamentalFacets()
		}
	}
}

func resolveBuiltinPrimitive(registry *builtinRegistry, name TypeName) Type {
	switch name {
	case TypeNameAnyType, TypeNameAnySimpleType:
		return nil
	}
	current := name
	for {
		if isPrimitiveName(current) {
			return registry.get(current)
		}
		base, ok := builtinBaseTypes[current]
		if !ok {
			return nil
		}
		current = base
	}
}
