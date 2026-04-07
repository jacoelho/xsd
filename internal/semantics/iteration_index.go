package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
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
		typeQNames:           model.SortedMapKeys(sch.TypeDefs),
		elementQNames:        model.SortedMapKeys(sch.ElementDecls),
		attributeQNames:      model.SortedMapKeys(sch.AttributeDecls),
		groupQNames:          model.SortedMapKeys(sch.Groups),
		attributeGroupQNames: model.SortedMapKeys(sch.AttributeGroups),
	}
	idx.elementRefsInContent = collectElementReferencesInSchemaWithIndex(sch, idx)
	idx.allIdentityConstraints = CollectAllIdentityConstraints(sch)
	idx.localConstraintElems = CollectLocalConstraintElements(sch)
	return idx
}
