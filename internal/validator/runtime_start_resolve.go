package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/validator/start"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) resolveStartEvent(in start.EventInput, resolver value.NSResolver, step start.StepModelFunc) (start.EventResult, error) {
	var match model.Match
	childErrorReported := false

	if in.Root {
		decision, err := start.ResolveRoot(s.rt, in.Sym, in.NSID)
		if err != nil {
			return start.EventResult{}, err
		}
		if decision.Skip {
			return start.EventResult{Result: start.Result{Skip: true}}, nil
		}
		match = decision.Match
	} else {
		child, err := start.ResolveChild(in.Parent, in.Sym, in.NSID, in.NS, step)
		if err != nil {
			return start.EventResult{ChildErrorReported: child.ChildErrorReported}, err
		}
		match = child.Match
		childErrorReported = child.ChildErrorReported
	}

	result, err := s.resolveStartResult(match, in.Sym, in.NSID, in.NS, in.Attrs, resolver)
	if err != nil {
		return start.EventResult{ChildErrorReported: childErrorReported}, err
	}
	return start.EventResult{
		Match:              match,
		Result:             result,
		ChildErrorReported: childErrorReported,
	}, nil
}

func (s *Session) resolveStartResult(match model.Match, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, classified attrs.Classification, resolver value.NSResolver) (start.Result, error) {
	if s == nil || s.rt == nil {
		return start.Result{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	decl, err := start.ResolveMatch(s.rt, match, sym, nsID, nsBytes)
	if err != nil {
		return start.Result{}, err
	}
	if decl == 0 {
		return start.Result{Skip: true}, nil
	}
	if int(decl) >= len(s.rt.Elements) {
		return start.Result{}, fmt.Errorf("element %d out of range", decl)
	}

	elem := s.rt.Elements[decl]
	if elem.Flags&runtime.ElemAbstract != 0 {
		return start.Result{}, diag.New(xsderrors.ErrElementAbstract, "element is abstract")
	}

	actualType := elem.Type
	if len(classified.XSIType) > 0 {
		resolved, err := start.ResolveXSIType(s.rt, classified.XSIType, resolver)
		if err != nil {
			return start.Result{}, diag.New(xsderrors.ErrValidateXsiTypeUnresolved, err.Error())
		}
		if err := start.CheckTypeDerivation(s.rt, resolved, actualType, elem.Block); err != nil {
			return start.Result{}, diag.New(xsderrors.ErrValidateXsiTypeDerivationBlocked, err.Error())
		}
		actualType = resolved
	}

	nilled := false
	if len(classified.XSINil) > 0 {
		flag, err := value.ParseBoolean(classified.XSINil)
		if err != nil {
			return start.Result{}, diag.New(xsderrors.ErrDatatypeInvalid, fmt.Sprintf("invalid xsi:nil: %v", err))
		}
		if flag {
			if elem.Flags&runtime.ElemNillable == 0 {
				return start.Result{}, diag.New(xsderrors.ErrValidateXsiNilNotNillable, "element is not nillable")
			}
			if elem.Fixed.Present {
				return start.Result{}, diag.New(xsderrors.ErrValidateNilledHasFixed, "element has fixed value and cannot be nilled")
			}
			nilled = true
		}
	}

	if int(actualType) < len(s.rt.Types) && s.rt.Types[actualType].Flags&runtime.TypeAbstract != 0 {
		return start.Result{}, diag.New(xsderrors.ErrElementTypeAbstract, "type is abstract")
	}

	return start.Result{Elem: decl, Type: actualType, Nilled: nilled}, nil
}
