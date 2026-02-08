package pipeline

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimecompile"
	"github.com/jacoelho/xsd/internal/schemaflow"
	"github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

type buildRuntimeFunc func(CompileConfig) (*runtime.Schema, error)

// ValidatedSchema stores validated schema artifacts for transformation.
type ValidatedSchema struct {
	registry *semantic.Registry
	schema   *parser.Schema
}

// SchemaSnapshot returns a deep-copied resolved schema for inspection.
func (v *ValidatedSchema) SchemaSnapshot() (*parser.Schema, error) {
	if v == nil || v.schema == nil {
		return nil, fmt.Errorf("prepare schema: validated schema is nil")
	}
	return loadmerge.CloneSchemaDeep(v.schema)
}

// PreparedSchema stores immutable runtime-build artifacts.
type PreparedSchema struct {
	build              buildRuntimeFunc
	globalElementOrder []types.QName
}

// CompileConfig configures runtime schema compilation from prepared artifacts.
type CompileConfig struct {
	Limits         models.Limits
	MaxOccursLimit uint32
}

// BuildRuntime compiles prepared artifacts into an immutable runtime schema.
func (p *PreparedSchema) BuildRuntime(cfg CompileConfig) (*runtime.Schema, error) {
	if p == nil || p.build == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return p.build(cfg)
}

// GlobalElementOrder returns global element names in deterministic prepared order.
func (p *PreparedSchema) GlobalElementOrder() []types.QName {
	if p == nil || len(p.globalElementOrder) == 0 {
		return nil
	}
	order := make([]types.QName, len(p.globalElementOrder))
	copy(order, p.globalElementOrder)
	return order
}

// Prepare validates and transforms a parsed schema for runtime compilation.
func Prepare(sch *parser.Schema) (*PreparedSchema, error) {
	validated, err := Validate(sch)
	if err != nil {
		return nil, err
	}
	return Transform(validated)
}

// Validate runs schema semantic checks and returns immutable preparation artifacts.
func Validate(sch *parser.Schema) (*ValidatedSchema, error) {
	types.PrecomputeBuiltinCaches()
	validatedSchema, reg, err := validateSchema(sch)
	if err != nil {
		return nil, err
	}
	return &ValidatedSchema{
		registry: reg,
		schema:   validatedSchema,
	}, nil
}

// Transform compiles validated semantic artifacts into a prepared schema.
func Transform(validated *ValidatedSchema) (*PreparedSchema, error) {
	if validated == nil {
		return nil, fmt.Errorf("prepare schema: validated schema is nil")
	}
	return transformSchema(validated.schema, validated.registry)
}

func transformSchema(sch *parser.Schema, reg *semantic.Registry) (*PreparedSchema, error) {
	if sch == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if reg == nil {
		return nil, fmt.Errorf("prepare schema: registry is nil")
	}
	refs, err := semantic.ResolveReferences(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	return &PreparedSchema{
		build:              newBuildRuntimeFunc(sch, reg, refs),
		globalElementOrder: globalElementOrder(reg),
	}, nil
}

func validateSchema(sch *parser.Schema) (*parser.Schema, *semantic.Registry, error) {
	if sch == nil {
		return nil, nil, fmt.Errorf("prepare schema: schema is nil")
	}
	resolvedSchema, err := schemaflow.ResolveAndValidate(sch)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare schema: %w", err)
	}
	reg, err := semantic.AssignIDs(resolvedSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	if cycleErr := semantic.DetectCycles(resolvedSchema); cycleErr != nil {
		return nil, nil, fmt.Errorf("prepare schema: detect cycles: %w", cycleErr)
	}
	if upaErr := semantic.ValidateUPA(resolvedSchema, reg); upaErr != nil {
		return nil, nil, fmt.Errorf("prepare schema: validate UPA: %w", upaErr)
	}
	return resolvedSchema, reg, nil
}

func newBuildRuntimeFunc(sch *parser.Schema, reg *semantic.Registry, refs *semantic.ResolvedReferences) buildRuntimeFunc {
	return func(cfg CompileConfig) (*runtime.Schema, error) {
		if sch == nil {
			return nil, fmt.Errorf("runtime build: prepared schema is nil")
		}
		if reg == nil {
			return nil, fmt.Errorf("runtime build: prepared registry is nil")
		}
		if refs == nil {
			return nil, fmt.Errorf("runtime build: prepared references are nil")
		}
		return runtimecompile.BuildArtifacts(sch, reg, refs, runtimecompile.BuildConfig{
			Limits:         cfg.Limits,
			MaxOccursLimit: cfg.MaxOccursLimit,
		})
	}
}

func globalElementOrder(reg *semantic.Registry) []types.QName {
	if reg == nil || len(reg.ElementOrder) == 0 {
		return nil
	}
	order := make([]types.QName, 0, len(reg.ElementOrder))
	for _, entry := range reg.ElementOrder {
		if entry.Global {
			order = append(order, entry.QName)
		}
	}
	return order
}
