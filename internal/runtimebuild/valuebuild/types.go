package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

// DefaultFixedValue stores canonical default/fixed metadata for runtime tables.
type DefaultFixedValue struct {
	Key    runtime.ValueKeyRef
	Ref    runtime.ValueRef
	Member runtime.ValidatorID
}

// Artifacts contains all runtime validator artifacts generated from schema IR.
type Artifacts struct {
	TypeValidators    map[schemair.TypeID]runtime.ValidatorID
	BuiltinValidators map[string]runtime.ValidatorID
	TextValidators    map[schemair.TypeID]runtime.ValidatorID
	ElementDefaults   map[schemair.ElementID]DefaultFixedValue
	ElementFixed      map[schemair.ElementID]DefaultFixedValue
	AttributeDefaults map[schemair.AttributeID]DefaultFixedValue
	AttributeFixed    map[schemair.AttributeID]DefaultFixedValue
	AttrUseDefaults   map[schemair.AttributeUseID]DefaultFixedValue
	AttrUseFixed      map[schemair.AttributeUseID]DefaultFixedValue
	Validators        runtime.ValidatorsBundle
	Enums             runtime.EnumTable
	Facets            []runtime.FacetInstr
	Patterns          []runtime.Pattern
	Values            runtime.ValueBlob
}

type typeKey struct {
	builtin string
	id      schemair.TypeID
}

type artifactCompiler struct {
	schema            *schemair.Schema
	builtinSpecs      map[string]schemair.SimpleTypeSpec
	builtinRuntimeIDs map[string]runtime.TypeID
	simpleSpecs       map[schemair.TypeID]schemair.SimpleTypeSpec
	typeKinds         map[schemair.TypeID]schemair.TypeKind
	complexPlans      map[schemair.TypeID]schemair.ComplexTypePlan
	validators        map[typeKey]runtime.ValidatorID
	compiling         map[typeKey]bool
	out               Artifacts
	enums             enumBuilder
	values            valueBuilder
	patterns          []runtime.Pattern
	facets            []runtime.FacetInstr
	bundle            runtime.ValidatorsBundle
}

func newArtifactCompiler(schema *schemair.Schema) (*artifactCompiler, error) {
	if schema == nil {
		return nil, fmt.Errorf("runtime build: schema ir is nil")
	}
	c := &artifactCompiler{
		schema:            schema,
		builtinSpecs:      make(map[string]schemair.SimpleTypeSpec, len(schema.BuiltinTypes)),
		builtinRuntimeIDs: make(map[string]runtime.TypeID, len(schema.BuiltinTypes)),
		simpleSpecs:       make(map[schemair.TypeID]schemair.SimpleTypeSpec, len(schema.SimpleTypes)),
		typeKinds:         make(map[schemair.TypeID]schemair.TypeKind, len(schema.Types)),
		complexPlans:      make(map[schemair.TypeID]schemair.ComplexTypePlan, len(schema.ComplexTypes)),
		validators:        make(map[typeKey]runtime.ValidatorID),
		compiling:         make(map[typeKey]bool),
		bundle: runtime.ValidatorsBundle{
			Meta: make([]runtime.ValidatorMeta, 1),
		},
	}
	for i, builtin := range schema.BuiltinTypes {
		c.builtinSpecs[builtin.Name.Local] = builtin.Value
		c.builtinRuntimeIDs[builtin.Name.Local] = runtime.TypeID(i + 1)
	}
	for _, spec := range schema.SimpleTypes {
		c.simpleSpecs[spec.TypeDecl] = spec
	}
	for _, typ := range schema.Types {
		c.typeKinds[typ.ID] = typ.Kind
	}
	for _, plan := range schema.ComplexTypes {
		c.complexPlans[plan.TypeDecl] = plan
	}
	c.out = Artifacts{
		TypeValidators:    make(map[schemair.TypeID]runtime.ValidatorID, len(schema.SimpleTypes)),
		BuiltinValidators: make(map[string]runtime.ValidatorID, len(schema.BuiltinTypes)),
		TextValidators:    make(map[schemair.TypeID]runtime.ValidatorID),
		ElementDefaults:   make(map[schemair.ElementID]DefaultFixedValue),
		ElementFixed:      make(map[schemair.ElementID]DefaultFixedValue),
		AttributeDefaults: make(map[schemair.AttributeID]DefaultFixedValue),
		AttributeFixed:    make(map[schemair.AttributeID]DefaultFixedValue),
		AttrUseDefaults:   make(map[schemair.AttributeUseID]DefaultFixedValue),
		AttrUseFixed:      make(map[schemair.AttributeUseID]DefaultFixedValue),
	}
	return c, nil
}

func (c *artifactCompiler) finish() *Artifacts {
	c.out.Validators = c.bundle
	c.out.Enums = c.enums.table()
	c.out.Facets = c.facets
	c.out.Patterns = c.patterns
	c.out.Values = c.values.table()
	return &c.out
}
