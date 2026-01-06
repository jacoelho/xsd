package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SimpleType represents a simple type definition
type SimpleType struct {
	QName                  QName
	variety                SimpleTypeVariety
	Restriction            *Restriction
	List                   *ListType
	Union                  *UnionType
	ResolvedBase           Type               // Resolved base type (can be set in struct literal or assigned directly)
	primitiveType          Type               // Ultimate primitive base type (cached)
	fundamentalFacetsCache *FundamentalFacets // Cached fundamental facets
	ItemType               Type               // Resolved item type for list types
	MemberTypes            []Type             // Resolved member types for union types
	whiteSpace             WhiteSpace         // WhiteSpace normalization (cached)
	whiteSpaceExplicit     bool               // True if whiteSpace was explicitly set in restriction
	builtin                bool
	SourceNamespace        NamespaceURI  // targetNamespace of the schema where this type was originally declared
	Final                  DerivationSet // Derivation methods blocked for this type (restriction, list, union)
}

// NewSimpleType creates a new simple type with the provided name and namespace.
func NewSimpleType(name QName, sourceNamespace NamespaceURI) *SimpleType {
	return &SimpleType{
		QName:           name,
		SourceNamespace: sourceNamespace,
	}
}

// NewSimpleTypeFromParsed validates a parsed simple type and returns it if valid.
func NewSimpleTypeFromParsed(st *SimpleType) (*SimpleType, error) {
	if st == nil {
		return nil, fmt.Errorf("simpleType is nil")
	}
	if err := validateSimpleTypeDefinition(st); err != nil {
		return nil, err
	}
	return st, nil
}

func validateSimpleTypeDefinition(st *SimpleType) error {
	switch st.Variety() {
	case AtomicVariety:
		if !st.IsBuiltin() && st.Restriction == nil {
			return fmt.Errorf("atomic simpleType must have a restriction")
		}
	case ListVariety:
		if st.List == nil && st.Restriction == nil {
			return fmt.Errorf("list simpleType must have a list definition or restriction")
		}
	case UnionVariety:
		if st.Union == nil && st.Restriction == nil {
			return fmt.Errorf("union simpleType must have a union definition or restriction")
		}
	default:
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			return fmt.Errorf("simpleType must have a derivation")
		}
	}

	if st.Restriction != nil {
		if st.Restriction.Base.IsZero() && st.ResolvedBase == nil {
			return fmt.Errorf("simpleType restriction must have a base type")
		}
		baseType := restrictionBaseType(st)
		if baseType != nil {
			if err := validateRestrictionFacetApplicability(st.Restriction.Facets, baseType); err != nil {
				return err
			}
		}
	}

	if st.List != nil {
		if st.List.InlineItemType != nil && !st.List.ItemType.IsZero() {
			return fmt.Errorf("list simpleType cannot have both inline and named item types")
		}
		if st.List.InlineItemType == nil && st.List.ItemType.IsZero() && st.ItemType == nil {
			return fmt.Errorf("list simpleType must declare an item type")
		}
	}

	if st.Union != nil {
		if len(st.Union.InlineTypes) == 0 && len(st.Union.MemberTypes) == 0 && len(st.MemberTypes) == 0 {
			return fmt.Errorf("union simpleType must declare member types")
		}
	}

	return nil
}

func restrictionBaseType(st *SimpleType) Type {
	if st == nil || st.Restriction == nil {
		return nil
	}
	if st.ResolvedBase != nil {
		if isNilType(st.ResolvedBase) {
			return nil
		}
		return st.ResolvedBase
	}
	if st.Restriction.Base.IsZero() {
		return nil
	}
	base := GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local)
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
	baseFacets := baseType.FundamentalFacets()
	if baseFacets == nil {
		if primitive := baseType.PrimitiveType(); primitive != nil {
			baseFacets = primitive.FundamentalFacets()
		}
	}
	if baseFacets == nil {
		return nil
	}
	for _, facet := range facets {
		namer, ok := facet.(facetNamer)
		if !ok {
			continue
		}
		facetName := namer.Name()
		if !IsFacetApplicable(facetName, baseFacets) {
			return fmt.Errorf("facet %q is not applicable to base type %s", facetName, baseType.Name())
		}
	}
	return nil
}

func isNilType(typ Type) bool {
	if typ == nil {
		return true
	}
	switch value := typ.(type) {
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
	// Remap union memberTypes and inline types if present
	if s.Union != nil {
		unionCopy := *s.Union
		if len(unionCopy.MemberTypes) > 0 {
			memberTypes := make([]QName, len(unionCopy.MemberTypes))
			for i, mt := range unionCopy.MemberTypes {
				// Only remap if not in XSD namespace (built-in types)
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
	// Remap list itemType if present
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
	// Return cached value if available
	if s.fundamentalFacetsCache != nil {
		return s.fundamentalFacetsCache
	}

	primitive := s.PrimitiveType()
	if primitive == nil {
		return nil
	}

	// For built-in types accessed as Type interface
	if bt, ok := as[*BuiltinType](primitive); ok {
		facets := bt.FundamentalFacets()
		s.fundamentalFacetsCache = facets
		return facets
	}

	// For SimpleType that is built-in
	if st, ok := as[*SimpleType](primitive); ok {
		if st.IsBuiltin() {
			facets := ComputeFundamentalFacets(TypeName(st.QName.Local))
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
	// Check if this type is itself a list type
	if s.List != nil {
		// List type: length is number of items (space-separated)
		if len(strings.TrimSpace(value)) == 0 {
			return 0
		}
		return len(strings.Fields(value))
	}

	// Check if this type restricts a list type
	if s.Restriction != nil {
		// First check ResolvedBase if available
		if s.ResolvedBase != nil {
			if lm, ok := as[LengthMeasurable](s.ResolvedBase); ok {
				// Check if base is a list type
				if baseST, ok := as[*SimpleType](s.ResolvedBase); ok && baseST.List != nil {
					// Restriction of list type: length is number of items
					if len(strings.TrimSpace(value)) == 0 {
						return 0
					}
					return len(strings.Fields(value))
				}
				if bt, ok := as[*BuiltinType](s.ResolvedBase); ok && isBuiltinListType(bt.Name().Local) {
					// Restriction of built-in list type: length is number of items
					if len(strings.TrimSpace(value)) == 0 {
						return 0
					}
					return len(strings.Fields(value))
				}
				// Otherwise delegate to base type
				return lm.MeasureLength(value)
			}
		}
		// Fallback: check if Restriction.Base QName suggests it's a list type
		if !s.Restriction.Base.IsZero() {
			baseLocal := s.Restriction.Base.Local
			if strings.HasPrefix(strings.ToLower(baseLocal), "list") ||
				isBuiltinListType(baseLocal) {
				// Likely a list type - count items
				if len(strings.TrimSpace(value)) == 0 {
					return 0
				}
				return len(strings.Fields(value))
			}
		}
	}

	// For user-defined types, delegate to primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType != nil {
		if lm, ok := as[LengthMeasurable](primitiveType); ok {
			return lm.MeasureLength(value)
		}
		// Fallback: use primitive name directly
		return measureLengthForPrimitive(value, TypeName(primitiveType.Name().Local))
	}

	// Fallback: character count
	return utf8.RuneCountInString(value)
}

// Variety returns the simple type variety (implements SimpleTypeDefinition)
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

	// For built-in types, use built-in validator
	if s.IsBuiltin() {
		if bt := GetBuiltinNS(s.QName.Namespace, s.QName.Local); bt != nil {
			return bt.Validate(normalized)
		}
	}

	// For user-defined types with restrictions, validate against base type and facets
	if s.Restriction != nil {
		baseType := GetBuiltinNS(s.Restriction.Base.Namespace, s.Restriction.Base.Local)
		if baseType != nil {
			if err := baseType.Validate(normalized); err != nil {
				return err
			}
		}
		// Facet validation is done separately in the validator
	}

	return nil
}

// ParseValue converts a lexical value to a TypedValue
func (s *SimpleType) ParseValue(lexical string) (TypedValue, error) {
	normalized, err := NormalizeValue(lexical, s)
	if err != nil {
		return nil, err
	}

	// First, try to parse based on the type's own name (for built-in types)
	if s.IsBuiltin() {
		typeName := TypeName(s.QName.Local)
		if result, err := ParseValueForType(normalized, typeName, s); err == nil {
			return result, nil
		}
	}

	// For user-defined types or if built-in type not handled above, use primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType == nil {
		return nil, fmt.Errorf("cannot determine primitive type")
	}

	primitiveST, ok := as[*SimpleType](primitiveType)
	if !ok {
		// Try BuiltinType
		if bt, ok := as[*BuiltinType](primitiveType); ok {
			return bt.ParseValue(normalized)
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
	// Return cached value if available
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

// getPrimitiveTypeWithVisited is the internal implementation with cycle detection
func (s *SimpleType) getPrimitiveTypeWithVisited(visited map[*SimpleType]bool) Type {
	// If already computed, return it
	if s.primitiveType != nil {
		return s.primitiveType
	}

	if visited[s] {
		// Circular reference detected - return nil to break the cycle
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	// Primitive types: self
	if s.builtin && s.Variety() == AtomicVariety {
		if isPrimitiveName(TypeName(s.QName.Local)) && s.QName.Namespace == XSDNamespace {
			s.primitiveType = s
			return s.primitiveType
		}
	}

	// Derived by restriction: follow base chain
	if s.Restriction != nil {
		// First try ResolvedBase if available (after type resolution)
		if s.ResolvedBase != nil {
			if baseSimple, ok := as[*SimpleType](s.ResolvedBase); ok {
				primitive := baseSimple.getPrimitiveTypeWithVisited(visited)
				if primitive != nil {
					s.primitiveType = primitive
					return primitive
				}
			} else if baseBuiltin, ok := as[*BuiltinType](s.ResolvedBase); ok {
				// Base is a built-in type, get its primitive type
				primitive := baseBuiltin.PrimitiveType()
				if primitive != nil {
					s.primitiveType = primitive
					return primitive
				}
			}
		} else if !s.Restriction.Base.IsZero() {
			// ResolvedBase not set yet (during parsing), try to resolve from Restriction.Base
			// For built-in types, we can resolve directly
			if s.Restriction.Base.Namespace == XSDNamespace {
				if bt := GetBuiltin(TypeName(s.Restriction.Base.Local)); bt != nil {
					primitive := bt.PrimitiveType()
					if primitive != nil {
						s.primitiveType = primitive
						return primitive
					}
				}
			}
			// For user-defined types, we can't resolve without schema context
			// This will be resolved during schema validation phase
		}
	}

	// List types: item type's primitive
	if s.List != nil && s.ItemType != nil {
		if itemSimple, ok := as[*SimpleType](s.ItemType); ok {
			primitive := itemSimple.getPrimitiveTypeWithVisited(visited)
			if primitive != nil {
				s.primitiveType = primitive
				return primitive
			}
		} else if itemBuiltin, ok := as[*BuiltinType](s.ItemType); ok {
			// Item type is a built-in type, get its primitive type
			primitive := itemBuiltin.PrimitiveType()
			if primitive != nil {
				s.primitiveType = primitive
				return primitive
			}
		}
	}

	// Union types: common primitive or anySimpleType
	// For now, if we can't determine, return nil
	// This will be resolved during schema compilation
	if s.Union != nil {
		// Try to find common primitive
		var commonPrimitive Type
		for _, memberType := range s.MemberTypes {
			var memberPrimitive Type
			if memberSimple, ok := as[*SimpleType](memberType); ok {
				memberPrimitive = memberSimple.getPrimitiveTypeWithVisited(visited)
			} else if memberBuiltin, ok := as[*BuiltinType](memberType); ok {
				// Member type is a built-in type, get its primitive type
				memberPrimitive = memberBuiltin.PrimitiveType()
			}
			if memberPrimitive == nil {
				continue
			}
			if commonPrimitive == nil {
				commonPrimitive = memberPrimitive
			} else if commonPrimitive != memberPrimitive {
				// Different primitives, return anySimpleType or nil
				// For now, return nil (will be resolved later)
				return nil
			}
		}
		if commonPrimitive != nil {
			s.primitiveType = commonPrimitive
			return commonPrimitive
		}
	}

	return nil
}
