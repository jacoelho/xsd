package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaops"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

type compiledDefaultFixed struct {
	key    runtime.ValueKeyRef
	ref    runtime.ValueRef
	member runtime.ValidatorID
	ok     bool
}

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
		var typ types.Type
		if decl.HasDefault || decl.HasFixed {
			var err error
			typ, err = c.valueTypeForElement(decl)
			if err != nil {
				return fmt.Errorf("element %s: %w", entry.QName, err)
			}
		}
		if decl.HasDefault {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("element %s default: %w", entry.QName, err)
			}
			storeDefaultFixed(c.elemDefaults, c.elemDefaultKeys, c.elemDefaultMembers, entry.ID, value)
		}
		if decl.HasFixed {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("element %s fixed: %w", entry.QName, err)
			}
			storeDefaultFixed(c.elemFixed, c.elemFixedKeys, c.elemFixedMembers, entry.ID, value)
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
		var typ types.Type
		if decl.HasDefault || decl.HasFixed {
			var err error
			typ, err = c.valueTypeForAttribute(decl)
			if err != nil {
				return fmt.Errorf("attribute %s: %w", entry.QName, err)
			}
		}
		if decl.HasDefault {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("attribute %s default: %w", entry.QName, err)
			}
			storeDefaultFixed(c.attrDefaults, c.attrDefaultKeys, c.attrDefaultMembers, entry.ID, value)
		}
		if decl.HasFixed {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("attribute %s fixed: %w", entry.QName, err)
			}
			storeDefaultFixed(c.attrFixed, c.attrFixedKeys, c.attrFixedMembers, entry.ID, value)
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
					value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
					if err != nil {
						return fmt.Errorf("attribute use %s default: %w", decl.Name, err)
					}
					storeDefaultFixed(c.attrUseDefaults, c.attrUseDefaultKeys, c.attrUseDefaultMembers, decl, value)
				}
			}
			if decl.HasFixed {
				if _, exists := c.attrUseFixed[decl]; !exists {
					value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
					if err != nil {
						return fmt.Errorf("attribute use %s fixed: %w", decl.Name, err)
					}
					storeDefaultFixed(c.attrUseFixed, c.attrUseFixedKeys, c.attrUseFixedMembers, decl, value)
				}
			}
		}
	}
	return nil
}

func (c *compiler) compileDefaultFixedValue(lexical string, typ types.Type, ctx map[string]string) (compiledDefaultFixed, error) {
	canon, member, key, err := c.canonicalizeDefaultFixed(lexical, typ, ctx)
	if err != nil {
		return compiledDefaultFixed{}, err
	}
	return compiledDefaultFixed{
		ok:     true,
		ref:    c.values.add(canon),
		key:    key,
		member: member,
	}, nil
}

func storeDefaultFixed[K comparable](
	valueMap map[K]runtime.ValueRef,
	keyMap map[K]runtime.ValueKeyRef,
	memberMap map[K]runtime.ValidatorID,
	mapKey K,
	value compiledDefaultFixed,
) {
	if !value.ok {
		return
	}
	valueMap[mapKey] = value.ref
	keyMap[mapKey] = value.key
	if value.member != 0 {
		memberMap[mapKey] = value.member
	}
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
	return schemaops.ResolveSimpleContentTextType(ct, schemaops.SimpleContentTextTypeOptions{
		ResolveQName: c.res.resolveQName,
		Cache:        c.simpleContent,
	})
}
