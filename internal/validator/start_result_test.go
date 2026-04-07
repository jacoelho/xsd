package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestResolveResultRejectsAbstractElement(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)
	rt.Types = make([]runtime.Type, 2)
	rt.Elements[1] = runtime.Element{
		Name:  sym,
		Type:  1,
		Flags: runtime.ElemAbstract,
	}

	_, err := ResolveStartResult(rt, StartMatch{Kind: StartMatchElem, Elem: 1}, sym, nsID, []byte("urn:test"), Classification{}, nil)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrElementAbstract {
		t.Fatalf("ResolveStartResult() error = %v, want %s", err, xsderrors.ErrElementAbstract)
	}
}

func TestResolveResultRejectsNonNillableXsiNil(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)
	rt.Types = make([]runtime.Type, 2)
	rt.Types[1] = runtime.Type{Kind: runtime.TypeSimple}
	rt.Elements[1] = runtime.Element{
		Name: sym,
		Type: 1,
	}

	_, err := ResolveStartResult(
		rt,
		StartMatch{Kind: StartMatchElem, Elem: 1},
		sym,
		nsID,
		[]byte("urn:test"),
		Classification{XSINil: []byte("true")},
		nil,
	)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateXsiNilNotNillable {
		t.Fatalf("ResolveStartResult() error = %v, want %s", err, xsderrors.ErrValidateXsiNilNotNillable)
	}
}

func TestResolveResultRejectsAbstractType(t *testing.T) {
	t.Parallel()

	rt, nsID, sym := buildSchema(t)
	rt.Types = make([]runtime.Type, 2)
	rt.Types[1] = runtime.Type{
		Kind:  runtime.TypeSimple,
		Flags: runtime.TypeAbstract,
	}
	rt.Elements[1] = runtime.Element{
		Name: sym,
		Type: 1,
	}

	_, err := ResolveStartResult(rt, StartMatch{Kind: StartMatchElem, Elem: 1}, sym, nsID, []byte("urn:test"), Classification{}, nil)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrElementTypeAbstract {
		t.Fatalf("ResolveStartResult() error = %v, want %s", err, xsderrors.ErrElementTypeAbstract)
	}
}
