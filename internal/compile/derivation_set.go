package compile

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const derivationAll = "#all"

// ParseDerivationSet parses an XSD block/final derivation set and returns
// compile diagnostics for invalid lexical values.
func ParseDerivationSet(value, label string, allowed runtime.DerivationMask) (runtime.DerivationMask, error) {
	var mask runtime.DerivationMask
	seenAll := false
	for token := range lex.XMLFieldsSeq(value) {
		if token == derivationAll {
			if seenAll || mask != 0 {
				return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, label+" cannot combine #all with other values")
			}
			seenAll = true
			continue
		}
		if seenAll {
			return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, label+" cannot combine #all with other values")
		}
		bit, ok := compileDerivationToken(token)
		if !ok {
			return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid "+label+" value "+token)
		}
		if allowed&bit == 0 {
			return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, label+" cannot contain "+token)
		}
		mask |= bit
	}
	if seenAll {
		return allowed, nil
	}
	return mask, nil
}

func compileDerivationToken(token string) (runtime.DerivationMask, bool) {
	switch token {
	case vocab.XSDElemExtension:
		return runtime.DerivationExtension, true
	case vocab.XSDElemRestriction:
		return runtime.DerivationRestriction, true
	case "substitution":
		return runtime.DerivationSubstitution, true
	case vocab.XSDElemList:
		return runtime.DerivationList, true
	case vocab.XSDElemUnion:
		return runtime.DerivationUnion, true
	default:
		return 0, false
	}
}
