package runtimecompile

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

// BuildConfig configures runtime schema compilation.
type BuildConfig struct {
	Limits         models.Limits
	MaxOccursLimit uint32
}

// BuildSchema compiles a parsed schema into a runtime schema model.
func BuildSchema(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	prepared, err := pipeline.Prepare(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	return BuildPrepared(prepared, cfg)
}

// BuildPrepared compiles a prepared schema into a runtime schema model.
func BuildPrepared(prepared *pipeline.PreparedSchema, cfg BuildConfig) (*runtime.Schema, error) {
	if prepared == nil || prepared.Schema == nil {
		return nil, fmt.Errorf("runtime build: prepared schema is nil")
	}
	if prepared.Registry == nil {
		return nil, fmt.Errorf("runtime build: prepared registry is nil")
	}
	if prepared.Refs == nil {
		return nil, fmt.Errorf("runtime build: prepared references are nil")
	}
	sch := prepared.Schema
	reg := prepared.Registry
	refs := prepared.Refs

	validators, err := CompileValidators(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	maxOccursLimit := cfg.MaxOccursLimit
	if maxOccursLimit == 0 {
		maxOccursLimit = defaultMaxOccursLimit
	}

	builder := &schemaBuilder{
		schema:     sch,
		registry:   reg,
		refs:       refs,
		validators: validators,
		limits:     cfg.Limits,
		builder:    runtime.NewBuilder(),
		typeIDs:    make(map[schema.TypeID]runtime.TypeID),
		elemIDs:    make(map[schema.ElemID]runtime.ElemID),
		attrIDs:    make(map[schema.AttrID]runtime.AttrID),
		builtinIDs: make(map[types.TypeName]runtime.TypeID),
		complexIDs: make(map[runtime.TypeID]uint32),
		maxOccurs:  maxOccursLimit,
	}
	rt, err := builder.build()
	if err != nil {
		return nil, err
	}
	sch.Phase = parser.PhaseRuntimeReady
	return rt, nil
}

type schemaBuilder struct {
	err             error
	attrIDs         map[schema.AttrID]runtime.AttrID
	elemIDs         map[schema.ElemID]runtime.ElemID
	validators      *CompiledValidators
	registry        *schema.Registry
	typeIDs         map[schema.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	builtinIDs      map[types.TypeName]runtime.TypeID
	refs            *schema.ResolvedReferences
	anyElementRules map[*types.AnyElement]runtime.WildcardID
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

func (b *schemaBuilder) build() (*runtime.Schema, error) {
	if err := b.initSymbols(); err != nil {
		return nil, err
	}
	if b.err != nil {
		return nil, b.err
	}
	rt, err := b.builder.Build()
	if err != nil {
		return nil, err
	}
	b.rt = rt
	b.rt.RootPolicy = runtime.RootStrict
	b.rt.Validators = b.validators.Validators
	b.rt.Facets = b.validators.Facets
	b.rt.Patterns = b.validators.Patterns
	b.rt.Enums = b.validators.Enums
	b.rt.Values = b.validators.Values
	b.rt.Notations = b.notations
	b.wildcards = make([]runtime.WildcardRule, 1)

	b.initIDs()
	if err := b.buildTypes(); err != nil {
		return nil, err
	}
	if err := b.buildAncestors(); err != nil {
		return nil, err
	}
	if err := b.buildAttributes(); err != nil {
		return nil, err
	}
	if err := b.buildElements(); err != nil {
		return nil, err
	}
	if err := b.buildModels(); err != nil {
		return nil, err
	}
	if err := b.buildIdentityConstraints(); err != nil {
		return nil, err
	}

	b.rt.Wildcards = b.wildcards
	b.rt.WildcardNS = b.wildcardNS
	b.rt.Paths = b.paths

	b.rt.BuildHash = computeBuildHash(b.rt)

	return b.rt, nil
}

func (b *schemaBuilder) buildTypes() error {
	xsdNS := types.XSDNamespace
	nextComplex := uint32(1)
	for _, name := range builtinTypeNames() {
		id := b.builtinIDs[name]
		sym := b.internQName(types.QName{Namespace: xsdNS, Local: string(name)})
		typ := runtime.Type{Name: sym}
		if builtin := types.GetBuiltin(name); builtin != nil {
			base := builtin.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: builtin type %s base %s not found", name, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = runtime.DerRestriction
			}
		}
		if name == types.TypeNameAnyType {
			typ.Kind = runtime.TypeComplex
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.anyTypeComplex = nextComplex
			nextComplex++
		} else {
			typ.Kind = runtime.TypeBuiltin
			typ.Validator = b.validatorForBuiltin(name)
		}
		b.rt.Types[id] = typ
		b.rt.GlobalTypes[sym] = id
		if name == types.TypeNameAnyType {
			b.rt.Builtin.AnyType = id
		}
		if name == types.TypeNameAnySimpleType {
			b.rt.Builtin.AnySimpleType = id
		}
	}

	for _, entry := range b.registry.TypeOrder {
		id := b.typeIDs[entry.ID]
		var sym runtime.SymbolID
		if !entry.QName.IsZero() {
			sym = b.internQName(entry.QName)
		} else if entry.Global {
			return fmt.Errorf("runtime build: global type %d missing name", entry.ID)
		}
		typ := runtime.Type{Name: sym}
		switch t := entry.Type.(type) {
		case *types.SimpleType:
			typ.Kind = runtime.TypeSimple
			if vid, ok := b.validators.TypeValidators[entry.ID]; ok {
				typ.Validator = vid
			} else if vid, ok := b.validators.ValidatorForType(t); ok {
				typ.Validator = vid
			}
			base, method := b.baseForSimpleType(t)
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = method
			}
			typ.Final = toRuntimeDerivationSet(t.Final)
		case *types.ComplexType:
			typ.Kind = runtime.TypeComplex
			if t.Abstract {
				typ.Flags |= runtime.TypeAbstract
			}
			base := t.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
			}
			method := t.DerivationMethod
			if method == 0 {
				method = types.DerivationRestriction
			}
			typ.Derivation = toRuntimeDerivation(method)
			typ.Final = toRuntimeDerivationSet(t.Final)
			typ.Block = toRuntimeDerivationSet(t.Block)
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.complexIDs[id] = nextComplex
			nextComplex++
		default:
			return fmt.Errorf("runtime build: unsupported type %T", entry.Type)
		}

		b.rt.Types[id] = typ
		if entry.Global {
			b.rt.GlobalTypes[sym] = id
		}
	}
	return nil
}
