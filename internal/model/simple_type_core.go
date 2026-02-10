package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/value"
)

// SimpleType represents a simple type definition
type SimpleType struct {
	identityListItemType           Type
	ResolvedBase                   Type
	primitiveType                  Type
	ItemType                       Type
	Restriction                    *Restriction
	List                           *ListType
	Union                          *UnionType
	fundamentalFacetsCache         *FundamentalFacets
	QName                          QName
	SourceNamespace                NamespaceURI
	identityMemberTypes            []Type
	MemberTypes                    []Type
	cacheGuard                     cacheGuard
	Final                          DerivationSet
	whiteSpace                     WhiteSpace
	fundamentalFacetsComputing     bool
	primitiveTypeComputing         bool
	qnameOrNotationReady           bool
	qnameOrNotation                bool
	identityNormalizationReady     bool
	identityNormalizable           bool
	identityNormalizationComputing bool
	whiteSpaceExplicit             bool
	builtin                        bool
}

// NewAtomicSimpleType creates a simple type derived by restriction.
func NewAtomicSimpleType(name QName, sourceNamespace NamespaceURI, restriction *Restriction) (*SimpleType, error) {
	st := &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
		Restriction:     restriction,
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
		if simpleType.Restriction.Base.IsZero() && simpleType.ResolvedBase == nil && simpleType.Restriction.SimpleType == nil {
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
		if err := validateFacetApplicability(facetName, baseType, baseType.Name()); err != nil {
			return err
		}
	}
	return nil
}

func isNilType(typ Type) bool {
	// interface-typed nils need a type switch to detect nil pointers.
	switch typed := typ.(type) {
	case nil:
		return true
	case *BuiltinType:
		return typed == nil
	case *SimpleType:
		return typed == nil
	case *ComplexType:
		return typed == nil
	default:
		return false
	}
}

// Name returns the QName of the simple type.
func (s *SimpleType) Name() QName {
	if s == nil {
		return QName{}
	}
	return s.QName
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (s *SimpleType) ComponentName() QName {
	if s == nil {
		return QName{}
	}
	return s.QName
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (s *SimpleType) DeclaredNamespace() NamespaceURI {
	if s == nil {
		return ""
	}
	return s.SourceNamespace
}

// Copy creates a copy of the simple type with remapped QNames.
func (s *SimpleType) Copy(opts CopyOptions) *SimpleType {
	if s == nil {
		return nil
	}
	if existing, ok := opts.lookupSimpleType(s); ok {
		return existing
	}
	clone := simpleTypeCopyWithoutGuard(s)
	opts.rememberSimpleType(s, &clone)
	clone.QName = opts.RemapQName(s.QName)
	clone.SourceNamespace = sourceNamespace(s.SourceNamespace, opts)
	clone.ResolvedBase = CopyType(s.ResolvedBase, opts)
	clone.primitiveType = CopyType(s.primitiveType, opts)
	clone.ItemType = CopyType(s.ItemType, opts)
	clone.identityListItemType = CopyType(s.identityListItemType, opts)
	if len(s.MemberTypes) > 0 {
		memberTypes := make([]Type, len(s.MemberTypes))
		for i, member := range s.MemberTypes {
			memberTypes[i] = CopyType(member, opts)
		}
		clone.MemberTypes = memberTypes
	}
	if len(s.identityMemberTypes) > 0 {
		identityMemberTypes := make([]Type, len(s.identityMemberTypes))
		for i, member := range s.identityMemberTypes {
			identityMemberTypes[i] = CopyType(member, opts)
		}
		clone.identityMemberTypes = identityMemberTypes
	}
	copyInline := func(inline *SimpleType) *SimpleType {
		if inline == nil {
			return nil
		}
		inlineCopy := inline.Copy(opts)
		if inline.QName.IsZero() {
			inlineCopy.QName = inline.QName
		}
		inlineCopy.SourceNamespace = sourceNamespace(inline.SourceNamespace, opts)
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
				if mt.Namespace == "" {
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
		if !listCopy.ItemType.IsZero() && listCopy.ItemType.Namespace == "" {
			listCopy.ItemType = opts.RemapQName(listCopy.ItemType)
		}
		listCopy.InlineItemType = copyInline(listCopy.InlineItemType)
		clone.List = &listCopy
	}
	return &clone
}

func simpleTypeCopyWithoutGuard(src *SimpleType) SimpleType {
	if src == nil {
		return SimpleType{}
	}
	return SimpleType{
		identityListItemType:           src.identityListItemType,
		ResolvedBase:                   src.ResolvedBase,
		primitiveType:                  src.primitiveType,
		ItemType:                       src.ItemType,
		Restriction:                    src.Restriction,
		List:                           src.List,
		Union:                          src.Union,
		fundamentalFacetsCache:         src.fundamentalFacetsCache,
		QName:                          src.QName,
		SourceNamespace:                src.SourceNamespace,
		identityMemberTypes:            src.identityMemberTypes,
		MemberTypes:                    src.MemberTypes,
		Final:                          src.Final,
		whiteSpace:                     src.whiteSpace,
		fundamentalFacetsComputing:     src.fundamentalFacetsComputing,
		primitiveTypeComputing:         src.primitiveTypeComputing,
		qnameOrNotationReady:           src.qnameOrNotationReady,
		qnameOrNotation:                src.qnameOrNotation,
		identityNormalizationReady:     src.identityNormalizationReady,
		identityNormalizable:           src.identityNormalizable,
		identityNormalizationComputing: src.identityNormalizationComputing,
		whiteSpaceExplicit:             src.whiteSpaceExplicit,
		builtin:                        src.builtin,
	}
}

// IsBuiltin reports whether the simple type is built-in.
func (s *SimpleType) IsBuiltin() bool {
	if s == nil {
		return false
	}
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
	if s == nil {
		return nil
	}
	if s.ResolvedBase == nil {
		return GetBuiltin(TypeNameAnySimpleType)
	}
	return s.ResolvedBase
}

// ResolvedBaseType returns the resolved base type, or nil if at root.
// Implements DerivedType interface.
func (s *SimpleType) ResolvedBaseType() Type {
	if s == nil {
		return nil
	}
	return s.ResolvedBase
}

// FundamentalFacets returns the fundamental facets for this simple type
func (s *SimpleType) FundamentalFacets() *FundamentalFacets {
	if s == nil {
		return nil
	}
	guard := s.guard()
	guard.mu.Lock()
	for s.fundamentalFacetsCache == nil && s.fundamentalFacetsComputing {
		guard.cond.Wait()
	}
	if s.fundamentalFacetsCache != nil {
		cached := s.fundamentalFacetsCache
		guard.mu.Unlock()
		return cached
	}
	s.fundamentalFacetsComputing = true
	guard.mu.Unlock()

	computed := s.computeFundamentalFacets()
	if computed == nil {
		guard.mu.Lock()
		s.fundamentalFacetsComputing = false
		guard.cond.Broadcast()
		guard.mu.Unlock()
		return nil
	}

	guard.mu.Lock()
	if s.fundamentalFacetsCache == nil {
		s.fundamentalFacetsCache = computed
	}
	s.fundamentalFacetsComputing = false
	cached := s.fundamentalFacetsCache
	guard.cond.Broadcast()
	guard.mu.Unlock()
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
	if s == nil {
		return WhiteSpacePreserve
	}
	return s.whiteSpace
}

// SetWhiteSpace sets the whitespace normalization for this simple type
func (s *SimpleType) SetWhiteSpace(ws WhiteSpace) {
	if s == nil {
		return
	}
	s.whiteSpace = ws
}

// SetWhiteSpaceExplicit sets the whitespace normalization and marks it as explicitly set.
// This is used when parsing a whiteSpace facet in a restriction.
func (s *SimpleType) SetWhiteSpaceExplicit(ws WhiteSpace) {
	if s == nil {
		return
	}
	s.whiteSpace = ws
	s.whiteSpaceExplicit = true
}

// WhiteSpaceExplicit returns true if whiteSpace was explicitly set in this type's restriction.
func (s *SimpleType) WhiteSpaceExplicit() bool {
	if s == nil {
		return false
	}
	return s.whiteSpaceExplicit
}

// MeasureLength returns length in type-appropriate units (octets, items, or characters).
// Implements LengthMeasurable interface.
func (s *SimpleType) MeasureLength(lexical string) int {
	if s == nil {
		return 0
	}
	// check if this type is itself a list type
	if s.List != nil {
		// list type: length is number of items (space-separated)
		return countXMLFields(lexical)
	}

	// check if this type restricts a list type
	if s.Restriction != nil {
		// first check ResolvedBase if available
		if s.ResolvedBase != nil {
			if lengthMeasurer, ok := as[LengthMeasurable](s.ResolvedBase); ok {
				// check if base is a list type
				if baseSimpleType, ok := AsSimpleType(s.ResolvedBase); ok && baseSimpleType.List != nil {
					// restriction of list type: length is number of items
					return countXMLFields(lexical)
				}
				if builtinType, ok := AsBuiltinType(s.ResolvedBase); ok && isBuiltinListTypeName(builtinType.Name().Local) {
					// restriction of built-in list type: length is number of items
					return countXMLFields(lexical)
				}
				// otherwise delegate to base type
				return lengthMeasurer.MeasureLength(lexical)
			}
		}
		// fallback: check if Restriction.Base QName suggests it's a list type
		if !s.Restriction.Base.IsZero() {
			baseLocal := s.Restriction.Base.Local
			if strings.HasPrefix(strings.ToLower(baseLocal), "list") ||
				isBuiltinListTypeName(baseLocal) {
				// likely a list type - count items
				return countXMLFields(lexical)
			}
		}
	}

	// for user-defined types, delegate to primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType != nil {
		if lengthMeasurer, ok := as[LengthMeasurable](primitiveType); ok {
			return lengthMeasurer.MeasureLength(lexical)
		}
		// fallback: use primitive name directly
		return measureLengthForPrimitive(lexical, TypeName(primitiveType.Name().Local))
	}
	// fallback: character count
	return utf8.RuneCountInString(lexical)
}

func countXMLFields(lexical string) int {
	// count fields delimited by XML whitespace (#x20/#x9/#xD/#xA) without allocations.
	count := 0
	inField := false
	for i := 0; i < len(lexical); i++ {
		if value.IsXMLWhitespaceByte(lexical[i]) {
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
				if isBuiltinListTypeName(base.Name().Local) {
					return ListVariety
				}
			}
		}
		if !s.Restriction.Base.IsZero() &&
			s.Restriction.Base.Namespace == XSDNamespace &&
			isBuiltinListTypeName(s.Restriction.Base.Local) {
			return ListVariety
		}
	}
	if s.builtin && isBuiltinListTypeName(s.QName.Local) {
		return ListVariety
	}
	return AtomicVariety
}

// Validate checks if a lexical value is valid for this type
