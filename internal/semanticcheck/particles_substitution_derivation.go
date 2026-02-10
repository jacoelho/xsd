package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func isSubstitutableElement(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil {
		return false
	}
	if headDecl.Block.Has(model.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := typeresolve.ResolveTypeReference(schema, headDecl.Type, typeresolve.TypeReferenceAllowMissing)
	memberType := typeresolve.ResolveTypeReference(schema, memberDecl.Type, typeresolve.TypeReferenceAllowMissing)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*model.ComplexType); ok {
		combinedBlock = combinedBlock.Add(model.DerivationMethod(headCT.Block))
	}
	if isDerivationBlocked(memberType, headType, combinedBlock) {
		return false
	}
	return true
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[model.QName]bool)
	var walk func(model.QName) bool
	walk = func(current model.QName) bool {
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

func isDerivationBlocked(memberType, headType model.Type, block model.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := model.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ model.Type) model.DerivationMethod {
	switch typed := typ.(type) {
	case *model.ComplexType:
		return typed.DerivationMethod
	case *model.SimpleType:
		if typed.List != nil || typed.Variety() == model.ListVariety {
			return model.DerivationList
		}
		if typed.Union != nil || typed.Variety() == model.UnionVariety {
			return model.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return model.DerivationRestriction
		}
	case *model.BuiltinType:
		return model.DerivationRestriction
	}
	return 0
}
