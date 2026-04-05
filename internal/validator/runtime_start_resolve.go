package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/validator/start"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) resolveStartEvent(in start.EventInput, resolver value.NSResolver, step start.StepModelFunc) (start.EventResult, error) {
	return start.ResolveEvent(s.rt, in, resolver, step)
}

func (s *Session) resolveStartResult(match model.Match, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, classified attrs.Classification, resolver value.NSResolver) (start.Result, error) {
	if s == nil || s.rt == nil {
		return start.Result{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	return start.ResolveResult(s.rt, match, sym, nsID, nsBytes, classified, resolver)
}
