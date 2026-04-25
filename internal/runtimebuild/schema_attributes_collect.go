package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) collectAttrUses(ids []schemair.AttributeUseID) ([]runtime.AttrUse, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]runtime.AttrUse, 0, len(ids))
	for _, id := range ids {
		if id == 0 || int(id) > len(b.schema.AttributeUses) {
			return nil, fmt.Errorf("runtime build: attribute use %d out of range", id)
		}
		entry := b.schema.AttributeUses[id-1]
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return nil, err
		}
		validator, ok := b.validatorForIRTypeRef(entry.TypeDecl)
		if !ok || validator == 0 {
			return nil, fmt.Errorf("runtime build: attribute use %s missing validator", formatIRName(entry.Name))
		}
		use := runtime.AttrUse{
			Name:      sym,
			Validator: validator,
			Use:       toRuntimeIRAttrUse(entry.Use),
		}
		if def, ok := b.artifacts.AttrUseDefaults[id]; ok {
			use.Default = def.Ref
			use.DefaultKey = def.Key
			use.DefaultMember = def.Member
		}
		if fixed, ok := b.artifacts.AttrUseFixed[id]; ok {
			use.Fixed = fixed.Ref
			use.FixedKey = fixed.Key
			use.FixedMember = fixed.Member
		}
		if !use.Default.Present && !use.Fixed.Present && entry.Decl != 0 {
			if def, ok := b.artifacts.AttributeDefaults[entry.Decl]; ok {
				use.Default = def.Ref
				use.DefaultKey = def.Key
				use.DefaultMember = def.Member
			}
			if fixed, ok := b.artifacts.AttributeFixed[entry.Decl]; ok {
				use.Fixed = fixed.Ref
				use.FixedKey = fixed.Key
				use.FixedMember = fixed.Member
			}
		}
		out = append(out, use)
	}
	return out, nil
}

func toRuntimeIRAttrUse(use schemair.AttributeUseKind) runtime.AttrUseKind {
	switch use {
	case schemair.AttributeRequired:
		return runtime.AttrRequired
	case schemair.AttributeProhibited:
		return runtime.AttrProhibited
	default:
		return runtime.AttrOptional
	}
}
