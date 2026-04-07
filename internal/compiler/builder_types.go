package compiler

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

type schemaBuilder struct {
	err             error
	attrIDs         map[analysis.AttrID]runtime.AttrID
	elemIDs         map[analysis.ElemID]runtime.ElemID
	validators      *semantics.CompiledValidators
	registry        *analysis.Registry
	typeIDs         map[analysis.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	complexTypes    *semantics.ComplexTypes
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
	limits          semantics.Limits
}

const defaultMaxOccursLimit = 1_000_000
