package lower

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

type schemaBuilder struct {
	err             error
	attrIDs         map[ids.AttrID]runtime.AttrID
	elemIDs         map[ids.ElemID]runtime.ElemID
	validators      *CompiledValidators
	registry        *analysis.Registry
	typeIDs         map[ids.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	complexTypes    *ComplexTypePlan
	builtinIDs      map[model.TypeName]runtime.TypeID
	refs            *analysis.ResolvedReferences
	anyElementRules map[*model.AnyElement]runtime.WildcardID
	rt              *runtime.Schema
	paths           []runtime.PathProgram
	wildcards       []runtime.WildcardRule
	wildcardNS      []runtime.NamespaceID
	notations       []runtime.SymbolID
	maxOccurs       uint32
	anyTypeComplex  uint32
	limits          contentmodel.Limits
}

const defaultMaxOccursLimit = 1_000_000
