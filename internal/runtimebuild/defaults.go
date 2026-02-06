package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileDefaults(registry *schema.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	for _, entry := range registry.ElementOrder {
		decl := entry.Decl
		if decl == nil || decl.IsReference {
			continue
		}
		if st, ok := types.AsSimpleType(decl.Type); ok && types.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("element %s type not resolved", entry.QName)
		}
		if decl.HasDefault {
			typ, err := c.valueTypeForElement(decl)
			if err != nil {
				return fmt.Errorf("element %s default: %w", entry.QName, err)
			}
			canon, member, key, err := c.canonicalizeDefaultFixed(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("element %s default: %w", entry.QName, err)
			}
			c.elemDefaults[entry.ID] = c.values.add(canon)
			c.elemDefaultKeys[entry.ID] = key
			if member != 0 {
				c.elemDefaultMembers[entry.ID] = member
			}
		}
		if decl.HasFixed {
			typ, err := c.valueTypeForElement(decl)
			if err != nil {
				return fmt.Errorf("element %s fixed: %w", entry.QName, err)
			}
			canon, member, key, err := c.canonicalizeDefaultFixed(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("element %s fixed: %w", entry.QName, err)
			}
			c.elemFixed[entry.ID] = c.values.add(canon)
			c.elemFixedKeys[entry.ID] = key
			if member != 0 {
				c.elemFixedMembers[entry.ID] = member
			}
		}
	}

	for _, entry := range registry.AttributeOrder {
		decl := entry.Decl
		if decl == nil {
			continue
		}
		if st, ok := types.AsSimpleType(decl.Type); ok && types.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("attribute %s type not resolved", entry.QName)
		}
		if decl.HasDefault {
			typ, err := c.valueTypeForAttribute(decl)
			if err != nil {
				return fmt.Errorf("attribute %s default: %w", entry.QName, err)
			}
			canon, member, key, err := c.canonicalizeDefaultFixed(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("attribute %s default: %w", entry.QName, err)
			}
			c.attrDefaults[entry.ID] = c.values.add(canon)
			c.attrDefaultKeys[entry.ID] = key
			if member != 0 {
				c.attrDefaultMembers[entry.ID] = member
			}
		}
		if decl.HasFixed {
			typ, err := c.valueTypeForAttribute(decl)
			if err != nil {
				return fmt.Errorf("attribute %s fixed: %w", entry.QName, err)
			}
			canon, member, key, err := c.canonicalizeDefaultFixed(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("attribute %s fixed: %w", entry.QName, err)
			}
			c.attrFixed[entry.ID] = c.values.add(canon)
			c.attrFixedKeys[entry.ID] = key
			if member != 0 {
				c.attrFixedMembers[entry.ID] = member
			}
		}
	}

	return nil
}

func (c *compiler) compileAttributeUses(registry *schema.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	for _, entry := range registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, _, err := collectAttributeUses(c.schema, ct)
		if err != nil {
			return err
		}
		for _, decl := range attrs {
			if decl == nil {
				continue
			}
			if st, ok := types.AsSimpleType(decl.Type); ok && types.IsPlaceholderSimpleType(st) {
				return fmt.Errorf("attribute use %s type not resolved", decl.Name)
			}
			if !decl.HasDefault && !decl.HasFixed {
				continue
			}
			typ, err := c.valueTypeForAttribute(decl)
			if err != nil {
				return fmt.Errorf("attribute use %s: %w", decl.Name, err)
			}
			if decl.HasDefault {
				if _, exists := c.attrUseDefaults[decl]; !exists {
					canon, member, key, err := c.canonicalizeDefaultFixed(decl.Default, typ, decl.DefaultContext)
					if err != nil {
						return fmt.Errorf("attribute use %s default: %w", decl.Name, err)
					}
					c.attrUseDefaults[decl] = c.values.add(canon)
					c.attrUseDefaultKeys[decl] = key
					if member != 0 {
						c.attrUseDefaultMembers[decl] = member
					}
				}
			}
			if decl.HasFixed {
				if _, exists := c.attrUseFixed[decl]; !exists {
					canon, member, key, err := c.canonicalizeDefaultFixed(decl.Fixed, typ, decl.FixedContext)
					if err != nil {
						return fmt.Errorf("attribute use %s fixed: %w", decl.Name, err)
					}
					c.attrUseFixed[decl] = c.values.add(canon)
					c.attrUseFixedKeys[decl] = key
					if member != 0 {
						c.attrUseFixedMembers[decl] = member
					}
				}
			}
		}
	}
	return nil
}

func (c *compiler) valueTypeForElement(decl *types.ElementDecl) (types.Type, error) {
	if decl == nil || decl.Type == nil {
		return nil, fmt.Errorf("missing type")
	}
	switch typed := decl.Type.(type) {
	case *types.SimpleType, *types.BuiltinType:
		return typed, nil
	case *types.ComplexType:
		textType, err := c.simpleContentTextType(typed)
		if err != nil {
			return nil, err
		}
		if textType == nil {
			return nil, fmt.Errorf("complex type has no simple content")
		}
		return textType, nil
	default:
		return nil, fmt.Errorf("unsupported element type")
	}
}

func (c *compiler) valueTypeForAttribute(decl *types.AttributeDecl) (types.Type, error) {
	if decl == nil {
		return nil, fmt.Errorf("missing attribute")
	}
	if decl.Type != nil {
		return decl.Type, nil
	}
	if decl.IsReference && c.schema != nil {
		if target := c.schema.AttributeDecls[decl.Name]; target != nil && target.Type != nil {
			return target.Type, nil
		}
	}
	return nil, fmt.Errorf("missing attribute type")
}

func (c *compiler) canonicalizeDefaultFixed(lexical string, typ types.Type, ctx map[string]string) ([]byte, runtime.ValidatorID, runtime.ValueKeyRef, error) {
	normalized := c.normalizeLexical(lexical, typ)
	facets, err := c.facetsForType(typ)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	err = c.validatePartialFacets(normalized, typ, facets)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	canon, memberType, err := c.canonicalizeNormalizedDefaultWithMember(lexical, normalized, typ, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	enumErr := c.validateEnumSets(lexical, normalized, typ, ctx)
	if enumErr != nil {
		return nil, 0, runtime.ValueKeyRef{}, enumErr
	}
	keyRef, err := c.defaultFixedKeyRef(lexical, normalized, typ, memberType, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	memberID := runtime.ValidatorID(0)
	if memberType != nil {
		memberID, err = c.compileType(memberType)
		if err != nil {
			return nil, 0, runtime.ValueKeyRef{}, err
		}
	}
	return canon, memberID, keyRef, nil
}

func (c *compiler) defaultFixedKeyRef(lexical, normalized string, typ, memberType types.Type, ctx map[string]string) (runtime.ValueKeyRef, error) {
	keyType := typ
	keyLexical := normalized
	if memberType != nil {
		keyType = memberType
		keyLexical = c.normalizeLexical(lexical, memberType)
	}
	keys, err := c.valueKeysForNormalized(lexical, keyLexical, keyType, ctx)
	if err != nil {
		return runtime.ValueKeyRef{}, err
	}
	if len(keys) == 0 {
		return runtime.ValueKeyRef{}, fmt.Errorf("no value key produced")
	}
	key := keys[0]
	return runtime.ValueKeyRef{
		Kind: key.Kind,
		Ref:  c.values.add(key.Bytes),
	}, nil
}

func (c *compiler) canonicalizeNormalizedDefault(lexical, normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	return c.canonicalizeNormalizedCore(lexical, normalized, typ, ctx, canonicalizeDefault)
}

func (c *compiler) canonicalizeNormalizedDefaultWithMember(lexical, normalized string, typ types.Type, ctx map[string]string) ([]byte, types.Type, error) {
	if c.res.varietyForType(typ) != types.UnionVariety {
		canon, err := c.canonicalizeNormalizedCore(lexical, normalized, typ, ctx, canonicalizeDefault)
		return canon, nil, err
	}
	members := c.res.unionMemberTypesFromType(typ)
	if len(members) == 0 {
		return nil, nil, fmt.Errorf("union has no member types")
	}
	for _, member := range members {
		memberLex := c.normalizeLexical(lexical, member)
		memberFacets, err := c.facetsForType(member)
		if err != nil {
			return nil, nil, err
		}
		if validateErr := c.validatePartialFacets(memberLex, member, memberFacets); validateErr != nil {
			continue
		}
		canon, canonErr := c.canonicalizeNormalizedCore(lexical, memberLex, member, ctx, canonicalizeDefault)
		if canonErr != nil {
			continue
		}
		if enumErr := c.validateEnumSets(lexical, memberLex, member, ctx); enumErr != nil {
			continue
		}
		return canon, member, nil
	}
	return nil, nil, fmt.Errorf("union value does not match any member type")
}

func (c *compiler) validateEnumSets(lexical, normalized string, typ types.Type, ctx map[string]string) error {
	validatorID, err := c.compileType(typ)
	if err != nil {
		return err
	}
	if validatorID == 0 {
		return nil
	}
	enumIDs := c.enumIDsForValidator(validatorID)
	if len(enumIDs) == 0 {
		return nil
	}
	keys, err := c.keyBytesForNormalized(lexical, normalized, typ, ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("value not in enumeration")
	}
	table := c.enums.table()
	for _, key := range keys {
		matched := true
		for _, enumID := range enumIDs {
			if !runtime.EnumContains(&table, enumID, key.kind, key.bytes) {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("value not in enumeration")
}

func (c *compiler) enumIDsForValidator(id runtime.ValidatorID) []runtime.EnumID {
	if id == 0 {
		return nil
	}
	if int(id) >= len(c.bundle.Meta) {
		return nil
	}
	meta := c.bundle.Meta[id]
	if meta.Facets.Len == 0 {
		return nil
	}
	start := meta.Facets.Off
	var out []runtime.EnumID
	for i := uint32(0); i < meta.Facets.Len; i++ {
		instr := c.facets[start+i]
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	return out
}

func (c *compiler) simpleContentTextType(ct *types.ComplexType) (types.Type, error) {
	if ct == nil {
		return nil, nil
	}
	if cached, ok := c.simpleContent[ct]; ok {
		return cached, nil
	}
	seen := make(map[*types.ComplexType]bool)
	return c.simpleContentTextTypeSeen(ct, seen)
}

func (c *compiler) simpleContentTextTypeSeen(ct *types.ComplexType, seen map[*types.ComplexType]bool) (types.Type, error) {
	if ct == nil {
		return nil, nil
	}
	if cached, ok := c.simpleContent[ct]; ok {
		return cached, nil
	}
	if seen[ct] {
		return nil, fmt.Errorf("simpleContent cycle detected")
	}
	seen[ct] = true
	defer delete(seen, ct)

	sc, ok := ct.Content().(*types.SimpleContent)
	if !ok {
		return nil, nil
	}
	baseType, err := c.simpleContentBaseType(ct, sc, seen)
	if err != nil {
		return nil, err
	}
	var result types.Type
	switch {
	case sc.Extension != nil:
		result = baseType
	case sc.Restriction != nil:
		st := &types.SimpleType{
			Restriction:  sc.Restriction,
			ResolvedBase: baseType,
		}
		if sc.Restriction.SimpleType != nil && sc.Restriction.SimpleType.WhiteSpaceExplicit() {
			st.SetWhiteSpaceExplicit(sc.Restriction.SimpleType.WhiteSpace())
		} else if baseType != nil {
			st.SetWhiteSpace(baseType.WhiteSpace())
		}
		result = st
	default:
		result = baseType
	}
	c.simpleContent[ct] = result
	return result, nil
}

func (c *compiler) simpleContentBaseType(ct *types.ComplexType, sc *types.SimpleContent, seen map[*types.ComplexType]bool) (types.Type, error) {
	if ct == nil {
		return nil, fmt.Errorf("simpleContent base missing")
	}
	base := ct.ResolvedBase
	if base == nil && sc != nil {
		qname := sc.BaseTypeQName()
		if !qname.IsZero() {
			base = c.res.resolveQName(qname)
		}
	}
	if base == nil {
		return nil, fmt.Errorf("simpleContent base missing")
	}
	switch typed := base.(type) {
	case *types.SimpleType, *types.BuiltinType:
		return typed, nil
	case *types.ComplexType:
		return c.simpleContentTextTypeSeen(typed, seen)
	default:
		return nil, fmt.Errorf("simpleContent base is not simple")
	}
}
