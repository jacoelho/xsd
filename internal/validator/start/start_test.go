package start

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
)

func TestResolveRootStrictUnknown(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.RootPolicy = runtime.RootStrict

	_, err := ResolveRoot(rt, 0, nsID)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrValidateRootNotDeclared {
		t.Fatalf("ResolveRoot() error = %v, want %s", err, xsderrors.ErrValidateRootNotDeclared)
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

	_, err := ResolveMatch(rt, model.Match{Kind: model.MatchWildcard, Wildcard: 1}, 0, nsID, []byte("urn:test"))
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrValidateWildcardElemStrictUnresolved {
		t.Fatalf("ResolveMatch() error = %v, want %s", err, xsderrors.ErrValidateWildcardElemStrictUnresolved)
	}
}

func TestResolveChildNilled(t *testing.T) {
	t.Parallel()

	out, err := ResolveChild(ChildInput{Nilled: true}, 1, 1, []byte("urn:test"), nil)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("ResolveChild() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("ResolveChild() ChildErrorReported = false, want true")
	}
}

func TestResolveChildDelegatesToStepper(t *testing.T) {
	t.Parallel()

	called := false
	out, err := ResolveChild(
		ChildInput{Content: runtime.ContentElementOnly, Model: runtime.ModelRef{Kind: runtime.ModelDFA, ID: 7}},
		11,
		12,
		[]byte("urn:test"),
		func(ref runtime.ModelRef, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (model.Match, error) {
			called = true
			if ref.ID != 7 || sym != 11 || nsID != 12 || string(ns) != "urn:test" {
				t.Fatalf("unexpected step inputs: ref=%+v sym=%d nsID=%d ns=%q", ref, sym, nsID, ns)
			}
			return model.Match{Kind: model.MatchElem, Elem: 21}, nil
		},
	)
	if err != nil {
		t.Fatalf("ResolveChild() error = %v", err)
	}
	if !called {
		t.Fatal("ResolveChild() did not call stepper")
	}
	if out.ChildErrorReported {
		t.Fatal("ResolveChild() ChildErrorReported = true, want false")
	}
	if out.Match.Kind != model.MatchElem || out.Match.Elem != 21 {
		t.Fatalf("ResolveChild() match = %+v, want elem 21", out.Match)
	}
}

func TestResolveEventRootSkip(t *testing.T) {
	t.Parallel()

	rt, nsID, _ := buildSchema(t)
	rt.RootPolicy = runtime.RootAny

	out, err := ResolveEvent(rt, EventInput{Root: true, NSID: nsID}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveEvent() error = %v", err)
	}
	if !out.Result.Skip {
		t.Fatal("ResolveEvent() skip = false, want true")
	}
}

func TestResolveEventChildPropagatesReportedError(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)

	out, err := ResolveEvent(rt, EventInput{
		Sym:  sym,
		NSID: nsID,
		NS:   []byte("urn:test"),
		Parent: ChildInput{
			Nilled: true,
		},
	}, nil, nil)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrValidateNilledNotEmpty {
		t.Fatalf("ResolveEvent() error = %v, want %s", err, xsderrors.ErrValidateNilledNotEmpty)
	}
	if !out.ChildErrorReported {
		t.Fatal("ResolveEvent() ChildErrorReported = false, want true")
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
