package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func isSubstitutableElement(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil {
		return false
	}
	if headDecl.Block.Has(types.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := resolveTypeForFinalValidation(schema, headDecl.Type)
	memberType := resolveTypeForFinalValidation(schema, memberDecl.Type)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*types.ComplexType); ok {
		combinedBlock = combinedBlock.Add(types.DerivationMethod(headCT.Block))
	}
	if isDerivationBlocked(memberType, headType, combinedBlock) {
		return false
	}
	return true
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[types.QName]bool)
	var walk func(types.QName) bool
	walk = func(current types.QName) bool {
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, sub := range schema.SubstitutionGroups[current] {
			if sub == member {
				return true
			}
			if walk(sub) {
				return true
			}
		}
		return false
	}
	return walk(head)
}

func isDerivationBlocked(memberType, headType types.Type, block types.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := types.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ types.Type) types.DerivationMethod {
	switch typed := typ.(type) {
	case *types.ComplexType:
		return typed.DerivationMethod
	case *types.SimpleType:
		if typed.List != nil || typed.Variety() == types.ListVariety {
			return types.DerivationList
		}
		if typed.Union != nil || typed.Variety() == types.UnionVariety {
			return types.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return types.DerivationRestriction
		}
	case *types.BuiltinType:
		return types.DerivationRestriction
	}
	return 0
}
