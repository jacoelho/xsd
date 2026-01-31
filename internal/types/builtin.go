package types

import "unicode/utf8"

// XSDNamespace is the XML Schema namespace URI.
const XSDNamespace NamespaceURI = "http://www.w3.org/2001/XMLSchema"

// TypeValidator validates a string value according to a type's rules.
// It is used to implement validation logic for built-in XSD types.
// Returns an error if the value is invalid, nil otherwise.
type TypeValidator func(value string) error

// TypeValidatorBytes validates a byte slice value according to a type's rules.
// It avoids allocations when values are already in byte form.
type TypeValidatorBytes func(value []byte) error

// BuiltinType represents a built-in XSD type
type BuiltinType struct {
	primitiveTypeCache         Type
	validator                  TypeValidator
	validatorBytes             TypeValidatorBytes
	fundamentalFacetsCache     *FundamentalFacets
	simpleWrapper              *SimpleType
	qname                      QName
	name                       string
	whiteSpace                 WhiteSpace
	fundamentalFacetsComputing bool
	primitiveTypeComputing     bool
	ordered                    bool
}

type orderingFlag bool

const (
	unordered orderingFlag = false
	ordered   orderingFlag = true
)

var builtinRegistry = map[string]*BuiltinType{
	// built-in complex type
	string(TypeNameAnyType): newBuiltin(TypeNameAnyType, validateAnyType, nil, WhiteSpacePreserve, unordered),

	// base simple type (base of all simple types, must be registered before primitives)
	string(TypeNameAnySimpleType): newBuiltin(TypeNameAnySimpleType, validateAnySimpleType, nil, WhiteSpacePreserve, unordered),

	// primitive types (19 total)
	string(TypeNameString):       newBuiltin(TypeNameString, validateString, nil, WhiteSpacePreserve, unordered),
	string(TypeNameBoolean):      newBuiltin(TypeNameBoolean, validateBoolean, validateBooleanBytes, WhiteSpaceCollapse, unordered),
	string(TypeNameDecimal):      newBuiltin(TypeNameDecimal, validateDecimal, validateDecimalBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameFloat):        newBuiltin(TypeNameFloat, validateFloat, validateFloatBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameDouble):       newBuiltin(TypeNameDouble, validateDouble, validateDoubleBytes, WhiteSpaceCollapse, ordered),
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
	string(TypeNameNOTATION):     newBuiltin(TypeNameNOTATION, validateNOTATION, nil, WhiteSpaceCollapse, unordered),

	// derived string types
	string(TypeNameNormalizedString): newBuiltin(TypeNameNormalizedString, validateNormalizedString, nil, WhiteSpaceReplace, unordered),
	string(TypeNameToken):            newBuiltin(TypeNameToken, validateToken, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameLanguage):         newBuiltin(TypeNameLanguage, validateLanguage, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameName):             newBuiltin(TypeNameName, validateName, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameNCName):           newBuiltin(TypeNameNCName, validateNCName, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameID):               newBuiltin(TypeNameID, validateID, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameIDREF):            newBuiltin(TypeNameIDREF, validateIDREF, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameIDREFS):           newBuiltin(TypeNameIDREFS, validateIDREFS, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameENTITY):           newBuiltin(TypeNameENTITY, validateENTITY, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameENTITIES):         newBuiltin(TypeNameENTITIES, validateENTITIES, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameNMTOKEN):          newBuiltin(TypeNameNMTOKEN, validateNMTOKEN, nil, WhiteSpaceCollapse, unordered),
	string(TypeNameNMTOKENS):         newBuiltin(TypeNameNMTOKENS, validateNMTOKENS, nil, WhiteSpaceCollapse, unordered),

	// derived numeric types
	string(TypeNameInteger):            newBuiltin(TypeNameInteger, validateInteger, validateIntegerBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameLong):               newBuiltin(TypeNameLong, validateLong, validateLongBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameInt):                newBuiltin(TypeNameInt, validateInt, validateIntBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameShort):              newBuiltin(TypeNameShort, validateShort, validateShortBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameByte):               newBuiltin(TypeNameByte, validateByte, validateByteBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameNonNegativeInteger): newBuiltin(TypeNameNonNegativeInteger, validateNonNegativeInteger, validateNonNegativeIntegerBytes, WhiteSpaceCollapse, ordered),
	string(TypeNamePositiveInteger):    newBuiltin(TypeNamePositiveInteger, validatePositiveInteger, validatePositiveIntegerBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedLong):       newBuiltin(TypeNameUnsignedLong, validateUnsignedLong, validateUnsignedLongBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedInt):        newBuiltin(TypeNameUnsignedInt, validateUnsignedInt, validateUnsignedIntBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedShort):      newBuiltin(TypeNameUnsignedShort, validateUnsignedShort, validateUnsignedShortBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameUnsignedByte):       newBuiltin(TypeNameUnsignedByte, validateUnsignedByte, validateUnsignedByteBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameNonPositiveInteger): newBuiltin(TypeNameNonPositiveInteger, validateNonPositiveInteger, validateNonPositiveIntegerBytes, WhiteSpaceCollapse, ordered),
	string(TypeNameNegativeInteger):    newBuiltin(TypeNameNegativeInteger, validateNegativeInteger, validateNegativeIntegerBytes, WhiteSpaceCollapse, ordered),
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

// IsQNameOrNotationType reports whether this built-in type is QName or NOTATION.
func (b *BuiltinType) IsQNameOrNotationType() bool {
	if b == nil {
		return false
	}
	return IsQNameOrNotation(b.Name())
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
	normalized, err := NormalizeValue(lexical, b)
	if err != nil {
		return nil, err
	}

	err = b.Validate(normalized)
	if err != nil {
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
	typeCacheMu.Lock()
	for b.fundamentalFacetsCache == nil && b.fundamentalFacetsComputing {
		typeCacheCond.Wait()
	}
	if b.fundamentalFacetsCache != nil {
		cached := b.fundamentalFacetsCache
		typeCacheMu.Unlock()
		return cached
	}
	b.fundamentalFacetsComputing = true
	typeCacheMu.Unlock()

	computed := b.computeFundamentalFacets()
	if computed == nil {
		typeCacheMu.Lock()
		b.fundamentalFacetsComputing = false
		typeCacheMu.Unlock()
		typeCacheCond.Broadcast()
		return nil
	}

	typeCacheMu.Lock()
	if b.fundamentalFacetsCache == nil {
		b.fundamentalFacetsCache = computed
	}
	b.fundamentalFacetsComputing = false
	cached := b.fundamentalFacetsCache
	typeCacheMu.Unlock()
	typeCacheCond.Broadcast()
	return cached
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
	typeCacheMu.Lock()
	for b.primitiveTypeCache == nil && b.primitiveTypeComputing {
		typeCacheCond.Wait()
	}
	if b.primitiveTypeCache != nil {
		cached := b.primitiveTypeCache
		typeCacheMu.Unlock()
		return cached
	}
	b.primitiveTypeComputing = true
	typeCacheMu.Unlock()

	primitive := b.computePrimitiveType()
	if primitive == nil {
		typeCacheMu.Lock()
		b.primitiveTypeComputing = false
		typeCacheMu.Unlock()
		typeCacheCond.Broadcast()
		return nil
	}

	typeCacheMu.Lock()
	if b.primitiveTypeCache == nil {
		b.primitiveTypeCache = primitive
	}
	b.primitiveTypeComputing = false
	cached := b.primitiveTypeCache
	typeCacheMu.Unlock()
	typeCacheCond.Broadcast()
	return cached
}

func (b *BuiltinType) computePrimitiveType() Type {
	// anySimpleType and anyType have no primitive type (they are abstract roots)
	if b.name == string(TypeNameAnySimpleType) || b.name == string(TypeNameAnyType) {
		return nil
	}

	// for primitive types, return self
	if isPrimitive(b.name) {
		return b
	}

	// for derived types, follow base type chain
	base := b.BaseType()
	if base == nil {
		return nil
	}
	return base.PrimitiveType()
}
