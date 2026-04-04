package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/validator/start"
	"github.com/jacoelho/xsd/internal/value"
)

// StartElement validates one start-element event and returns resolved runtime metadata.
func (s *Session) StartElement(match model.Match, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, inputAttrs []attrs.Start, resolver value.NSResolver) (start.Result, error) {
	if s == nil || s.rt == nil {
		return start.Result{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(inputAttrs, false)
	if err != nil {
		return start.Result{}, err
	}
	return s.resolveStartResult(match, sym, nsID, nsBytes, classified, resolver)
}
