package runtimeassemble

import (
	schema "github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/ids"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	model "github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

type schemaBuilder struct {
	err             error
	attrIDs         map[ids.AttrID]runtime.AttrID
	elemIDs         map[ids.ElemID]runtime.ElemID
	validators      *validatorgen.CompiledValidators
	registry        *schema.Registry
	typeIDs         map[ids.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	complexTypes    *complextypeplan.Plan
	builtinIDs      map[model.TypeName]runtime.TypeID
	refs            *schema.ResolvedReferences
	anyElementRules map[*model.AnyElement]runtime.WildcardID
	rt              *runtime.Schema
	paths           []runtime.PathProgram
	wildcards       []runtime.WildcardRule
	wildcardNS      []runtime.NamespaceID
	notations       []runtime.SymbolID
	maxOccurs       uint32
	anyTypeComplex  uint32
	limits          models.Limits
}

const defaultMaxOccursLimit = 1_000_000
