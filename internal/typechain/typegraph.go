package typechain

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
)

// LookupType returns a type definition by QName from the schema.
func LookupType(schema *parser.Schema, qname model.QName) (model.Type, bool) {
	if schema == nil {
		return nil, false
	}
	typ, ok := schema.TypeDefs[qname]
	return typ, ok
}

// LookupComplexType returns a complex type definition by QName.
func LookupComplexType(schema *parser.Schema, qname model.QName) (*model.ComplexType, bool) {
	typ, ok := LookupType(schema, qname)
	if !ok {
		return nil, false
	}
	ct, ok := model.AsComplexType(typ)
	return ct, ok
}

// ResolveBaseComplexType resolves a complex base type, including xs:anyType.
func ResolveBaseComplexType(schema *parser.Schema, ct *model.ComplexType, baseQName model.QName) *model.ComplexType {
	if ct != nil && ct.ResolvedBase != nil {
		if baseCT, ok := model.AsComplexType(ct.ResolvedBase); ok {
			return baseCT
		}
		if model.IsAnyTypeQName(ct.ResolvedBase.Name()) {
			return model.NewAnyTypeComplexType()
		}
	}
	if schema != nil && !baseQName.IsZero() {
		if model.IsAnyTypeQName(baseQName) {
			return model.NewAnyTypeComplexType()
		}
		if baseCT, ok := LookupComplexType(schema, baseQName); ok {
			return baseCT
		}
	}
	return nil
}

// EffectiveContentParticle returns the effective element particle for a type.
func EffectiveContentParticle(schema *parser.Schema, typ model.Type) model.Particle {
	ct, ok := model.AsComplexType(typ)
	if !ok || ct == nil {
		return nil
	}
	return effectiveContentParticleForComplexType(schema, ct, make(map[*model.ComplexType]bool))
}

func effectiveContentParticleForComplexType(schema *parser.Schema, ct *model.ComplexType, visited map[*model.ComplexType]bool) model.Particle {
	if ct == nil {
		return nil
	}
	if visited[ct] {
		return nil
	}
	visited[ct] = true
	defer delete(visited, ct)

	switch content := ct.Content().(type) {
	case *model.ElementContent:
		return content.Particle
	case *model.SimpleContent, *model.EmptyContent:
		return nil
	case *model.ComplexContent:
		if content.Restriction != nil {
			return content.Restriction.Particle
		}
		if content.Extension != nil {
			baseCT := ResolveBaseComplexType(schema, ct, content.BaseTypeQName())
			baseParticle := effectiveContentParticleForComplexType(schema, baseCT, visited)
			return combineExtensionParticles(baseParticle, content.Extension.Particle)
		}
	}
	return nil
}

func combineExtensionParticles(baseParticle, extParticle model.Particle) model.Particle {
	if baseParticle == nil {
		return extParticle
	}
	if extParticle == nil {
		return baseParticle
	}
	return &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{baseParticle, extParticle},
	}
}

// ComplexTypeChainMode controls how implicit anyType is handled in type chains.
type ComplexTypeChainMode uint8

const (
	ComplexTypeChainExplicitBaseOnly ComplexTypeChainMode = iota
	ComplexTypeChainAllowImplicitAnyType
)

// CollectComplexTypeChain walks base links from a complex type to root.
func CollectComplexTypeChain(schema *parser.Schema, ct *model.ComplexType, mode ComplexTypeChainMode) []*model.ComplexType {
	var chain []*model.ComplexType
	visited := make(map[*model.ComplexType]bool)
	for current := ct; current != nil; {
		if visited[current] {
			break
		}
		visited[current] = true
		chain = append(chain, current)
		next := nextBaseComplexType(schema, current, mode)
		if next == nil {
			break
		}
		current = next
	}
	return chain
}

func nextBaseComplexType(schema *parser.Schema, current *model.ComplexType, mode ComplexTypeChainMode) *model.ComplexType {
	if current == nil {
		return nil
	}
	baseQName := model.QName{}
	if content := current.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if current.ResolvedBase == nil && baseQName.IsZero() {
		if mode == ComplexTypeChainAllowImplicitAnyType && !model.IsAnyTypeQName(current.QName) {
			return model.NewAnyTypeComplexType()
		}
		return nil
	}

	next, _, err := nextDerivationStep(current, func(name model.QName) (model.Type, error) {
		if name.IsZero() {
			return nil, nil
		}
		if model.IsAnyTypeQName(name) {
			return model.NewAnyTypeComplexType(), nil
		}
		typ, ok := LookupType(schema, name)
		if !ok {
			return nil, nil
		}
		return typ, nil
	})
	if err != nil {
		return nil
	}
	if baseCT, ok := model.AsComplexType(next); ok {
		return baseCT
	}
	if mode == ComplexTypeChainAllowImplicitAnyType && next != nil && model.IsAnyTypeQName(next.Name()) {
		return model.NewAnyTypeComplexType()
	}
	return nil
}

type typeQNameResolver func(model.QName) (model.Type, error)

func nextDerivationStep(current model.Type, resolve typeQNameResolver) (model.Type, model.DerivationMethod, error) {
	switch typed := current.(type) {
	case *model.ComplexType:
		method := typed.DerivationMethod
		if method == 0 {
			method = model.DerivationRestriction
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, method, nil
		}
		baseQName := model.QName{}
		if content := typed.Content(); content != nil {
			baseQName = content.BaseTypeQName()
		}
		if !baseQName.IsZero() && resolve != nil {
			base, err := resolve(baseQName)
			if err != nil {
				return nil, method, err
			}
			return base, method, nil
		}
		return typed.BaseType(), method, nil
	case *model.SimpleType:
		if typed.List != nil {
			return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationList, nil
		}
		if typed.Union != nil {
			return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationUnion, nil
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, model.DerivationRestriction, nil
		}
		if typed.Restriction != nil {
			if typed.Restriction.SimpleType != nil {
				return typed.Restriction.SimpleType, model.DerivationRestriction, nil
			}
			if !typed.Restriction.Base.IsZero() && resolve != nil {
				base, err := resolve(typed.Restriction.Base)
				if err != nil {
					return nil, model.DerivationRestriction, err
				}
				return base, model.DerivationRestriction, nil
			}
		}
		return nil, 0, nil
	case *model.BuiltinType:
		name := model.TypeName(typed.Name().Local)
		switch name {
		case model.TypeNameAnyType:
			return nil, 0, nil
		case model.TypeNameAnySimpleType:
			return model.GetBuiltin(model.TypeNameAnyType), model.DerivationRestriction, nil
		default:
			if _, ok := model.BuiltinListItemTypeName(typed.Name().Local); ok {
				return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationList, nil
			}
			base := typed.BaseType()
			if base == nil {
				return nil, 0, nil
			}
			return base, model.DerivationRestriction, nil
		}
	default:
		return nil, 0, nil
	}
}
