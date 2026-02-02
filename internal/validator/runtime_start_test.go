package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type mapResolver map[string]string

func (m mapResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	key := string(prefix)
	if prefix == nil {
		key = ""
	}
	if uri, ok := m[key]; ok {
		return []byte(uri), true
	}
	return nil, false
}

func TestStartElementXsiTypeRetarget(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	sess := NewSession(schema)

	attrs := []StartAttr{{
		Sym:   schema.Predef.XsiType,
		Value: []byte("t:Derived"),
	}}
	result, err := sess.StartElement(StartMatch{Kind: MatchElem, Elem: ids.elemBase}, ids.elemSym, ids.nsID, []byte("urn:test"), attrs, mapResolver{"t": "urn:test"})
	if err != nil {
		t.Fatalf("StartElement: %v", err)
	}
	if result.Type != ids.typeDerived {
		t.Fatalf("type = %d, want %d", result.Type, ids.typeDerived)
	}
}

func TestStartElementXsiTypeBlocked(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Elements[ids.elemBase].Block = runtime.ElemBlockExtension
	sess := NewSession(schema)

	attrs := []StartAttr{{
		Sym:   schema.Predef.XsiType,
		Value: []byte("t:Derived"),
	}}
	_, err := sess.StartElement(StartMatch{Kind: MatchElem, Elem: ids.elemBase}, ids.elemSym, ids.nsID, []byte("urn:test"), attrs, mapResolver{"t": "urn:test"})
	if err == nil {
		t.Fatalf("expected derivation blocked error")
	}
}

func TestStartElementXsiNil(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Elements[ids.elemBase].Flags |= runtime.ElemNillable
	sess := NewSession(schema)

	attrs := []StartAttr{{
		Sym:   schema.Predef.XsiNil,
		Value: []byte("true"),
	}}
	result, err := sess.StartElement(StartMatch{Kind: MatchElem, Elem: ids.elemBase}, ids.elemSym, ids.nsID, []byte("urn:test"), attrs, nil)
	if err != nil {
		t.Fatalf("StartElement: %v", err)
	}
	if !result.Nilled {
		t.Fatalf("expected nilled element")
	}
}

func TestStartElementXsiNilNotAllowed(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	sess := NewSession(schema)

	attrs := []StartAttr{{
		Sym:   schema.Predef.XsiNil,
		Value: []byte("1"),
	}}
	_, err := sess.StartElement(StartMatch{Kind: MatchElem, Elem: ids.elemBase}, ids.elemSym, ids.nsID, []byte("urn:test"), attrs, nil)
	if err == nil {
		t.Fatalf("expected nillable error")
	}
}

func TestStartElementAbstract(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Elements[ids.elemBase].Flags |= runtime.ElemAbstract
	sess := NewSession(schema)

	_, err := sess.StartElement(StartMatch{Kind: MatchElem, Elem: ids.elemBase}, ids.elemSym, ids.nsID, []byte("urn:test"), nil, nil)
	if err == nil {
		t.Fatalf("expected abstract element error")
	}
}

func TestStartElementWildcardStrictUnresolved(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: runtime.PCStrict,
		},
	}
	sess := NewSession(schema)

	_, err := sess.StartElement(StartMatch{Kind: MatchWildcard, Wildcard: 1}, 0, ids.nsID, []byte("urn:test"), nil, nil)
	if err == nil {
		t.Fatalf("expected strict wildcard error")
	}
}

func TestStartElementWildcardLaxSkip(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: runtime.PCLax,
		},
	}
	sess := NewSession(schema)

	result, err := sess.StartElement(StartMatch{Kind: MatchWildcard, Wildcard: 1}, 0, ids.nsID, []byte("urn:test"), nil, nil)
	if err != nil {
		t.Fatalf("StartElement: %v", err)
	}
	if !result.Skip {
		t.Fatalf("expected skip for lax wildcard unresolved")
	}
}

func TestStartElementWildcardResolvesGlobal(t *testing.T) {
	schema, ids := buildRuntimeFixture(t)
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: runtime.PCLax,
		},
	}
	sess := NewSession(schema)

	result, err := sess.StartElement(StartMatch{Kind: MatchWildcard, Wildcard: 1}, ids.elemSym, ids.nsID, []byte("urn:test"), nil, nil)
	if err != nil {
		t.Fatalf("StartElement: %v", err)
	}
	if result.Elem != ids.elemBase {
		t.Fatalf("elem = %d, want %d", result.Elem, ids.elemBase)
	}
}

func buildRuntimeFixture(tb testing.TB) (*runtime.Schema, fixtureIDs) {
	tb.Helper()
	builder := runtime.NewBuilder()
	nsID := builder.InternNamespace([]byte("urn:test"))
	elemSym := builder.InternSymbol(nsID, []byte("Base"))
	baseSym := builder.InternSymbol(nsID, []byte("BaseType"))
	derivedSym := builder.InternSymbol(nsID, []byte("Derived"))
	schema, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build() error = %v", err)
	}

	schema.Types = make([]runtime.Type, 3)
	schema.Types[1] = runtime.Type{Name: baseSym, Kind: runtime.TypeComplex}
	schema.Types[2] = runtime.Type{Name: derivedSym, Kind: runtime.TypeComplex, Base: 1, Derivation: runtime.DerExtension}
	schema.Ancestors = runtime.TypeAncestors{
		IDs:   []runtime.TypeID{1},
		Masks: []runtime.DerivationMethod{runtime.DerExtension},
	}
	schema.Types[2].AncOff = 0
	schema.Types[2].AncLen = 1
	schema.Types[2].AncMaskOff = 0
	schema.GlobalTypes = make([]runtime.TypeID, schema.Symbols.Count()+1)
	schema.GlobalTypes[baseSym] = 1
	schema.GlobalTypes[derivedSym] = 2

	schema.Elements = make([]runtime.Element, 2)
	schema.Elements[1] = runtime.Element{Name: elemSym, Type: 1}
	schema.GlobalElements = make([]runtime.ElemID, schema.Symbols.Count()+1)
	schema.GlobalElements[elemSym] = 1

	return schema, fixtureIDs{
		nsID:        nsID,
		elemSym:     elemSym,
		elemBase:    1,
		typeBase:    1,
		typeDerived: 2,
	}
}

type fixtureIDs struct {
	nsID        runtime.NamespaceID
	elemSym     runtime.SymbolID
	elemBase    runtime.ElemID
	typeBase    runtime.TypeID
	typeDerived runtime.TypeID
}

func TestResolveXsiTypeUsesResolver(t *testing.T) {
	schema, _ := buildRuntimeFixture(t)
	sess := NewSession(schema)

	_, err := sess.resolveXsiType([]byte("p:Derived"), mapResolver{"p": "urn:test"})
	if err != nil {
		t.Fatalf("resolveXsiType: %v", err)
	}
	_, err = sess.resolveXsiType([]byte("p:Derived"), mapResolver{"p": "urn:missing"})
	if err == nil {
		t.Fatalf("expected namespace lookup error")
	}

	_, err = sess.resolveXsiType([]byte("Bad QName"), mapResolver{"p": "urn:test"})
	if err == nil {
		t.Fatalf("expected QName parse error")
	}

	_, err = sess.resolveXsiType([]byte("Derived"), mapResolver{"": "urn:test"})
	if err != nil {
		t.Fatalf("resolveXsiType default: %v", err)
	}

	_, err = sess.resolveXsiType([]byte("Derived"), mapResolver{})
	if err == nil {
		t.Fatalf("expected missing default namespace error")
	}
}
