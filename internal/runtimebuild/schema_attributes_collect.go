package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validatorcompile"
)

func (b *schemaBuilder) collectAttrUses(ct *types.ComplexType) ([]runtime.AttrUse, *types.AnyAttribute, error) {
	if ct == nil {
		return nil, nil, nil
	}
	attrs, wildcard, err := validatorcompile.CollectAttributeUses(b.schema, ct)
	if err != nil {
		return nil, nil, err
	}
	if len(attrs) == 0 {
		return nil, wildcard, nil
	}
	out := make([]runtime.AttrUse, 0, len(attrs))
	for _, decl := range attrs {
		if decl == nil {
			continue
		}
		target := decl
		if decl.IsReference {
			target = b.resolveAttributeDecl(decl)
			if target == nil {
				return nil, nil, fmt.Errorf("runtime build: attribute ref %s not found", decl.Name)
			}
		}
		sym := b.internQName(typeops.EffectiveAttributeQName(b.schema, decl))
		use := runtime.AttrUse{
			Name: sym,
			Use:  toRuntimeAttrUse(decl.Use),
		}
		if target.Type == nil {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing type", target.Name)
		}
		vid, ok := b.validators.ValidatorForType(target.Type)
		if !ok || vid == 0 {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing validator", target.Name)
		}
		use.Validator = vid
		if decl.HasDefault {
			if def, ok := b.validators.AttrUseDefault(decl); ok {
				use.Default = def.Ref
				use.DefaultKey = def.Key
				use.DefaultMember = def.Member
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s default missing", decl.Name)
			}
		}
		if decl.HasFixed {
			if fixed, ok := b.validators.AttrUseFixed(decl); ok {
				use.Fixed = fixed.Ref
				use.FixedKey = fixed.Key
				use.FixedMember = fixed.Member
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s fixed missing", decl.Name)
			}
		}
		if !use.Default.Present && !use.Fixed.Present {
			if attrID, ok := b.schemaAttrID(target); ok {
				if def, ok := b.validators.AttributeDefault(attrID); ok {
					use.Default = def.Ref
					use.DefaultKey = def.Key
					use.DefaultMember = def.Member
				}
				if fixed, ok := b.validators.AttributeFixed(attrID); ok {
					use.Fixed = fixed.Ref
					use.FixedKey = fixed.Key
					use.FixedMember = fixed.Member
				}
			}
		}
		out = append(out, use)
	}
	return out, wildcard, nil
}

func (b *schemaBuilder) resolveAttributeDecl(decl *types.AttributeDecl) *types.AttributeDecl {
	if decl == nil {
		return nil
	}
	if !decl.IsReference {
		return decl
	}
	return b.schema.AttributeDecls[decl.Name]
}

func (b *schemaBuilder) schemaAttrID(decl *types.AttributeDecl) (schema.AttrID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.AttributeRefs[decl.Name]; ok {
			return id, true
		}
		return 0, false
	}
	if id, ok := b.registry.Attributes[decl.Name]; ok {
		return id, true
	}
	if id, ok := b.registry.LookupLocalAttributeID(decl); ok {
		return id, true
	}
	return 0, false
}
