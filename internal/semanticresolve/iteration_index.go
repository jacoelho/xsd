package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

type iterationIndex struct {
	typeQNames           []types.QName
	elementQNames        []types.QName
	attributeQNames      []types.QName
	groupQNames          []types.QName
	attributeGroupQNames []types.QName

	elementRefsInContent   []*types.ElementDecl
	allIdentityConstraints []*types.IdentityConstraint
	localConstraintElems   []*types.ElementDecl
}

func buildIterationIndex(sch *parser.Schema) *iterationIndex {
	idx := &iterationIndex{
		typeQNames:           traversal.SortedQNames(sch.TypeDefs),
		elementQNames:        traversal.SortedQNames(sch.ElementDecls),
		attributeQNames:      traversal.SortedQNames(sch.AttributeDecls),
		groupQNames:          traversal.SortedQNames(sch.Groups),
		attributeGroupQNames: traversal.SortedQNames(sch.AttributeGroups),
	}
	idx.elementRefsInContent = collectElementReferencesInSchemaWithIndex(sch, idx)
	idx.allIdentityConstraints = collectAllIdentityConstraintsWithIndex(sch, idx)
	idx.localConstraintElems = collectLocalConstraintElementsWithIndex(sch, idx)
	return idx
}
