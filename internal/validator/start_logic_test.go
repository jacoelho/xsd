package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestResolveRootStrictUnknown(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.RootPolicy = runtime.RootStrict

	_, err := ResolveStartRoot(rt, 0, nsID)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateRootNotDeclared {
		t.Fatalf("ResolveStartRoot() error = %v, want %s", err, xsderrors.ErrValidateRootNotDeclared)
	}
}

func TestResolveMatchWildcardStrictUnresolved(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: runtime.PCStrict,
		},
	}

	_, err := ResolveStartMatch(rt, StartMatch{Kind: StartMatchWildcard, Wildcard: 1}, 0, nsID, []byte("urn:test"))
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateWildcardElemStrictUnresolved {
		t.Fatalf("ResolveStartMatch() error = %v, want %s", err, xsderrors.ErrValidateWildcardElemStrictUnresolved)
	}
}

func TestResolveChildNilled(t *testing.T) {
	t.Parallel()

	out, err := resolveStartChild(startChildInput{Nilled: true}, 1, 1, []byte("urn:test"), nil)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("resolveStartChild() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("resolveStartChild() ChildErrorReported = false, want true")
	}
}

func TestResolveChildDelegatesToStepper(t *testing.T) {
	t.Parallel()

	called := false
	out, err := resolveStartChild(
		startChildInput{Content: runtime.ContentElementOnly, Model: runtime.ModelRef{Kind: runtime.ModelDFA, ID: 7}},
		11,
		12,
		[]byte("urn:test"),
		func(ref runtime.ModelRef, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (StartMatch, error) {
			called = true
			if ref.ID != 7 || sym != 11 || nsID != 12 || string(ns) != "urn:test" {
				t.Fatalf("unexpected step inputs: ref=%+v sym=%d nsID=%d ns=%q", ref, sym, nsID, ns)
			}
			return StartMatch{Kind: StartMatchElem, Elem: 21}, nil
		},
	)
	if err != nil {
		t.Fatalf("resolveStartChild() error = %v", err)
	}
	if !called {
		t.Fatal("resolveStartChild() did not call stepper")
	}
	if out.ChildErrorReported {
		t.Fatal("resolveStartChild() ChildErrorReported = true, want false")
	}
	if out.Match.Kind != StartMatchElem || out.Match.Elem != 21 {
		t.Fatalf("resolveStartChild() match = %+v, want elem 21", out.Match)
	}
}

func TestPlanStartElementRootSkip(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.RootPolicy = runtime.RootAny

	sess := NewSession(rt)
	out, err := sess.planStartElement(startPlanInput{
		Entry: NameEntry{NS: nsID},
		Root:  true,
	})
	if err != nil {
		t.Fatalf("planStartElement() error = %v", err)
	}
	if !out.Skip {
		t.Fatal("planStartElement() skip = false, want true")
	}
}

func TestPlanStartElementChildPropagatesReportedError(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)
	sess := NewSession(rt)

	out, err := sess.planStartElement(startPlanInput{
		Entry: NameEntry{Sym: sym, NS: nsID},
		NS:    []byte("urn:test"),
		Parent: &elemFrame{
			nilled: true,
		},
	})
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("planStartElement() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("planStartElement() ChildErrorReported = false, want true")
	}
}

func buildSchema(t *testing.T) (*runtime.Schema, runtime.NamespaceID, runtime.SymbolID) {
	t.Helper()

	builder := runtime.NewBuilder()
	nsID, err := builder.InternNamespace([]byte("urn:test"))
	if err != nil {
		t.Fatalf("InternNamespace() error = %v", err)
	}
	sym, err := builder.InternSymbol(nsID, []byte("root"))
	if err != nil {
		t.Fatalf("InternSymbol() error = %v", err)
	}
	rt, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	rt.Elements = make([]runtime.Element, 2)
	rt.Elements[1] = runtime.Element{Name: sym}
	rt.GlobalElements = make([]runtime.ElemID, rt.Symbols.Count()+1)
	rt.GlobalElements[sym] = 1
	return rt, nsID, sym
}
