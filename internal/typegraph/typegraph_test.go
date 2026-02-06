package typegraph

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestLookupTypeAndComplexType(t *testing.T) {
	schema := parser.NewSchema()
	name := types.QName{Namespace: "urn:test", Local: "T"}
	ct := types.NewComplexType(name, "urn:test")
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
	if !IsAnyTypeQName(types.QName{Namespace: types.XSDNamespace, Local: string(types.TypeNameAnyType)}) {
		t.Fatalf("IsAnyTypeQName() = false, want true")
	}
	if IsAnyTypeQName(types.QName{Namespace: "urn:test", Local: "anyType"}) {
		t.Fatalf("IsAnyTypeQName() = true, want false")
	}
}

func TestCollectComplexTypeChain(t *testing.T) {
	schema := parser.NewSchema()
	baseName := types.QName{Namespace: "urn:test", Local: "Base"}
	derivedName := types.QName{Namespace: "urn:test", Local: "Derived"}

	base := types.NewComplexType(baseName, baseName.Namespace)
	base.SetContent(&types.EmptyContent{})
	derived := types.NewComplexType(derivedName, derivedName.Namespace)
	derived.SetContent(&types.ComplexContent{
		Extension: &types.Extension{Base: baseName},
	})

	schema.TypeDefs[baseName] = base
	schema.TypeDefs[derivedName] = derived

	chain := CollectComplexTypeChain(schema, derived)
	if len(chain) != 2 || chain[0] != derived || chain[1] != base {
		t.Fatalf("CollectComplexTypeChain() unexpected chain: %#v", chain)
	}
}

func TestCollectComplexTypeChainWithImplicitAnyType(t *testing.T) {
	ct := types.NewComplexType(types.QName{Namespace: "urn:test", Local: "LocalType"}, "urn:test")
	ct.SetContent(&types.EmptyContent{})

	chain := CollectComplexTypeChainWithImplicitAnyType(nil, ct)
	if len(chain) != 2 {
		t.Fatalf("CollectComplexTypeChainWithImplicitAnyType() len = %d, want 2", len(chain))
	}
	if chain[0] != ct {
		t.Fatalf("CollectComplexTypeChainWithImplicitAnyType()[0] mismatch")
	}
	if !IsAnyTypeQName(chain[1].QName) {
		t.Fatalf("CollectComplexTypeChainWithImplicitAnyType()[1] = %s, want xs:anyType", chain[1].QName)
	}
}

func TestEffectiveContentParticle_Extension(t *testing.T) {
	schema := parser.NewSchema()
	baseName := types.QName{Namespace: "urn:test", Local: "Base"}
	derivedName := types.QName{Namespace: "urn:test", Local: "Derived"}

	baseParticle := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:test", Local: "a"},
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}
	extParticle := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:test", Local: "b"},
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}

	base := types.NewComplexType(baseName, baseName.Namespace)
	base.SetContent(&types.ElementContent{Particle: baseParticle})
	derived := types.NewComplexType(derivedName, derivedName.Namespace)
	derived.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base:     baseName,
			Particle: extParticle,
		},
	})

	schema.TypeDefs[baseName] = base
	schema.TypeDefs[derivedName] = derived

	got := EffectiveContentParticle(schema, derived)
	mg, ok := got.(*types.ModelGroup)
	if !ok {
		t.Fatalf("EffectiveContentParticle() type = %T, want *types.ModelGroup", got)
	}
	if mg.Kind != types.Sequence || len(mg.Particles) != 2 {
		t.Fatalf("EffectiveContentParticle() model group mismatch: %#v", mg)
	}
	if mg.Particles[0] != baseParticle || mg.Particles[1] != extParticle {
		t.Fatalf("EffectiveContentParticle() particles mismatch")
	}
}
