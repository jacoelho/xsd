package pipeline

import (
	"fmt"
	"iter"
	"sync"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimeassemble"
	"github.com/jacoelho/xsd/internal/schemaanalysis"
	"github.com/jacoelho/xsd/internal/schemaprep"
)

type buildRuntimeFunc func(CompileConfig) (*runtime.Schema, error)

// PreparedSchema stores immutable runtime-build artifacts.
type PreparedSchema struct {
	build              buildRuntimeFunc
	globalElementOrder []model.QName
}

// CompileConfig configures runtime schema compilation from prepared artifacts.
type CompileConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// BuildRuntime compiles prepared artifacts into an immutable runtime schema.
func (p *PreparedSchema) BuildRuntime(cfg CompileConfig) (*runtime.Schema, error) {
	if p == nil || p.build == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return p.build(cfg)
}

// GlobalElementOrderSeq yields global element names in deterministic prepared order.
func (p *PreparedSchema) GlobalElementOrderSeq() iter.Seq[model.QName] {
	return func(yield func(model.QName) bool) {
		if p == nil || len(p.globalElementOrder) == 0 {
			return
		}
		for _, item := range p.globalElementOrder {
			if !yield(item) {
				return
			}
		}
	}
}

// Prepare validates and transforms a parsed schema for runtime compilation.
func Prepare(sch *parser.Schema) (*PreparedSchema, error) {
	validatedSchema, reg, refs, err := validateSchema(sch)
	if err != nil {
		return nil, err
	}
	return &PreparedSchema{
		build:              newBuildRuntimeFunc(validatedSchema, reg, refs),
		globalElementOrder: globalElementOrder(reg),
	}, nil
}

func validateSchema(sch *parser.Schema) (*parser.Schema, *schemaanalysis.Registry, *schemaanalysis.ResolvedReferences, error) {
	if sch == nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: schema is nil")
	}

	resolvedSchema, err := loadmerge.CloneSchemaDeep(sch)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: clone schema: %w", err)
	}
	if resolveErr := schemaprep.ResolveAndValidateOwned(resolvedSchema); resolveErr != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: %w", resolveErr)
	}
	reg, err := schemaanalysis.AssignIDs(resolvedSchema)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	if cycleErr := schemaanalysis.DetectCycles(resolvedSchema); cycleErr != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: detect cycles: %w", cycleErr)
	}
	if upaErr := schemaprep.ValidateUPA(resolvedSchema, reg); upaErr != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: validate UPA: %w", upaErr)
	}
	refs, err := schemaanalysis.ResolveReferences(resolvedSchema, reg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	return resolvedSchema, reg, refs, nil
}

func newBuildRuntimeFunc(sch *parser.Schema, reg *schemaanalysis.Registry, refs *schemaanalysis.ResolvedReferences) buildRuntimeFunc {
	var (
		once     sync.Once
		prepared *runtimeassemble.PreparedArtifacts
		prepErr  error
	)
	return func(cfg CompileConfig) (*runtime.Schema, error) {
		once.Do(func() {
			prepared, prepErr = runtimeassemble.PrepareBuildArtifacts(sch, reg, refs)
		})
		if prepErr != nil {
			return nil, prepErr
		}
		return prepared.Build(runtimeassemble.BuildConfig{
			MaxDFAStates:   cfg.MaxDFAStates,
			MaxOccursLimit: cfg.MaxOccursLimit,
		})
	}
}

func globalElementOrder(reg *schemaanalysis.Registry) []model.QName {
	if reg == nil || len(reg.ElementOrder) == 0 {
		return nil
	}
	order := make([]model.QName, 0, len(reg.ElementOrder))
	for _, entry := range reg.ElementOrder {
		if entry.Global {
			order = append(order, entry.QName)
		}
	}
	return order
}
