package substpolicy

import (
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
)

func TestMethodLabel(t *testing.T) {
	if got := MethodLabel(0); got != "unknown" {
		t.Fatalf("MethodLabel(0) = %q, want unknown", got)
	}
}

func TestNextDerivationStep_BuiltinList(t *testing.T) {
	typ := builtins.Get(builtins.TypeNameIDREFS)
	if typ == nil {
		t.Fatal("builtin IDREFS missing")
	}
	next, method, err := NextDerivationStep(typ, nil)
	if err != nil {
		t.Fatalf("NextDerivationStep() error = %v", err)
	}
	if method != model.DerivationList {
		t.Fatalf("method = %v, want %v", method, model.DerivationList)
	}
	if next == nil || next.Name().Local != string(model.TypeNameAnySimpleType) {
		t.Fatalf("next = %v, want xs:anySimpleType", next)
	}
}

func TestNextDerivationStep_SimpleRestrictionUsesResolver(t *testing.T) {
	baseQName := model.QName{Namespace: "urn:test", Local: "Base"}
	base := &model.SimpleType{QName: baseQName}
	typ := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "Derived"},
		Restriction: &model.Restriction{
			Base: baseQName,
		},
	}
	next, method, err := NextDerivationStep(typ, func(name model.QName) (model.Type, error) {
		if name == baseQName {
			return base, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("NextDerivationStep() error = %v", err)
	}
	if method != model.DerivationRestriction {
		t.Fatalf("method = %v, want %v", method, model.DerivationRestriction)
	}
	if next != base {
		t.Fatalf("next = %p, want %p", next, base)
	}
}

func TestNextDerivationStep_ComplexUsesResolvedBase(t *testing.T) {
	base := &model.ComplexType{QName: model.QName{Namespace: "urn:test", Local: "Base"}}
	typ := &model.ComplexType{
		QName:            model.QName{Namespace: "urn:test", Local: "Derived"},
		ResolvedBase:     base,
		DerivationMethod: model.DerivationExtension,
	}
	next, method, err := NextDerivationStep(typ, nil)
	if err != nil {
		t.Fatalf("NextDerivationStep() error = %v", err)
	}
	if method != model.DerivationExtension {
		t.Fatalf("method = %v, want %v", method, model.DerivationExtension)
	}
	if next != base {
		t.Fatalf("next = %p, want %p", next, base)
	}
}
