package start

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
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

	_, err := ResolveResult(rt, model.Match{Kind: model.MatchElem, Elem: 1}, sym, nsID, []byte("urn:test"), attrs.Classification{}, nil)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrElementAbstract {
		t.Fatalf("ResolveResult() error = %v, want %s", err, xsderrors.ErrElementAbstract)
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

	_, err := ResolveResult(
		rt,
		model.Match{Kind: model.MatchElem, Elem: 1},
		sym,
		nsID,
		[]byte("urn:test"),
		attrs.Classification{XSINil: []byte("true")},
		nil,
	)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrValidateXsiNilNotNillable {
		t.Fatalf("ResolveResult() error = %v, want %s", err, xsderrors.ErrValidateXsiNilNotNillable)
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

	_, err := ResolveResult(rt, model.Match{Kind: model.MatchElem, Elem: 1}, sym, nsID, []byte("urn:test"), attrs.Classification{}, nil)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrElementTypeAbstract {
		t.Fatalf("ResolveResult() error = %v, want %s", err, xsderrors.ErrElementTypeAbstract)
	}
}
