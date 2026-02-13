package runtimeassemble

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

type schemaBuilder struct {
	err             error
	attrIDs         map[ids.AttrID]runtime.AttrID
	elemIDs         map[ids.ElemID]runtime.ElemID
	validators      *validatorgen.CompiledValidators
	registry        *analysis.Registry
	typeIDs         map[ids.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	complexTypes    *complextypeplan.Plan
	builtinIDs      map[types.TypeName]runtime.TypeID
	refs            *analysis.ResolvedReferences
	anyElementRules map[*types.AnyElement]runtime.WildcardID
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
