package typechain

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestLookupTypeAndComplexType(t *testing.T) {
	schema := parser.NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "T"}
	ct := model.NewComplexType(name, "urn:test")
	schema.TypeDefs[name] = ct

	gotType, ok := LookupType(schema, name)
	if !ok || gotType != ct {
		t.Fatalf("LookupType() = (%v, %v), want (%v, true)", gotType, ok, ct)
	}
	gotCT, ok := LookupComplexType(schema, name)
	if !ok || gotCT != ct {
		t.Fatalf("LookupComplexType() = (%v, %v), want (%v, true)", gotCT, ok, ct)
	}
}

func TestIsAnyTypeQName(t *testing.T) {
	if !model.IsAnyTypeQName(model.QName{Namespace: model.XSDNamespace, Local: string(model.TypeNameAnyType)}) {
		t.Fatalf("model.IsAnyTypeQName() = false, want true")
	}
	if model.IsAnyTypeQName(model.QName{Namespace: "urn:test", Local: "anyType"}) {
		t.Fatalf("model.IsAnyTypeQName() = true, want false")
	}
}

func TestCollectComplexTypeChainExplicitBaseOnly(t *testing.T) {
	schema := parser.NewSchema()
	baseName := model.QName{Namespace: "urn:test", Local: "Base"}
	derivedName := model.QName{Namespace: "urn:test", Local: "Derived"}

	base := model.NewComplexType(baseName, baseName.Namespace)
	base.SetContent(&model.EmptyContent{})
	derived := model.NewComplexType(derivedName, derivedName.Namespace)
	derived.SetContent(&model.ComplexContent{
		Extension: &model.Extension{Base: baseName},
	})

	schema.TypeDefs[baseName] = base
	schema.TypeDefs[derivedName] = derived

	chain := CollectComplexTypeChain(schema, derived, ComplexTypeChainExplicitBaseOnly)
	if len(chain) != 2 || chain[0] != derived || chain[1] != base {
		t.Fatalf("CollectComplexTypeChain() unexpected chain: %#v", chain)
	}
}

func TestCollectComplexTypeChainAllowImplicitAnyType(t *testing.T) {
	ct := model.NewComplexType(model.QName{Namespace: "urn:test", Local: "LocalType"}, "urn:test")
	ct.SetContent(&model.EmptyContent{})

	chain := CollectComplexTypeChain(nil, ct, ComplexTypeChainAllowImplicitAnyType)
	if len(chain) != 2 {
		t.Fatalf("CollectComplexTypeChain() len = %d, want 2", len(chain))
	}
	if chain[0] != ct {
		t.Fatalf("CollectComplexTypeChain()[0] mismatch")
	}
	if !model.IsAnyTypeQName(chain[1].QName) {
		t.Fatalf("CollectComplexTypeChain()[1] = %s, want xs:anyType", chain[1].QName)
	}
}

func TestEffectiveContentParticle_Extension(t *testing.T) {
	schema := parser.NewSchema()
	baseName := model.QName{Namespace: "urn:test", Local: "Base"}
	derivedName := model.QName{Namespace: "urn:test", Local: "Derived"}

	baseParticle := &model.ElementDecl{
		Name:      model.QName{Namespace: "urn:test", Local: "a"},
		MinOccurs: model.OccursFromInt(1),
		MaxOccurs: model.OccursFromInt(1),
	}
	extParticle := &model.ElementDecl{
		Name:      model.QName{Namespace: "urn:test", Local: "b"},
		MinOccurs: model.OccursFromInt(1),
		MaxOccurs: model.OccursFromInt(1),
	}

	base := model.NewComplexType(baseName, baseName.Namespace)
	base.SetContent(&model.ElementContent{Particle: baseParticle})
	derived := model.NewComplexType(derivedName, derivedName.Namespace)
	derived.SetContent(&model.ComplexContent{
		Extension: &model.Extension{
			Base:     baseName,
			Particle: extParticle,
		},
	})

	schema.TypeDefs[baseName] = base
	schema.TypeDefs[derivedName] = derived

	got := EffectiveContentParticle(schema, derived)
	mg, ok := got.(*model.ModelGroup)
	if !ok {
		t.Fatalf("EffectiveContentParticle() type = %T, want *model.ModelGroup", got)
	}
	if mg.Kind != model.Sequence || len(mg.Particles) != 2 {
		t.Fatalf("EffectiveContentParticle() model group mismatch: %#v", mg)
	}
	if mg.Particles[0] != baseParticle || mg.Particles[1] != extParticle {
		t.Fatalf("EffectiveContentParticle() particles mismatch")
	}
}
