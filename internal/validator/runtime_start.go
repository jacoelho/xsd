package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// StartElement is an exported function.
func (s *Session) StartElement(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, attrs []StartAttr, resolver value.NSResolver) (StartResult, error) {
	if s == nil || s.rt == nil {
		return StartResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(attrs, false)
	if err != nil {
		return StartResult{}, err
	}
	return s.startElementClassified(match, sym, nsID, nsBytes, resolver, classified)
}
