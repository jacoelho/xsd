package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

type iterationIndex struct {
	typeQNames           []model.QName
	elementQNames        []model.QName
	attributeQNames      []model.QName
	groupQNames          []model.QName
	attributeGroupQNames []model.QName

	elementRefsInContent   []*model.ElementDecl
	allIdentityConstraints []*model.IdentityConstraint
	localConstraintElems   []*model.ElementDecl
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
