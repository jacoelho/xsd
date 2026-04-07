package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
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

	out, err := ResolveStartChild(StartChildInput{Nilled: true}, 1, 1, []byte("urn:test"), nil)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("ResolveStartChild() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("ResolveStartChild() ChildErrorReported = false, want true")
	}
}

func TestResolveChildDelegatesToStepper(t *testing.T) {
	t.Parallel()

	called := false
	out, err := ResolveStartChild(
		StartChildInput{Content: runtime.ContentElementOnly, Model: runtime.ModelRef{Kind: runtime.ModelDFA, ID: 7}},
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
		t.Fatalf("ResolveStartChild() error = %v", err)
	}
	if !called {
		t.Fatal("ResolveStartChild() did not call stepper")
	}
	if out.ChildErrorReported {
		t.Fatal("ResolveStartChild() ChildErrorReported = true, want false")
	}
	if out.Match.Kind != StartMatchElem || out.Match.Elem != 21 {
		t.Fatalf("ResolveStartChild() match = %+v, want elem 21", out.Match)
	}
}

func TestResolveEventRootSkip(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.RootPolicy = runtime.RootAny

	out, err := ResolveStartEvent(rt, StartEventInput{Root: true, NSID: nsID}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveStartEvent() error = %v", err)
	}
	if !out.Result.Skip {
		t.Fatal("ResolveStartEvent() skip = false, want true")
	}
}

func TestResolveEventChildPropagatesReportedError(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)

	out, err := ResolveStartEvent(rt, StartEventInput{
		Sym:  sym,
		NSID: nsID,
		NS:   []byte("urn:test"),
		Parent: StartChildInput{
			Nilled: true,
		},
	}, nil, nil)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("ResolveStartEvent() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("ResolveStartEvent() ChildErrorReported = false, want true")
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
