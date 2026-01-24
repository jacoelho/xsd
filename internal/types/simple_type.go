package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SimpleType represents a simple type definition
type SimpleType struct {
	ItemType                   Type
	ResolvedBase               Type
	primitiveType              Type
	Restriction                *Restriction
	List                       *ListType
	Union                      *UnionType
	fundamentalFacetsCache     *FundamentalFacets
	identityListItemType       Type
	QName                      QName
	SourceNamespace            NamespaceURI
	MemberTypes                []Type
	identityMemberTypes        []Type
	whiteSpace                 WhiteSpace
	Final                      DerivationSet
	qnameOrNotationReady       bool
	qnameOrNotation            bool
	identityNormalizationReady bool
	identityNormalizable       bool
	whiteSpaceExplicit         bool
	builtin                    bool
}

// NewAtomicSimpleType creates a simple type derived by restriction.
func NewAtomicSimpleType(name QName, sourceNamespace NamespaceURI, restriction *Restriction) (*SimpleType, error) {
	st := &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
		Restriction:     restriction,
	}
	if restriction != nil && restriction.SimpleType != nil {
		st.ResolvedBase = restriction.SimpleType
	}
	if err := validateSimpleTypeDefinition(st); err != nil {
		return nil, err
	}
	return st, nil
}

// NewListSimpleType creates a simple type derived by list.
func NewListSimpleType(name QName, sourceNamespace NamespaceURI, list *ListType, restriction *Restriction) (*SimpleType, error) {
	if list == nil {
		return nil, fmt.Errorf("list simpleType must have a list definition")
	}
	st := &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
		List:            list,
		Restriction:     restriction,
		whiteSpace:      WhiteSpaceCollapse,
	}
	if list.InlineItemType != nil {
		st.ItemType = list.InlineItemType
	}
	if err := validateSimpleTypeDefinition(st); err != nil {
		return nil, err
	}
	return st, nil
}

// NewUnionSimpleType creates a simple type derived by union.
func NewUnionSimpleType(name QName, sourceNamespace NamespaceURI, union *UnionType) (*SimpleType, error) {
	if union == nil {
		return nil, fmt.Errorf("union simpleType must have a union definition")
	}
	st := &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
		Union:           union,
		whiteSpace:      WhiteSpaceCollapse,
	}
	if err := validateSimpleTypeDefinition(st); err != nil {
		return nil, err
	}
	return st, nil
}

// NewBuiltinSimpleType creates a SimpleType wrapper for a built-in type name.
func NewBuiltinSimpleType(name TypeName) (*SimpleType, error) {
	builtin := GetBuiltin(name)
	if builtin == nil {
		return nil, fmt.Errorf("unknown built-in type %s", name)
	}
	st := newBuiltinSimpleType(builtin)
	if st == nil {
		return nil, fmt.Errorf("failed to build built-in type %s", name)
	}
	if st.List != nil {
		if itemName, ok := builtinListItemTypeName(string(name)); ok {
			if itemType := GetBuiltin(itemName); itemType != nil {
				st.ItemType = itemType
			}
		}
	}
	st.fundamentalFacetsCache = builtin.FundamentalFacets()
	if err := validateSimpleTypeDefinition(st); err != nil {
		return nil, err
	}
	return st, nil
}

// NewPlaceholderSimpleType creates a simple type placeholder for unresolved references.
func NewPlaceholderSimpleType(name QName) *SimpleType {
	return &SimpleType{
		QName:           name,
		SourceNamespace: name.Namespace,
	}
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
		if facetApplicabilityReady(baseType) {
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
	if simpleType.Restriction.SimpleType != nil {
		if isNilType(simpleType.Restriction.SimpleType) {
			return nil
		}
		return simpleType.Restriction.SimpleType
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

func facetApplicabilityReady(baseType Type) bool {
	if isNilType(baseType) {
		return false
	}
	if baseType.IsBuiltin() {
		return true
	}
	if st, ok := as[*SimpleType](baseType); ok {
		if st.List != nil || st.Union != nil {
			return true
		}
	}
	return baseType.PrimitiveType() != nil
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
	if s == nil {
		return nil
	}
	// return cached value if available
	typeCacheMu.RLock()
	cached := s.fundamentalFacetsCache
	typeCacheMu.RUnlock()
	if cached != nil {
		return cached
	}

	computed := s.computeFundamentalFacets()
	if computed == nil {
		return nil
	}
	typeCacheMu.Lock()
	if s.fundamentalFacetsCache == nil {
		s.fundamentalFacetsCache = computed
	}
	cached = s.fundamentalFacetsCache
	typeCacheMu.Unlock()
	return cached
}

func (s *SimpleType) computeFundamentalFacets() *FundamentalFacets {
	primitive := s.PrimitiveType()
	if primitive == nil {
		return nil
	}

	// for built-in types accessed as Type interface
	if builtinType, ok := AsBuiltinType(primitive); ok {
		return builtinType.FundamentalFacets()
	}

	// for SimpleType that is built-in
	if simpleType, ok := AsSimpleType(primitive); ok {
		if simpleType.IsBuiltin() {
			return ComputeFundamentalFacets(TypeName(simpleType.QName.Local))
		}
	}

	return nil
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
					return countXMLFields(value)
				}
				if builtinType, ok := AsBuiltinType(s.ResolvedBase); ok && isBuiltinListType(builtinType.Name().Local) {
					// restriction of built-in list type: length is number of items
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
		if IsXMLWhitespaceByte(value[i]) {
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
	if s == nil {
		return AtomicVariety
	}
	if s.List != nil {
		return ListVariety
	}
	if s.Union != nil {
		return UnionVariety
	}
	if s.Restriction != nil {
		if s.ResolvedBase != nil {
			switch base := s.ResolvedBase.(type) {
			case *SimpleType:
				return base.Variety()
			case *BuiltinType:
				if isBuiltinListType(base.Name().Local) {
					return ListVariety
				}
			}
		}
		if !s.Restriction.Base.IsZero() &&
			s.Restriction.Base.Namespace == XSDNamespace &&
			isBuiltinListType(s.Restriction.Base.Local) {
			return ListVariety
		}
	}
	if s.builtin && isBuiltinListType(s.QName.Local) {
		return ListVariety
	}
	return AtomicVariety
}

// Validate checks if a lexical value is valid for this type
func (s *SimpleType) Validate(lexical string) error {
	normalized, err := NormalizeValue(lexical, s)
	if err != nil {
		return err
	}
	return s.validateNormalized(normalized, make(map[*SimpleType]bool))
}

func (s *SimpleType) validateNormalized(normalized string, visited map[*SimpleType]bool) error {
	if s == nil {
		return nil
	}
	if visited[s] {
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	// for built-in types, use built-in validator
	if s.IsBuiltin() {
		if builtinType := GetBuiltinNS(s.QName.Namespace, s.QName.Local); builtinType != nil {
			return builtinType.Validate(normalized)
		}
	}

	// for user-defined atomic types with restrictions, validate against primitive base
	if s.Restriction != nil && s.Variety() == AtomicVariety {
		primitive := s.PrimitiveType()
		if builtinType, ok := AsBuiltinType(primitive); ok {
			return builtinType.Validate(normalized)
		}
		if primitiveST, ok := AsSimpleType(primitive); ok && primitiveST.IsBuiltin() {
			if builtinType := GetBuiltinNS(primitiveST.QName.Namespace, primitiveST.QName.Local); builtinType != nil {
				return builtinType.Validate(normalized)
			}
		}
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

// PrimitiveType returns the ultimate primitive base type for this simple type
func (s *SimpleType) PrimitiveType() Type {
	// return cached value if available
	if s == nil {
		return nil
	}
	typeCacheMu.RLock()
	cached := s.primitiveType
	typeCacheMu.RUnlock()
	if cached != nil {
		return cached
	}

	computed := s.computePrimitiveType(make(map[*SimpleType]bool))
	if computed == nil {
		return nil
	}
	typeCacheMu.Lock()
	if s.primitiveType == nil {
		s.primitiveType = computed
	}
	cached = s.primitiveType
	typeCacheMu.Unlock()
	return cached
}

func (s *SimpleType) precomputeCaches() {
	if s == nil {
		return
	}
	_ = s.PrimitiveType()
	_ = s.FundamentalFacets()
	s.precomputeIdentityNormalization()
}

// IsQNameOrNotationType reports whether this type derives from QName or NOTATION.
func (s *SimpleType) IsQNameOrNotationType() bool {
	if s == nil {
		return false
	}
	typeCacheMu.RLock()
	ready := s.qnameOrNotationReady
	value := s.qnameOrNotation
	typeCacheMu.RUnlock()
	if ready {
		return value
	}
	computed := s.computeQNameOrNotationType()
	typeCacheMu.Lock()
	if !s.qnameOrNotationReady {
		s.qnameOrNotation = computed
		s.qnameOrNotationReady = true
	}
	value = s.qnameOrNotation
	typeCacheMu.Unlock()
	return value
}

// SetQNameOrNotationType stores the precomputed QName/NOTATION derivation flag.
func (s *SimpleType) SetQNameOrNotationType(value bool) {
	if s == nil {
		return
	}
	typeCacheMu.Lock()
	s.qnameOrNotation = value
	s.qnameOrNotationReady = true
	typeCacheMu.Unlock()
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
	if primitive := s.PrimitiveType(); primitive != nil && IsQNameOrNotation(primitive.Name()) {
		return true
	}
	return false
}

// computePrimitiveType is the internal implementation with cycle detection.
func (s *SimpleType) computePrimitiveType(visited map[*SimpleType]bool) Type {
	// if already computed, return it
	typeCacheMu.RLock()
	cached := s.primitiveType
	typeCacheMu.RUnlock()
	if cached != nil {
		return cached
	}

	if visited[s] {
		// circular reference detected - return nil to break the cycle
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	if primitive := s.primitiveFromSelf(); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromRestriction(visited); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromList(visited); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromUnion(visited); primitive != nil {
		return primitive
	}

	return nil
}

func (s *SimpleType) primitiveFromSelf() Type {
	if s.builtin && s.QName.Namespace == XSDNamespace && s.Variety() == AtomicVariety {
		if builtin := GetBuiltin(TypeName(s.QName.Local)); builtin != nil {
			return builtin.PrimitiveType()
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
	if s.List == nil {
		return nil
	}
	if s.ItemType != nil {
		return primitiveFromBaseType(s.ItemType, visited)
	}
	if !s.List.ItemType.IsZero() {
		if builtin := GetBuiltinNS(s.List.ItemType.Namespace, s.List.ItemType.Local); builtin != nil {
			return builtin.PrimitiveType()
		}
	}
	return nil
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
		return typed.computePrimitiveType(visited)
	case *BuiltinType:
		return typed.PrimitiveType()
	default:
		return nil
	}
}
