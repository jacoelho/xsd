package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SimpleType represents a simple type definition
type SimpleType struct {
	ItemType               Type
	ResolvedBase           Type
	primitiveType          Type
	Restriction            *Restriction
	List                   *ListType
	Union                  *UnionType
	fundamentalFacetsCache *FundamentalFacets
	QName                  QName
	SourceNamespace        NamespaceURI
	MemberTypes            []Type
	whiteSpace             WhiteSpace
	variety                SimpleTypeVariety
	Final                  DerivationSet
	qnameOrNotationReady   bool
	qnameOrNotation        bool
	whiteSpaceExplicit     bool
	builtin                bool
}

// NewSimpleType creates a new simple type with the provided name and namespace.
func NewSimpleType(name QName, sourceNamespace NamespaceURI) *SimpleType {
	return &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
	}
}

// NewSimpleTypeFromParsed validates a parsed simple type and returns it if valid.
func NewSimpleTypeFromParsed(simpleType *SimpleType) (*SimpleType, error) {
	if simpleType == nil {
		return nil, fmt.Errorf("simpleType is nil")
	}
	if err := validateSimpleTypeDefinition(simpleType); err != nil {
		return nil, err
	}
	return simpleType, nil
}

func validateSimpleTypeDefinition(simpleType *SimpleType) error {
	switch simpleType.Variety() {
	case AtomicVariety:
		if !simpleType.IsBuiltin() && simpleType.Restriction == nil {
			return fmt.Errorf("atomic simpleType must have a restriction")
		}
	case ListVariety:
		if simpleType.List == nil && simpleType.Restriction == nil {
			return fmt.Errorf("list simpleType must have a list definition or restriction")
		}
	case UnionVariety:
		if simpleType.Union == nil && simpleType.Restriction == nil {
			return fmt.Errorf("union simpleType must have a union definition or restriction")
		}
	default:
		if !simpleType.IsBuiltin() && simpleType.Restriction == nil && simpleType.List == nil && simpleType.Union == nil {
			return fmt.Errorf("simpleType must have a derivation")
		}
	}

	if simpleType.Restriction != nil {
		if simpleType.Restriction.Base.IsZero() && simpleType.ResolvedBase == nil {
			return fmt.Errorf("simpleType restriction must have a base type")
		}
		baseType := restrictionBaseType(simpleType)
		if baseType != nil {
			if err := validateRestrictionFacetApplicability(simpleType.Restriction.Facets, baseType); err != nil {
				return err
			}
		}
	}

	if simpleType.List != nil {
		if simpleType.List.InlineItemType != nil && !simpleType.List.ItemType.IsZero() {
			return fmt.Errorf("list simpleType cannot have both inline and named item types")
		}
		if simpleType.List.InlineItemType == nil && simpleType.List.ItemType.IsZero() && simpleType.ItemType == nil {
			return fmt.Errorf("list simpleType must declare an item type")
		}
	}

	if simpleType.Union != nil {
		if len(simpleType.Union.InlineTypes) == 0 && len(simpleType.Union.MemberTypes) == 0 && len(simpleType.MemberTypes) == 0 {
			return fmt.Errorf("union simpleType must declare member types")
		}
	}

	return nil
}

func restrictionBaseType(simpleType *SimpleType) Type {
	if simpleType == nil || simpleType.Restriction == nil {
		return nil
	}
	if simpleType.ResolvedBase != nil {
		if isNilType(simpleType.ResolvedBase) {
			return nil
		}
		return simpleType.ResolvedBase
	}
	if simpleType.Restriction.Base.IsZero() {
		return nil
	}
	base := GetBuiltinNS(simpleType.Restriction.Base.Namespace, simpleType.Restriction.Base.Local)
	if base == nil {
		return nil
	}
	return base
}

type facetNamer interface {
	Name() string
}

func validateRestrictionFacetApplicability(facets []any, baseType Type) error {
	if isNilType(baseType) {
		return nil
	}
	for _, facet := range facets {
		namer, ok := facet.(facetNamer)
		if !ok {
			continue
		}
		facetName := namer.Name()
		if err := ValidateFacetApplicability(facetName, baseType, baseType.Name()); err != nil {
			return err
		}
	}
	return nil
}

func isNilType(typ Type) bool {
	// interface-typed nils need a type switch to detect nil pointers.
	switch value := typ.(type) {
	case nil:
		return true
	case *BuiltinType:
		return value == nil
	case *SimpleType:
		return value == nil
	case *ComplexType:
		return value == nil
	default:
		return false
	}
}

// Name returns the QName of the simple type.
func (s *SimpleType) Name() QName {
	return s.QName
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (s *SimpleType) ComponentName() QName {
	return s.QName
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (s *SimpleType) DeclaredNamespace() NamespaceURI {
	return s.SourceNamespace
}

// Copy creates a copy of the simple type with remapped QNames.
func (s *SimpleType) Copy(opts CopyOptions) *SimpleType {
	clone := *s
	clone.QName = opts.RemapQName(s.QName)
	clone.SourceNamespace = opts.SourceNamespace
	copyInline := func(inline *SimpleType) *SimpleType {
		if inline == nil {
			return nil
		}
		inlineCopy := inline.Copy(opts)
		if inline.QName.IsZero() {
			inlineCopy.QName = inline.QName
		}
		inlineCopy.SourceNamespace = opts.SourceNamespace
		return inlineCopy
	}
	clone.Restriction = copyRestriction(s.Restriction, opts)
	// remap union memberTypes and inline types if present
	if s.Union != nil {
		unionCopy := *s.Union
		if len(unionCopy.MemberTypes) > 0 {
			memberTypes := make([]QName, len(unionCopy.MemberTypes))
			for i, mt := range unionCopy.MemberTypes {
				// only remap if not in XSD namespace (built-in types)
				if mt.Namespace.IsEmpty() {
					memberTypes[i] = opts.RemapQName(mt)
				} else {
					memberTypes[i] = mt
				}
			}
			unionCopy.MemberTypes = memberTypes
		}
		if len(unionCopy.InlineTypes) > 0 {
			inlineCopies := make([]*SimpleType, len(unionCopy.InlineTypes))
			for i, inline := range unionCopy.InlineTypes {
				inlineCopies[i] = copyInline(inline)
			}
			unionCopy.InlineTypes = inlineCopies
		}
		clone.Union = &unionCopy
	}
	// remap list itemType if present
	if s.List != nil {
		listCopy := *s.List
		if !listCopy.ItemType.IsZero() && listCopy.ItemType.Namespace.IsEmpty() {
			listCopy.ItemType = opts.RemapQName(listCopy.ItemType)
		}
		listCopy.InlineItemType = copyInline(listCopy.InlineItemType)
		clone.List = &listCopy
	}
	return &clone
}

// IsBuiltin reports whether the simple type is built-in.
func (s *SimpleType) IsBuiltin() bool {
	return s.builtin
}

// IsPlaceholderSimpleType reports whether simpleType represents an unresolved type reference.
func IsPlaceholderSimpleType(simpleType *SimpleType) bool {
	if simpleType == nil || simpleType.IsBuiltin() || simpleType.QName.IsZero() {
		return false
	}
	return simpleType.Restriction == nil && simpleType.List == nil && simpleType.Union == nil
}

// BaseType returns the base type for this simple type
// If ResolvedBase is nil, returns anySimpleType (the base of all simple types)
func (s *SimpleType) BaseType() Type {
	if s.ResolvedBase == nil {
		return GetBuiltin(TypeNameAnySimpleType)
	}
	return s.ResolvedBase
}

// ResolvedBaseType returns the resolved base type, or nil if at root.
// Implements DerivedType interface.
func (s *SimpleType) ResolvedBaseType() Type {
	return s.ResolvedBase
}

// FundamentalFacets returns the fundamental facets for this simple type
func (s *SimpleType) FundamentalFacets() *FundamentalFacets {
	// return cached value if available
	if s.fundamentalFacetsCache != nil {
		return s.fundamentalFacetsCache
	}

	primitive := s.PrimitiveType()
	if primitive == nil {
		return nil
	}

	// for built-in types accessed as Type interface
	if builtinType, ok := AsBuiltinType(primitive); ok {
		facets := builtinType.FundamentalFacets()
		s.fundamentalFacetsCache = facets
		return facets
	}

	// for SimpleType that is built-in
	if simpleType, ok := AsSimpleType(primitive); ok {
		if simpleType.IsBuiltin() {
			facets := ComputeFundamentalFacets(TypeName(simpleType.QName.Local))
			s.fundamentalFacetsCache = facets
			return facets
		}
	}

	return nil
}

// SetFundamentalFacets sets the cached fundamental facets for this simple type
func (s *SimpleType) SetFundamentalFacets(facets *FundamentalFacets) {
	s.fundamentalFacetsCache = facets
}

// WhiteSpace returns the whitespace normalization for this simple type
func (s *SimpleType) WhiteSpace() WhiteSpace {
	return s.whiteSpace
}

// SetWhiteSpace sets the whitespace normalization for this simple type
func (s *SimpleType) SetWhiteSpace(ws WhiteSpace) {
	s.whiteSpace = ws
}

// SetWhiteSpaceExplicit sets the whitespace normalization and marks it as explicitly set.
// This is used when parsing a whiteSpace facet in a restriction.
func (s *SimpleType) SetWhiteSpaceExplicit(ws WhiteSpace) {
	s.whiteSpace = ws
	s.whiteSpaceExplicit = true
}

// WhiteSpaceExplicit returns true if whiteSpace was explicitly set in this type's restriction.
func (s *SimpleType) WhiteSpaceExplicit() bool {
	return s.whiteSpaceExplicit
}

// MeasureLength returns length in type-appropriate units (octets, items, or characters).
// Implements LengthMeasurable interface.
func (s *SimpleType) MeasureLength(value string) int {
	// check if this type is itself a list type
	if s.List != nil {
		// list type: length is number of items (space-separated)
		if strings.TrimSpace(value) == "" {
			return 0
		}
		return countXMLFields(value)
	}

	// check if this type restricts a list type
	if s.Restriction != nil {
		// first check ResolvedBase if available
		if s.ResolvedBase != nil {
			if lengthMeasurer, ok := as[LengthMeasurable](s.ResolvedBase); ok {
				// check if base is a list type
				if baseSimpleType, ok := AsSimpleType(s.ResolvedBase); ok && baseSimpleType.List != nil {
					// restriction of list type: length is number of items
					if strings.TrimSpace(value) == "" {
						return 0
					}
					return countXMLFields(value)
				}
				if builtinType, ok := AsBuiltinType(s.ResolvedBase); ok && isBuiltinListType(builtinType.Name().Local) {
					// restriction of built-in list type: length is number of items
					if strings.TrimSpace(value) == "" {
						return 0
					}
					return countXMLFields(value)
				}
				// otherwise delegate to base type
				return lengthMeasurer.MeasureLength(value)
			}
		}
		// fallback: check if Restriction.Base QName suggests it's a list type
		if !s.Restriction.Base.IsZero() {
			baseLocal := s.Restriction.Base.Local
			if strings.HasPrefix(strings.ToLower(baseLocal), "list") ||
				isBuiltinListType(baseLocal) {
				// likely a list type - count items
				if strings.TrimSpace(value) == "" {
					return 0
				}
				return countXMLFields(value)
			}
		}
	}

	// for user-defined types, delegate to primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType != nil {
		if lengthMeasurer, ok := as[LengthMeasurable](primitiveType); ok {
			return lengthMeasurer.MeasureLength(value)
		}
		// fallback: use primitive name directly
		return measureLengthForPrimitive(value, TypeName(primitiveType.Name().Local))
	}
	// fallback: character count
	return utf8.RuneCountInString(value)
}

func countXMLFields(value string) int {
	// count fields delimited by XML whitespace (#x20/#x9/#xD/#xA) without allocations.
	count := 0
	inField := false
	for i := 0; i < len(value); i++ {
		if isXMLWhitespaceByte(value[i]) {
			if inField {
				count++
				inField = false
			}
			continue
		}
		inField = true
	}
	if inField {
		count++
	}
	return count
}

// Variety returns the simple type variety.
func (s *SimpleType) Variety() SimpleTypeVariety {
	return s.variety
}

// SetVariety sets the simple type variety
func (s *SimpleType) SetVariety(v SimpleTypeVariety) {
	s.variety = v
}

// Validate checks if a lexical value is valid for this type
func (s *SimpleType) Validate(lexical string) error {
	normalized, err := NormalizeValue(lexical, s)
	if err != nil {
		return err
	}

	// for built-in types, use built-in validator
	if s.IsBuiltin() {
		if builtinType := GetBuiltinNS(s.QName.Namespace, s.QName.Local); builtinType != nil {
			return builtinType.Validate(normalized)
		}
	}

	// for user-defined types with restrictions, validate against base type and facets
	if s.Restriction != nil {
		baseType := GetBuiltinNS(s.Restriction.Base.Namespace, s.Restriction.Base.Local)
		if baseType != nil {
			if err := baseType.Validate(normalized); err != nil {
				return err
			}
		}
		// facet validation is done separately in the validator
	}

	return nil
}

// ParseValue converts a lexical value to a TypedValue
func (s *SimpleType) ParseValue(lexical string) (TypedValue, error) {
	normalized, err := NormalizeValue(lexical, s)
	if err != nil {
		return nil, err
	}

	// first, try to parse based on the type's own name (for built-in types)
	if s.IsBuiltin() {
		typeName := TypeName(s.QName.Local)
		if result, err := ParseValueForType(normalized, typeName, s); err == nil {
			return result, nil
		}
	}

	// for user-defined types or if built-in type not handled above, use primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType == nil {
		return nil, fmt.Errorf("cannot determine primitive type")
	}

	primitiveST, ok := as[*SimpleType](primitiveType)
	if !ok {
		// try BuiltinType
		if builtinType, ok := AsBuiltinType(primitiveType); ok {
			return builtinType.ParseValue(normalized)
		}
		return nil, fmt.Errorf("primitive type is not a SimpleType or BuiltinType")
	}

	primitiveName := TypeName(primitiveST.QName.Local)
	return ParseValueForType(normalized, primitiveName, s)
}

// MarkBuiltin marks the type as a built-in type
func (s *SimpleType) MarkBuiltin() {
	s.builtin = true
}

// PrimitiveType returns the ultimate primitive base type for this simple type
func (s *SimpleType) PrimitiveType() Type {
	// return cached value if available
	if s.primitiveType != nil {
		return s.primitiveType
	}

	primitive := s.getPrimitiveTypeWithVisited(make(map[*SimpleType]bool))
	s.primitiveType = primitive
	return primitive
}

// SetPrimitiveType sets the cached primitive type for this simple type
func (s *SimpleType) SetPrimitiveType(primitive Type) {
	s.primitiveType = primitive
}

// IsQNameOrNotationType reports whether this type derives from QName or NOTATION.
func (s *SimpleType) IsQNameOrNotationType() bool {
	if s == nil {
		return false
	}
	if s.qnameOrNotationReady {
		return s.qnameOrNotation
	}
	value := s.computeQNameOrNotationType()
	s.qnameOrNotation = value
	s.qnameOrNotationReady = true
	return value
}

// SetQNameOrNotationType stores the precomputed QName/NOTATION derivation flag.
func (s *SimpleType) SetQNameOrNotationType(value bool) {
	if s == nil {
		return
	}
	s.qnameOrNotation = value
	s.qnameOrNotationReady = true
}

func (s *SimpleType) computeQNameOrNotationType() bool {
	if s == nil || s.Variety() == ListVariety {
		return false
	}
	if IsQNameOrNotation(s.QName) {
		return true
	}
	if s.Restriction != nil && !s.Restriction.Base.IsZero() {
		base := s.Restriction.Base
		if (base.Namespace == XSDNamespace || base.Namespace.IsEmpty()) &&
			(base.Local == string(TypeNameQName) || base.Local == string(TypeNameNOTATION)) {
			return true
		}
	}
	switch base := s.ResolvedBase.(type) {
	case *SimpleType:
		if base.IsQNameOrNotationType() {
			return true
		}
	case *BuiltinType:
		if base.IsQNameOrNotationType() {
			return true
		}
	}
	if s.primitiveType != nil && IsQNameOrNotation(s.primitiveType.Name()) {
		return true
	}
	return false
}

// getPrimitiveTypeWithVisited is the internal implementation with cycle detection
func (s *SimpleType) getPrimitiveTypeWithVisited(visited map[*SimpleType]bool) Type {
	// if already computed, return it
	if s.primitiveType != nil {
		return s.primitiveType
	}

	if visited[s] {
		// circular reference detected - return nil to break the cycle
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	if primitive := s.primitiveFromSelf(); primitive != nil {
		s.primitiveType = primitive
		return primitive
	}

	if primitive := s.primitiveFromRestriction(visited); primitive != nil {
		s.primitiveType = primitive
		return primitive
	}

	if primitive := s.primitiveFromList(visited); primitive != nil {
		s.primitiveType = primitive
		return primitive
	}

	if primitive := s.primitiveFromUnion(visited); primitive != nil {
		s.primitiveType = primitive
		return primitive
	}

	return nil
}

func (s *SimpleType) primitiveFromSelf() Type {
	if s.builtin && s.Variety() == AtomicVariety {
		if isPrimitiveName(TypeName(s.QName.Local)) && s.QName.Namespace == XSDNamespace {
			return s
		}
	}
	return nil
}

func (s *SimpleType) primitiveFromRestriction(visited map[*SimpleType]bool) Type {
	if s.Restriction == nil {
		return nil
	}
	if s.ResolvedBase != nil {
		return primitiveFromBaseType(s.ResolvedBase, visited)
	}
	if s.Restriction.Base.IsZero() {
		return nil
	}
	if s.Restriction.Base.Namespace != XSDNamespace {
		return nil
	}
	builtinType := GetBuiltin(TypeName(s.Restriction.Base.Local))
	if builtinType == nil {
		return nil
	}
	return builtinType.PrimitiveType()
}

func (s *SimpleType) primitiveFromList(visited map[*SimpleType]bool) Type {
	if s.List == nil || s.ItemType == nil {
		return nil
	}
	return primitiveFromBaseType(s.ItemType, visited)
}

func (s *SimpleType) primitiveFromUnion(visited map[*SimpleType]bool) Type {
	if s.Union == nil {
		return nil
	}
	var commonPrimitive Type
	for _, memberType := range s.MemberTypes {
		memberPrimitive := primitiveFromBaseType(memberType, visited)
		if memberPrimitive == nil {
			continue
		}
		if commonPrimitive == nil {
			commonPrimitive = memberPrimitive
			continue
		}
		if commonPrimitive != memberPrimitive {
			return nil
		}
	}
	return commonPrimitive
}

func primitiveFromBaseType(base Type, visited map[*SimpleType]bool) Type {
	switch typed := base.(type) {
	case *SimpleType:
		return typed.getPrimitiveTypeWithVisited(visited)
	case *BuiltinType:
		return typed.PrimitiveType()
	default:
		return nil
	}
}
