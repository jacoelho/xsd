package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// ParseDerivationSet parses an XSD block/final derivation set and returns
// compile diagnostics for invalid lexical values.
func ParseDerivationSet(value, label string, allowed runtime.DerivationMask) (runtime.DerivationMask, error) {
	mask, issue := runtime.ParseDerivationSet(value, allowed)
	switch issue.Kind {
	case runtime.DerivationSetOK:
		return mask, nil
	case runtime.DerivationSetAllCombination:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, label+" cannot combine #all with other values")
	case runtime.DerivationSetDisallowedToken:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, label+" cannot contain "+issue.Token)
	case runtime.DerivationSetInvalidToken:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid "+label+" value "+issue.Token)
	default:
		return 0, xsderrors.InternalInvariant("unknown derivation set issue")
	}
}
