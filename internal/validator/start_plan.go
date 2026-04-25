package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

type startPlanInput struct {
	Match                StartMatch
	Entry                NameEntry
	Local                []byte
	NS                   []byte
	Name                 NameID
	Resolver             value.NSResolver
	Parent               *elemFrame
	HasMatch             bool
	Root                 bool
	CheckAttrDuplicates  bool
	ValidateAttrs        bool
	TrackIdentityAttrs   bool
	StoreAttributeValues bool
	BuildFrame           bool
	Attrs                []Start
}

type startPlan struct {
	Attrs              AttrResult
	Frame              elemFrame
	Identity           identityStartInput
	Match              StartMatch
	Result             StartResult
	Skip               bool
	ChildErrorReported bool
}

func (s *Session) planStartElement(in startPlanInput) (startPlan, error) {
	if s == nil || s.rt == nil {
		return startPlan{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	classified, err := s.classifyAttrs(in.Attrs, in.CheckAttrDuplicates)
	if err != nil {
		return startPlan{}, err
	}

	plan := startPlan{}
	match, skip, childErrorReported, err := s.resolveStartMatch(in)
	if err != nil {
		plan.ChildErrorReported = childErrorReported
		return plan, err
	}
	if skip {
		plan.Skip = true
		return plan, nil
	}
	plan.Match = match
	plan.ChildErrorReported = childErrorReported

	result, err := ResolveStartResult(s.rt, match, in.Entry.Sym, in.Entry.NS, in.NS, classified, in.Resolver)
	if err != nil {
		return plan, err
	}
	plan.Result = result
	plan.Skip = result.Skip
	if plan.Skip {
		return plan, nil
	}

	if in.ValidateAttrs {
		storeIdentityAttrs := in.TrackIdentityAttrs && s.needsIdentityAttrs(result.Elem)
		attrResult, err := s.validateAttributesClassifiedWithStorage(result.Type, in.Attrs, in.Resolver, classified, storeIdentityAttrs, in.StoreAttributeValues)
		if err != nil {
			return plan, err
		}
		plan.Attrs = attrResult
	}

	if in.BuildFrame {
		frame, err := s.planStartFrame(in, result)
		if err != nil {
			return plan, err
		}
		plan.Frame = frame
	}

	plan.Identity = identityStartInput{
		Elem:    result.Elem,
		Type:    result.Type,
		Sym:     in.Entry.Sym,
		NS:      in.Entry.NS,
		Attrs:   plan.Attrs.Attrs,
		Applied: plan.Attrs.Applied,
		Nilled:  result.Nilled,
	}

	return plan, nil
}

func (s *Session) resolveStartMatch(in startPlanInput) (StartMatch, bool, bool, error) {
	if in.HasMatch {
		return in.Match, false, false, nil
	}
	if in.Root {
		decision, err := ResolveStartRoot(s.rt, in.Entry.Sym, in.Entry.NS)
		if err != nil {
			return StartMatch{}, false, false, err
		}
		if decision.Skip {
			return StartMatch{}, true, false, nil
		}
		return decision.Match, false, false, nil
	}
	if in.Parent == nil {
		return StartMatch{}, false, false, fmt.Errorf("parent frame missing")
	}
	child, err := resolveStartChild(
		startChildInput{
			Content: in.Parent.content,
			Model:   in.Parent.model,
			Nilled:  in.Parent.nilled,
		},
		in.Entry.Sym,
		in.Entry.NS,
		in.NS,
		func(ref runtime.ModelRef, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (StartMatch, error) {
			return s.StepModel(ref, &in.Parent.modelState, sym, nsID, ns)
		},
	)
	if err != nil {
		return StartMatch{}, false, child.ChildErrorReported, err
	}
	return child.Match, false, child.ChildErrorReported, nil
}

func (s *Session) planStartFrame(in startPlanInput, result StartResult) (elemFrame, error) {
	typ, ok := s.typeByID(result.Type)
	if !ok {
		return elemFrame{}, fmt.Errorf("type %d not found", result.Type)
	}
	plan, err := planStartFrame(
		startNameInput{
			Local:  in.Local,
			NS:     in.NS,
			Cached: in.Entry.LocalLen != 0 || in.Entry.NSLen != 0,
		},
		result,
		typ,
		s.rt.ComplexTypes,
	)
	if err != nil {
		return elemFrame{}, err
	}

	frame := elemFrame{
		local:   plan.Local,
		ns:      plan.NS,
		model:   plan.Model,
		name:    in.Name,
		elem:    result.Elem,
		typ:     result.Type,
		content: plan.Content,
		mixed:   plan.Mixed,
		nilled:  result.Nilled,
	}
	if frame.model.Kind != runtime.ModelNone {
		state, err := s.InitModelState(frame.model)
		if err != nil {
			return elemFrame{}, err
		}
		frame.modelState = state
	}
	return frame, nil
}
