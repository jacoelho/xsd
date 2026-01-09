package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// resolveTypeReferences implements two-phase type resolution (Phase 2)
// Resolves all QName references to Type objects after all types are parsed
func resolveTypeReferences(schema *parser.Schema) error {
	detector := NewCycleDetector[types.QName]()
	visitedPtrs := make(map[types.Type]bool) // track anonymous types by pointer

	for qname, typeDef := range schema.TypeDefs {
		if detector.IsVisited(qname) {
			continue
		}
		if err := resolveType(qname, typeDef, schema, detector); err != nil {
			return err
		}
	}

	for _, elem := range schema.ElementDecls {
		if err := resolveElementAnonymousTypes(elem, schema, detector, visitedPtrs); err != nil {
			return err
		}
	}

	// re-link attribute type references to use the current TypeDefs
	// this is needed because when schemas are merged, attributes may point to
	// stale type pointers that were later replaced by redefine operations
	for _, attr := range schema.AttributeDecls {
		if attr.Type == nil {
			continue
		}
		// get the type's QName
		typeQName := attr.Type.Name()
		if typeQName.IsZero() {
			continue // anonymous type, no need to re-link
		}
		// look up the current type definition
		if currentType, ok := schema.TypeDefs[typeQName]; ok {
			if currentType != attr.Type {
				attr.Type = currentType
			}
		}
	}

	if err := resolveAttributeTypes(schema); err != nil {
		return err
	}

	return nil
}

// ResolveTypeReferences resolves all type references after schema parsing.
func ResolveTypeReferences(schema *parser.Schema) error {
	return resolveTypeReferences(schema)
}

func resolveAttributeTypes(schema *parser.Schema) error {
	resolveAttr := func(attr *types.AttributeDecl) error {
		if attr == nil || attr.Type == nil {
			return nil
		}
		if st, ok := attr.Type.(*types.SimpleType); ok {
			// case 1: Placeholder type (QName reference) - replace with resolved type
			if st.Restriction == nil && st.List == nil && st.Union == nil && !st.QName.IsZero() {
				resolved, err := lookupType(schema, st.QName)
				if err != nil {
					return err
				}
				attr.Type = resolved
			}
			// case 2: Inline simpleType with restriction - resolve its base type
			if st.Restriction != nil && !st.Restriction.Base.IsZero() && st.ResolvedBase == nil {
				baseType, err := lookupType(schema, st.Restriction.Base)
				if err != nil {
					return fmt.Errorf("attribute %s inline simpleType base: %w", attr.Name, err)
				}
				st.ResolvedBase = baseType
			}
			// case 3: Inline simpleType with list - resolve its item type
			if st.List != nil && !st.List.ItemType.IsZero() && st.ItemType == nil {
				itemType, err := lookupType(schema, st.List.ItemType)
				if err != nil {
					if allowMissingTypeReference(schema, st.List.ItemType) {
						st.ItemType = &types.SimpleType{QName: st.List.ItemType}
						return nil
					}
					return fmt.Errorf("attribute %s inline list itemType: %w", attr.Name, err)
				}
				st.ItemType = itemType
			}
		}
		return nil
	}

	resolveCT := func(ct *types.ComplexType) error {
		for _, attr := range ct.Attributes() {
			if err := resolveAttr(attr); err != nil {
				return err
			}
		}
		content := ct.Content()
		if ext := content.ExtensionDef(); ext != nil {
			for _, attr := range ext.Attributes {
				if err := resolveAttr(attr); err != nil {
					return err
				}
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			for _, attr := range restr.Attributes {
				if err := resolveAttr(attr); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, ag := range schema.AttributeGroups {
		for _, attr := range ag.Attributes {
			if err := resolveAttr(attr); err != nil {
				return err
			}
		}
	}

	for _, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			if err := resolveCT(ct); err != nil {
				return err
			}
		}
	}

	for _, elem := range schema.ElementDecls {
		if elem.Type != nil {
			if ct, ok := elem.Type.(*types.ComplexType); ok {
				if err := resolveCT(ct); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func resolveElementAnonymousTypes(elem *types.ElementDecl, schema *parser.Schema, detector *CycleDetector[types.QName], visitedPtrs map[types.Type]bool) error {
	if elem.Type == nil {
		return nil
	}

	// skip if already visited (pointer-based cycle detection for anonymous types)
	if visitedPtrs[elem.Type] {
		return nil
	}
	visitedPtrs[elem.Type] = true

	// if it's a simple type (inline or reference), resolve it
	if st, ok := elem.Type.(*types.SimpleType); ok {
		// placeholder simpleType with only a QName - resolve to the actual type.
		if st.Restriction == nil && st.List == nil && st.Union == nil && !st.QName.IsZero() {
			if detector.IsResolving(st.QName) {
				return nil
			}
			resolved, err := lookupType(schema, st.QName)
			if err != nil {
				if allowMissingTypeReference(schema, st.QName) {
					return nil
				}
				return err
			}
			elem.Type = resolved
			return nil
		}
		// anonymous SimpleType (inline definition) - resolve restrictions, lists, unions
		if st.QName.IsZero() {
			if err := resolveSimpleType(st, schema, detector); err != nil {
				return fmt.Errorf("element '%s' inline simpleType: %w", elem.Name, err)
			}
		}
	}

	// if it's a complex type (inline or reference), resolve it
	if ct, ok := elem.Type.(*types.ComplexType); ok {
		// check if it's a named type (should be in TypeDefs)
		if !ct.QName.IsZero() {
			if detector.IsResolving(ct.QName) {
				return nil
			}
			// named type - resolve using QName-based resolution
			if err := resolveType(ct.QName, ct, schema, detector); err != nil {
				return err
			}
		} else {
			// anonymous type - resolve directly
			if err := resolveComplexType(ct, schema, detector); err != nil {
				return err
			}
			// also resolve anonymous types in the content model
			if err := resolveContentModelAnonymousTypes(ct.Content(), schema, detector, visitedPtrs); err != nil {
				return err
			}
		}
	}

	return nil
}

func resolveContentModelAnonymousTypes(content types.Content, schema *parser.Schema, detector *CycleDetector[types.QName], visitedPtrs map[types.Type]bool) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			if err := resolveParticleAnonymousTypes(c.Particle, schema, detector, visitedPtrs); err != nil {
				return err
			}
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if err := resolveParticleAnonymousTypes(c.Extension.Particle, schema, detector, visitedPtrs); err != nil {
				return err
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			if err := resolveParticleAnonymousTypes(c.Restriction.Particle, schema, detector, visitedPtrs); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveParticleAnonymousTypes(particle types.Particle, schema *parser.Schema, detector *CycleDetector[types.QName], visitedPtrs map[types.Type]bool) error {
	switch p := particle.(type) {
	case *types.ElementDecl:
		return resolveElementAnonymousTypes(p, schema, detector, visitedPtrs)
	case *types.ModelGroup:
		for _, child := range p.Particles {
			if err := resolveParticleAnonymousTypes(child, schema, detector, visitedPtrs); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveType resolves a single type and all its references
func resolveType(qname types.QName, typeDef types.Type, schema *parser.Schema, detector *CycleDetector[types.QName]) error {
	if detector.IsVisited(qname) {
		return nil // already resolved
	}
	if detector.IsResolving(qname) {
		if _, ok := typeDef.(*types.ComplexType); ok {
			// complex types can be recursive via element content; skip re-entry.
			return nil
		}
		return fmt.Errorf("circular reference detected: %v", qname)
	}

	// use CycleDetector's WithScope for automatic cycle detection and cleanup
	return detector.WithScope(qname, func() error {
		// (a redefined type can reference the old version of itself)
		// self-references are handled in resolveSimpleType/resolveComplexType
		// if we get here, it's a real circular dependency - CycleDetector will catch it

		switch t := typeDef.(type) {
		case *types.SimpleType:
			return resolveSimpleType(t, schema, detector)
		case *types.ComplexType:
			return resolveComplexType(t, schema, detector)
		default:
			return nil
		}
	})
}

// resolveSimpleType resolves all references in a SimpleType
func resolveSimpleType(st *types.SimpleType, schema *parser.Schema, detector *CycleDetector[types.QName]) error {
	if st.Restriction != nil {
		// if base is specified as a QName, resolve it
		if !st.Restriction.Base.IsZero() {
			if st.ResolvedBase == nil {
				// check for self-reference (circular dependency)
				if st.Restriction.Base == st.QName {
					return fmt.Errorf("type %s: circular derivation (type cannot be its own base)", st.QName)
				}
				baseType, err := lookupType(schema, st.Restriction.Base)
				if err != nil {
					return fmt.Errorf("type %s: %w", st.QName, err)
				}
				st.ResolvedBase = baseType
			}
			// inherit variety from base type when restricting a list or union type
			// per XSD spec, restricting a list type produces a list type, etc.
			if baseType := st.BaseType(); baseType != nil {
				if baseST, ok := baseType.(*types.SimpleType); ok {
					if baseST.Variety() == types.ListVariety || baseST.Variety() == types.UnionVariety {
						st.SetVariety(baseST.Variety())
					}
				}
			}
			// inherit whiteSpace from base type if not explicitly set in this restriction
			if !st.WhiteSpaceExplicit() {
				if baseType := st.BaseType(); baseType != nil {
					st.SetWhiteSpace(baseType.WhiteSpace())
				}
			}
		}
		// recursively resolve base type if it's not a built-in and not a self-reference
		// (This handles both QName-based bases and inline simpleType bases)
		// for self-references in redefine context (where base == st.QName), we already
		// set ResolvedBase to the original type above, so we need to resolve that
		if st.BaseType() != nil && st.BaseType() != st {
			if baseST, ok := st.BaseType().(*types.SimpleType); ok && !baseST.IsBuiltin() {
				if baseST.QName.IsZero() {
					// inline simpleType (no QName) - resolve directly
					if err := resolveSimpleType(baseST, schema, detector); err != nil {
						return err
					}
				} else if baseST.QName != st.QName {
					// named type (but NOT a self-reference) - use resolveType for cycle detection
					if err := resolveType(baseST.QName, baseST, schema, detector); err != nil {
						return err
					}
				} else {
					// self-reference in redefine context - resolve the original directly
					// without cycle detection (since it has the same QName but is a different type instance)
					if err := resolveSimpleType(baseST, schema, detector); err != nil {
						return err
					}
				}
			}
		}
	}

	if st.List != nil {
		// per XSD spec, list types always have whiteSpace=collapse
		st.SetWhiteSpace(types.WhiteSpaceCollapse)
		if st.List.ItemType.IsZero() {
			// inline simpleType - resolve it
			if st.List.InlineItemType != nil {
				if !st.List.InlineItemType.IsBuiltin() {
					// note: List types can have circular item type references
					// for inline types, check if we're already resolving this type
					if !st.List.InlineItemType.QName.IsZero() && !detector.IsResolving(st.List.InlineItemType.QName) {
						if err := resolveType(st.List.InlineItemType.QName, st.List.InlineItemType, schema, detector); err != nil {
							return err
						}
					}
				}
				// also store in st.ItemType for validator access
				st.ItemType = st.List.InlineItemType
			}
		} else {
			// itemType attribute - resolve from QName
			if st.ItemType == nil {
				itemType, err := lookupType(schema, st.List.ItemType)
				if err != nil {
					if allowMissingTypeReference(schema, st.List.ItemType) {
						st.ItemType = &types.SimpleType{QName: st.List.ItemType}
						return nil
					}
					return fmt.Errorf("type %s: list itemType: %w", st.QName, err)
				}
				st.ItemType = itemType
			}
			// recursively resolve item type if it's not a built-in
			// note: List types can have circular item type references (e.g., list of list)
			// if the item type is already being resolved, skip recursive resolution
			// to allow the circular reference
			if itemST, ok := st.ItemType.(*types.SimpleType); ok && !itemST.IsBuiltin() {
				if !detector.IsResolving(itemST.QName) {
					if err := resolveType(itemST.QName, itemST, schema, detector); err != nil {
						return err
					}
				}
			}
		}
	}

	if st.Union != nil {
		if len(st.Union.MemberTypes) > 0 {
			if st.MemberTypes == nil {
				st.MemberTypes = make([]types.Type, 0, len(st.Union.MemberTypes))
			}
			for i, memberQName := range st.Union.MemberTypes {
				if i < len(st.MemberTypes) && st.MemberTypes[i] != nil {
					continue // already resolved
				}
				memberType, err := lookupType(schema, memberQName)
				if err != nil {
					if allowMissingTypeReference(schema, memberQName) {
						memberType = &types.SimpleType{QName: memberQName}
					} else {
						return fmt.Errorf("type %s: union memberType %d: %w", st.QName, i, err)
					}
				}
				if i >= len(st.MemberTypes) {
					st.MemberTypes = append(st.MemberTypes, memberType)
				} else {
					st.MemberTypes[i] = memberType
				}
				// recursively resolve member type if it's not a built-in
				// note: Union types can have circular member references (this is valid in XSD)
				// if the member type is already being resolved, skip recursive resolution
				// to allow the circular reference
				if memberST, ok := memberType.(*types.SimpleType); ok && !memberST.IsBuiltin() {
					if !detector.IsResolving(memberST.QName) {
						if err := resolveType(memberST.QName, memberST, schema, detector); err != nil {
							return err
						}
					}
				}
			}
		}

		// resolve inline types (anonymous simpleTypes)
		for _, inlineType := range st.Union.InlineTypes {
			if inlineType.Restriction != nil && !inlineType.Restriction.Base.IsZero() {
				if inlineType.ResolvedBase == nil {
					baseType, err := lookupType(schema, inlineType.Restriction.Base)
					if err != nil {
						return fmt.Errorf("type %s: union inline type base: %w", st.QName, err)
					}
					inlineType.ResolvedBase = baseType
				}
				// set whiteSpace from base type if not explicitly set
				if inlineType.WhiteSpace() == types.WhiteSpacePreserve {
					// check if whiteSpace was explicitly set (default is preserve)
					// if base type has different whiteSpace, inherit it
					if baseType := inlineType.BaseType(); baseType != nil {
						inlineType.SetWhiteSpace(baseType.WhiteSpace())
					}
				}
				// recursively resolve base type if it's not a built-in and not a self-reference
				if inlineType.BaseType() != inlineType {
					if baseST, ok := inlineType.BaseType().(*types.SimpleType); ok && !baseST.IsBuiltin() {
						if err := resolveType(baseST.QName, baseST, schema, detector); err != nil {
							return err
						}
					}
				}
			}
			if inlineType.List != nil && !inlineType.List.ItemType.IsZero() {
				if inlineType.ItemType == nil {
					itemType, err := lookupType(schema, inlineType.List.ItemType)
					if err != nil {
						if allowMissingTypeReference(schema, inlineType.List.ItemType) {
							inlineType.ItemType = &types.SimpleType{QName: inlineType.List.ItemType}
							continue
						}
						return fmt.Errorf("type %s: union inline type list itemType: %w", st.QName, err)
					}
					inlineType.ItemType = itemType
				}
				// recursively resolve item type if it's not a built-in
				// note: List types can have circular item type references
				if itemST, ok := inlineType.ItemType.(*types.SimpleType); ok && !itemST.IsBuiltin() {
					// try to enter - if it's already resolving, that's a valid circular reference
					if err := detector.Enter(itemST.QName); err != nil {
						// circular reference - this is valid, skip recursive resolution
					} else {
						detector.Leave(itemST.QName)
						if err := resolveType(itemST.QName, itemST, schema, detector); err != nil {
							return err
						}
					}
				}
			}
			// recursively resolve nested union inline types
			if inlineType.Union != nil {
				// use the same detector since inline types are anonymous
				if err := resolveSimpleType(inlineType, schema, detector); err != nil {
					return err
				}
			}
			// add resolved inline type to MemberTypes so compiler can compile it
			if st.MemberTypes == nil {
				st.MemberTypes = make([]types.Type, 0, len(st.Union.InlineTypes))
			}
			st.MemberTypes = append(st.MemberTypes, inlineType)
		}
	}

	return nil
}

// resolveComplexType resolves all references in a ComplexType
func resolveComplexType(ct *types.ComplexType, schema *parser.Schema, detector *CycleDetector[types.QName]) error {
	// check for self-reference (circular dependency)
	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		baseQName := sc.BaseTypeQName()
		if !baseQName.IsZero() && baseQName == ct.QName {
			return fmt.Errorf("complex type %s: circular derivation (type cannot be its own base)", ct.QName)
		}
	}
	if cc, ok := ct.Content().(*types.ComplexContent); ok {
		baseQName := cc.BaseTypeQName()
		if !baseQName.IsZero() && baseQName == ct.QName {
			return fmt.Errorf("complex type %s: circular derivation (type cannot be its own base)", ct.QName)
		}
	}

	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		baseQName := sc.BaseTypeQName()
		if !baseQName.IsZero() {
			if ct.ResolvedBase == nil {
				baseType, err := lookupType(schema, baseQName)
				if err != nil {
					return fmt.Errorf("type %s: simpleContent base: %w", ct.QName, err)
				}
				ct.ResolvedBase = baseType
			}
			skipBaseResolve := ct.QName.IsZero() && detector.IsResolving(baseQName)
			// recursively resolve base type
			if baseCT, ok := ct.BaseType().(*types.ComplexType); ok {
				if !skipBaseResolve && !detector.IsResolving(baseCT.QName) {
					if err := resolveType(baseCT.QName, baseCT, schema, detector); err != nil {
						return err
					}
				}
			} else if baseST, ok := ct.BaseType().(*types.SimpleType); ok && !baseST.IsBuiltin() {
				if !skipBaseResolve {
					if err := resolveType(baseST.QName, baseST, schema, detector); err != nil {
						return err
					}
				}
			}
		}
	}

	if cc, ok := ct.Content().(*types.ComplexContent); ok {
		baseQName := cc.BaseTypeQName()
		if !baseQName.IsZero() {
			if ct.ResolvedBase == nil {
				baseType, err := lookupType(schema, baseQName)
				if err != nil {
					return fmt.Errorf("type %s: complexContent base: %w", ct.QName, err)
				}
				ct.ResolvedBase = baseType
			}
			skipBaseResolve := ct.QName.IsZero() && detector.IsResolving(baseQName)
			// recursively resolve base type
			if baseCT, ok := ct.BaseType().(*types.ComplexType); ok {
				if !skipBaseResolve && !detector.IsResolving(baseCT.QName) {
					if err := resolveType(baseCT.QName, baseCT, schema, detector); err != nil {
						return err
					}
				}
			}
		}
	}

	// resolve element types within the content model (including local elements).
	if err := resolveContentModelAnonymousTypes(ct.Content(), schema, detector, make(map[types.Type]bool)); err != nil {
		return err
	}

	return nil
}
