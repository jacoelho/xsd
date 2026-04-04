package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/semantics"
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
		typeQNames:           qname.SortedMapKeys(sch.TypeDefs),
		elementQNames:        qname.SortedMapKeys(sch.ElementDecls),
		attributeQNames:      qname.SortedMapKeys(sch.AttributeDecls),
		groupQNames:          qname.SortedMapKeys(sch.Groups),
		attributeGroupQNames: qname.SortedMapKeys(sch.AttributeGroups),
	}
	idx.elementRefsInContent = collectElementReferencesInSchemaWithIndex(sch, idx)
	idx.allIdentityConstraints = semantics.CollectAllIdentityConstraints(sch)
	idx.localConstraintElems = semantics.CollectLocalConstraintElements(sch)
	return idx
}
