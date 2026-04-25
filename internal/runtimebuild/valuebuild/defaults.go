package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (c *artifactCompiler) compileDefaults() error {
	for _, entry := range c.schema.Elements {
		spec, err := c.valueSpecForElement(entry)
		if err != nil {
			if entry.Default.Present || entry.Fixed.Present {
				return fmt.Errorf("element %s: %w", formatName(entry.Name), err)
			}
			continue
		}
		if entry.Default.Present {
			value, err := c.compileDefaultFixedValue(entry.Default, spec)
			if err != nil {
				return fmt.Errorf("element %s default: %w", formatName(entry.Name), err)
			}
			c.out.ElementDefaults[entry.ID] = value
		}
		if entry.Fixed.Present {
			value, err := c.compileDefaultFixedValue(entry.Fixed, spec)
			if err != nil {
				return fmt.Errorf("element %s fixed: %w", formatName(entry.Name), err)
			}
			c.out.ElementFixed[entry.ID] = value
		}
	}
	for _, entry := range c.schema.Attributes {
		spec, ok := c.specForRef(entry.TypeDecl)
		if !ok {
			if entry.Default.Present || entry.Fixed.Present {
				return fmt.Errorf("attribute %s missing value type", formatName(entry.Name))
			}
			continue
		}
		if entry.Default.Present {
			value, err := c.compileDefaultFixedValue(entry.Default, spec)
			if err != nil {
				return fmt.Errorf("attribute %s default: %w", formatName(entry.Name), err)
			}
			c.out.AttributeDefaults[entry.ID] = value
		}
		if entry.Fixed.Present {
			value, err := c.compileDefaultFixedValue(entry.Fixed, spec)
			if err != nil {
				return fmt.Errorf("attribute %s fixed: %w", formatName(entry.Name), err)
			}
			c.out.AttributeFixed[entry.ID] = value
		}
	}
	seenAttrUse := make(map[schemair.AttributeUseID]bool, len(c.schema.AttributeUses))
	for _, plan := range c.schema.ComplexTypes {
		for _, id := range plan.Attrs {
			if err := c.compileAttributeUseDefaultFixed(id, seenAttrUse); err != nil {
				return err
			}
		}
	}
	for _, entry := range c.schema.AttributeUses {
		if err := c.compileAttributeUseDefaultFixed(entry.ID, seenAttrUse); err != nil {
			return err
		}
	}
	return nil
}

func (c *artifactCompiler) compileAttributeUseDefaultFixed(id schemair.AttributeUseID, seen map[schemair.AttributeUseID]bool) error {
	if id == 0 || seen[id] {
		return nil
	}
	seen[id] = true
	if int(id) > len(c.schema.AttributeUses) {
		return fmt.Errorf("attribute use %d out of range", id)
	}
	entry := c.schema.AttributeUses[id-1]
	spec, ok := c.specForRef(entry.TypeDecl)
	if !ok {
		if entry.Default.Present || entry.Fixed.Present {
			return fmt.Errorf("attribute use %s missing value type", formatName(entry.Name))
		}
		return nil
	}
	if entry.Default.Present {
		value, err := c.compileDefaultFixedValue(entry.Default, spec)
		if err != nil {
			return fmt.Errorf("attribute use %s default: %w", formatName(entry.Name), err)
		}
		c.out.AttrUseDefaults[entry.ID] = value
	}
	if entry.Fixed.Present {
		value, err := c.compileDefaultFixedValue(entry.Fixed, spec)
		if err != nil {
			return fmt.Errorf("attribute use %s fixed: %w", formatName(entry.Name), err)
		}
		c.out.AttrUseFixed[entry.ID] = value
	}
	return nil
}

func (c *artifactCompiler) valueSpecForElement(entry schemair.Element) (schemair.SimpleTypeSpec, error) {
	if entry.TypeDecl.Builtin {
		if entry.TypeDecl.Name.Local == "anyType" {
			if spec, ok := c.builtinSpecs["anySimpleType"]; ok {
				return spec, nil
			}
		}
		if spec, ok := c.specForRef(entry.TypeDecl); ok {
			return spec, nil
		}
		return schemair.SimpleTypeSpec{}, fmt.Errorf("missing value type")
	}
	switch c.typeKinds[entry.TypeDecl.ID] {
	case schemair.TypeSimple:
		if spec, ok := c.specForRef(entry.TypeDecl); ok {
			return spec, nil
		}
	case schemair.TypeComplex:
		plan, ok := c.complexPlans[entry.TypeDecl.ID]
		if !ok {
			return schemair.SimpleTypeSpec{}, fmt.Errorf("complex type missing plan")
		}
		if plan.Content == schemair.ContentSimple {
			if isZeroRef(plan.TextType) {
				if !isZeroSpec(plan.TextSpec) {
					return plan.TextSpec, nil
				}
				return schemair.SimpleTypeSpec{}, fmt.Errorf("complex type missing simple content type")
			}
			if spec, ok := c.specForRef(plan.TextType); ok {
				return spec, nil
			}
			return schemair.SimpleTypeSpec{}, fmt.Errorf("complex type missing simple content type")
		}
		if plan.Mixed {
			if spec, ok := c.builtinSpecs["anySimpleType"]; ok {
				return spec, nil
			}
		}
		return schemair.SimpleTypeSpec{}, fmt.Errorf("complex type has no simple content")
	}
	return schemair.SimpleTypeSpec{}, fmt.Errorf("missing value type")
}

func (c *artifactCompiler) compileDefaultFixedValue(constraint schemair.ValueConstraint, spec schemair.SimpleTypeSpec) (DefaultFixedValue, error) {
	canon, member, key, err := c.canonicalizeDefaultFixed(constraint.Lexical, spec, constraint.Context)
	if err != nil {
		return DefaultFixedValue{}, err
	}
	return DefaultFixedValue{
		Ref:    c.values.addWithHash(canon, runtime.HashBytes(canon)),
		Key:    key,
		Member: member,
	}, nil
}

func (c *artifactCompiler) canonicalizeDefaultFixed(lexical string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]byte, runtime.ValidatorID, runtime.ValueKeyRef, error) {
	normalized := c.normalizeLexical(lexical, spec)
	if err := c.validatePartialFacets(normalized, spec, spec.Facets, ctx); err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	canon, memberSpec, err := c.canonicalizeDefaultWithMember(lexical, normalized, spec, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	if err := c.validateEnumSets(lexical, normalized, spec, ctx); err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	keyRef, err := c.defaultFixedKeyRef(lexical, normalized, spec, memberSpec, ctx)
	if err != nil {
		return nil, 0, runtime.ValueKeyRef{}, err
	}
	memberID := runtime.ValidatorID(0)
	if !isZeroSpec(memberSpec) {
		memberID, err = c.compileSpec(memberSpec)
		if err != nil {
			return nil, 0, runtime.ValueKeyRef{}, err
		}
	}
	return canon, memberID, keyRef, nil
}

func (c *artifactCompiler) canonicalizeDefaultWithMember(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]byte, schemair.SimpleTypeSpec, error) {
	if spec.Variety != schemair.TypeVarietyUnion {
		canon, err := c.canonicalizeNormalized(lexical, normalized, spec, ctx, canonicalizeDefault)
		return canon, schemair.SimpleTypeSpec{}, err
	}
	if len(spec.Members) == 0 {
		return nil, schemair.SimpleTypeSpec{}, fmt.Errorf("union has no member types")
	}
	for _, ref := range spec.Members {
		member, ok := c.specForRef(ref)
		if !ok {
			continue
		}
		memberLex := c.normalizeLexical(lexical, member)
		if err := c.validatePartialFacets(memberLex, member, member.Facets, ctx); err != nil {
			continue
		}
		canon, err := c.canonicalizeNormalized(lexical, memberLex, member, ctx, canonicalizeDefault)
		if err != nil {
			continue
		}
		if err := c.validateEnumSets(lexical, memberLex, member, ctx); err != nil {
			continue
		}
		if member.Primitive == "string" && member.BuiltinBase == "string" && member.Whitespace == schemair.WhitespacePreserve {
			return []byte(lexical), member, nil
		}
		return canon, member, nil
	}
	return nil, schemair.SimpleTypeSpec{}, fmt.Errorf("union value does not match any member type")
}

func (c *artifactCompiler) defaultFixedKeyRef(lexical, normalized string, spec, memberSpec schemair.SimpleTypeSpec, ctx map[string]string) (runtime.ValueKeyRef, error) {
	keySpec := spec
	keyLexical := normalized
	if !isZeroSpec(memberSpec) {
		keySpec = memberSpec
		keyLexical = c.normalizeLexical(lexical, memberSpec)
	}
	keys, err := c.valueKeysForNormalized(lexical, keyLexical, keySpec, ctx)
	if err != nil {
		return runtime.ValueKeyRef{}, err
	}
	if len(keys) == 0 {
		return runtime.ValueKeyRef{}, fmt.Errorf("no value key produced")
	}
	key := keys[0]
	return runtime.ValueKeyRef{
		Kind: key.Kind,
		Ref:  c.values.addWithHash(key.Bytes, runtime.HashBytes(key.Bytes)),
	}, nil
}

func isZeroSpec(spec schemair.SimpleTypeSpec) bool {
	return spec.TypeDecl == 0 &&
		spec.Name.Local == "" &&
		spec.Name.Namespace == "" &&
		!spec.Builtin &&
		spec.Primitive == "" &&
		spec.BuiltinBase == "" &&
		spec.Variety == schemair.TypeVarietyAtomic &&
		isZeroRef(spec.Base) &&
		isZeroRef(spec.Item) &&
		len(spec.Members) == 0 &&
		len(spec.Facets) == 0
}
