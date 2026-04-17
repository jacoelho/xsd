package semantics

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

// ResolvedReferences records resolved references without mutating the parsed schema.
type ResolvedReferences struct {
	ElementRefs   map[model.QName]analysis.ElemID
	AttributeRefs map[model.QName]analysis.AttrID
	GroupRefs     map[model.QName]model.QName
}
