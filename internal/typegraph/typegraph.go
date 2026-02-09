package typegraph

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// LookupType returns a type definition by QName from the schema.
func LookupType(schema *parser.Schema, qname types.QName) (types.Type, bool) {
	if schema == nil {
		return nil, false
	}
	typ, ok := schema.TypeDefs[qname]
	return typ, ok
}

// LookupComplexType returns a complex type definition by QName.
func LookupComplexType(schema *parser.Schema, qname types.QName) (*types.ComplexType, bool) {
	typ, ok := LookupType(schema, qname)
	if !ok {
		return nil, false
	}
	ct, ok := types.AsComplexType(typ)
	return ct, ok
}

// ResolveBaseComplexType resolves a complex base type, including xs:anyType.
func ResolveBaseComplexType(schema *parser.Schema, ct *types.ComplexType, baseQName types.QName) *types.ComplexType {
	if ct != nil && ct.ResolvedBase != nil {
		if baseCT, ok := types.AsComplexType(ct.ResolvedBase); ok {
			return baseCT
		}
		if types.IsAnyTypeQName(ct.ResolvedBase.Name()) {
			return types.NewAnyTypeComplexType()
		}
	}
	if schema != nil && !baseQName.IsZero() {
		if types.IsAnyTypeQName(baseQName) {
			return types.NewAnyTypeComplexType()
		}
		if baseCT, ok := LookupComplexType(schema, baseQName); ok {
			return baseCT
		}
	}
	return nil
}

// EffectiveContentParticle returns the effective element particle for a type.
func EffectiveContentParticle(schema *parser.Schema, typ types.Type) types.Particle {
	ct, ok := types.AsComplexType(typ)
	if !ok || ct == nil {
		return nil
	}
	return effectiveContentParticleForComplexType(schema, ct, make(map[*types.ComplexType]bool))
}

func effectiveContentParticleForComplexType(schema *parser.Schema, ct *types.ComplexType, visited map[*types.ComplexType]bool) types.Particle {
	if ct == nil {
		return nil
	}
	if visited[ct] {
		return nil
	}
	visited[ct] = true
	defer delete(visited, ct)

	switch content := ct.Content().(type) {
	case *types.ElementContent:
		return content.Particle
	case *types.SimpleContent, *types.EmptyContent:
		return nil
	case *types.ComplexContent:
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

func combineExtensionParticles(baseParticle, extParticle types.Particle) types.Particle {
	if baseParticle == nil {
		return extParticle
	}
	if extParticle == nil {
		return baseParticle
	}
	return &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{baseParticle, extParticle},
	}
}

// ComplexTypeChainMode controls how implicit anyType is handled in type chains.
type ComplexTypeChainMode uint8

const (
	ComplexTypeChainExplicitBaseOnly ComplexTypeChainMode = iota
	ComplexTypeChainAllowImplicitAnyType
)

// CollectComplexTypeChain walks base links from a complex type to root.
func CollectComplexTypeChain(schema *parser.Schema, ct *types.ComplexType, mode ComplexTypeChainMode) []*types.ComplexType {
	var chain []*types.ComplexType
	visited := make(map[*types.ComplexType]bool)
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

func nextBaseComplexType(schema *parser.Schema, current *types.ComplexType, mode ComplexTypeChainMode) *types.ComplexType {
	if current == nil {
		return nil
	}
	if baseCT, ok := current.ResolvedBase.(*types.ComplexType); ok {
		return baseCT
	}
	if current.ResolvedBase != nil {
		if mode == ComplexTypeChainAllowImplicitAnyType && types.IsAnyTypeQName(current.ResolvedBase.Name()) {
			return types.NewAnyTypeComplexType()
		}
		return nil
	}

	baseQName := types.QName{}
	if content := current.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if !baseQName.IsZero() {
		if types.IsAnyTypeQName(baseQName) {
			if mode == ComplexTypeChainAllowImplicitAnyType {
				return types.NewAnyTypeComplexType()
			}
			return nil
		}
		if baseCT, ok := LookupComplexType(schema, baseQName); ok {
			return baseCT
		}
		return nil
	}
	if mode == ComplexTypeChainAllowImplicitAnyType && !types.IsAnyTypeQName(current.QName) {
		return types.NewAnyTypeComplexType()
	}
	return nil
}
