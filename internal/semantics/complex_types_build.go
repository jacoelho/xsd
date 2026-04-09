package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func buildComplexTypes(schema *parser.Schema, registry *analysis.Registry, cache map[*model.ComplexType]model.Type) (*complexplan.ComplexTypes, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if cache == nil {
		cache = make(map[*model.ComplexType]model.Type)
	}
	plan, err := complexplan.Build(registry, complexplan.BuildFuncs{
		AttributeUses: func(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
			return CollectAttributeUses(schema, ct)
		},
		ContentParticle: func(ct *model.ComplexType) model.Particle {
			return EffectiveContentParticle(schema, ct)
		},
		SimpleContentType: func(ct *model.ComplexType) (model.Type, error) {
			return resolveSimpleContentTextTypeForCompiler(schema, cache, ct)
		},
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func resolveSimpleContentTextTypeForCompiler(schema *parser.Schema, cache map[*model.ComplexType]model.Type, ct *model.ComplexType) (model.Type, error) {
	return ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{
		ResolveQName: func(name model.QName) model.Type {
			if name.IsZero() {
				return nil
			}
			if builtin := model.GetBuiltinNS(name.Namespace, name.Local); builtin != nil {
				return builtin
			}
			if schema == nil {
				return nil
			}
			return schema.TypeDefs[name]
		},
		Cache: cache,
	})
}
