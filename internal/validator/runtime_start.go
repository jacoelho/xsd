package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// StartElement validates one start-element event and returns resolved runtime metadata.
func (s *Session) StartElement(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, inputAttrs []Start, resolver value.NSResolver) (StartResult, error) {
	if s == nil || s.rt == nil {
		return StartResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(inputAttrs, false)
	if err != nil {
		return StartResult{}, err
	}
	return ResolveStartResult(s.rt, match, sym, nsID, nsBytes, classified, resolver)
}
