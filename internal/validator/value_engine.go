package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// TextValueOptions controls canonicalization and key-derivation behavior for text validation.
type TextValueOptions struct {
	RequireCanonical bool
	NeedKey          bool
}

// ValidateTextValue validates simple-content text and returns canonical bytes plus value metrics.
func (s *Session) ValidateTextValue(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions) ([]byte, ValueMetrics, error) {
	validated, err := newValueRunner(s).validateText(textValueRequest{
		Type:     typeID,
		Lexical:  text,
		Resolver: resolver,
		Options:  textOpts,
	})
	if err != nil {
		return nil, ValueMetrics{}, err
	}
	return validated.Canonical, validated.Metrics, nil
}

func hasLengthFacet(meta runtime.ValidatorMeta, facetCode []runtime.FacetInstr) bool {
	if meta.Facets.Len == 0 {
		return false
	}
	ok, err := RuntimeProgramHasOp(meta, facetCode, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}

func validateTemporalNoCanonical(kind runtime.ValidatorKind, normalized []byte) error {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return xsderrors.Invalidf("unsupported temporal kind %d", kind)
	}
	_, err := value.Parse(spec.Kind, normalized)
	return err
}

func validateAnyURINoCanonical(normalized []byte) error {
	return value.ValidateAnyURI(normalized)
}

func validateHexBinaryNoCanonical(normalized []byte) error {
	_, err := value.ParseHexBinary(normalized)
	return err
}

func validateBase64BinaryNoCanonical(normalized []byte) error {
	_, err := value.ParseBase64Binary(normalized)
	return err
}

func valueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WSReplace:
		return value.WhitespaceReplace
	case runtime.WSCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
