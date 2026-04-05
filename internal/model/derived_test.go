package model

import "testing"

func TestNextDerivationStepBuiltinListAndUnion(t *testing.T) {
	list := GetBuiltin(TypeNameIDREFS)
	if list == nil {
		t.Fatal("builtin IDREFS missing")
	}
	next, method, err := NextDerivationStep(list, nil)
	if err != nil {
		t.Fatalf("NextDerivationStep(list) error = %v", err)
	}
	if method != DerivationList {
		t.Fatalf("list method = %v, want %v", method, DerivationList)
	}
	if next == nil || next.Name().Local != string(TypeNameAnySimpleType) {
		t.Fatalf("list next = %v, want xs:anySimpleType", next)
	}

	union := &SimpleType{
		QName: QName{Namespace: "urn:test", Local: "Union"},
		Union: &UnionType{},
	}
	next, method, err = NextDerivationStep(union, nil)
	if err != nil {
		t.Fatalf("NextDerivationStep(union) error = %v", err)
	}
	if method != DerivationUnion {
		t.Fatalf("union method = %v, want %v", method, DerivationUnion)
	}
	if next == nil || next.Name().Local != string(TypeNameAnySimpleType) {
		t.Fatalf("union next = %v, want xs:anySimpleType", next)
	}
}

func TestNextDerivationStepComplexAndSimpleResolvers(t *testing.T) {
	baseQName := QName{Namespace: "urn:test", Local: "Base"}
	base := &ComplexType{QName: baseQName}
	complexDerived := &ComplexType{
		QName:            QName{Namespace: "urn:test", Local: "Derived"},
		DerivationMethod: DerivationExtension,
	}
	complexDerived.SetContent(&ComplexContent{
		Extension: &Extension{Base: baseQName},
	})
	next, method, err := NextDerivationStep(complexDerived, func(name QName) (Type, error) {
		if name == baseQName {
			return base, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("NextDerivationStep(complex) error = %v", err)
	}
	if method != DerivationExtension {
		t.Fatalf("complex method = %v, want %v", method, DerivationExtension)
	}
	if next != base {
		t.Fatalf("complex next = %p, want %p", next, base)
	}

	simpleBaseQName := QName{Namespace: "urn:test", Local: "SimpleBase"}
	simpleBase := &SimpleType{QName: simpleBaseQName}
	simpleDerived := &SimpleType{
		QName: QName{Namespace: "urn:test", Local: "SimpleDerived"},
		Restriction: &Restriction{
			Base: simpleBaseQName,
		},
	}
	next, method, err = NextDerivationStep(simpleDerived, func(name QName) (Type, error) {
		if name == simpleBaseQName {
			return simpleBase, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("NextDerivationStep(simple) error = %v", err)
	}
	if method != DerivationRestriction {
		t.Fatalf("simple method = %v, want %v", method, DerivationRestriction)
	}
	if next != simpleBase {
		t.Fatalf("simple next = %p, want %p", next, simpleBase)
	}
}

func TestDerivationMaskAccumulatesAndStopsOnCycle(t *testing.T) {
	base := &ComplexType{QName: QName{Namespace: "urn:test", Local: "Base"}}
	mid := &ComplexType{
		QName:            QName{Namespace: "urn:test", Local: "Mid"},
		ResolvedBase:     base,
		DerivationMethod: DerivationRestriction,
	}
	leaf := &ComplexType{
		QName:            QName{Namespace: "urn:test", Local: "Leaf"},
		ResolvedBase:     mid,
		DerivationMethod: DerivationExtension,
	}
	mask, ok, err := DerivationMask(leaf, base, nil)
	if err != nil {
		t.Fatalf("DerivationMask() error = %v", err)
	}
	if !ok {
		t.Fatal("DerivationMask() ok = false, want true")
	}
	if want := DerivationExtension | DerivationRestriction; mask != want {
		t.Fatalf("mask = %v, want %v", mask, want)
	}

	cycleA := &ComplexType{QName: QName{Namespace: "urn:test", Local: "CycleA"}}
	cycleB := &ComplexType{
		QName:            QName{Namespace: "urn:test", Local: "CycleB"},
		ResolvedBase:     cycleA,
		DerivationMethod: DerivationExtension,
	}
	cycleA.ResolvedBase = cycleB
	mask, ok, err = DerivationMask(cycleA, base, nil)
	if err != nil {
		t.Fatalf("DerivationMask(cycle) error = %v", err)
	}
	if ok || mask != 0 {
		t.Fatalf("DerivationMask(cycle) = (%v, %v), want (0, false)", mask, ok)
	}
}

func TestBlockedDerivationsIncludesElementAndTypeMasks(t *testing.T) {
	headType := &ComplexType{}
	headType.Block = headType.Block.Add(DerivationExtension)
	headType.Final = headType.Final.Add(DerivationRestriction)
	head := &ElementDecl{Type: headType}
	head.Block = head.Block.Add(DerivationExtension)
	head.Final = head.Final.Add(DerivationUnion)

	mask := BlockedDerivations(head)
	want := DerivationExtension | DerivationRestriction | DerivationUnion
	if mask != want {
		t.Fatalf("BlockedDerivations() = %v, want %v", mask, want)
	}
}

func TestBlockedDerivationsNilHead(t *testing.T) {
	if mask := BlockedDerivations(nil); mask != 0 {
		t.Fatalf("BlockedDerivations(nil) = %v, want 0", mask)
	}
}
