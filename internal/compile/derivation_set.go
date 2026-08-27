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
	parser := derivationSetParser{label: label, allowed: allowed}
	for token := range lex.XMLFieldsSeq(value) {
		if err := parser.add(token); err != nil {
			return 0, err
		}
	}
	if parser.seenAll {
		return allowed, nil
	}
	return parser.mask, nil
}

type derivationSetParser struct {
	label   string
	allowed runtime.DerivationMask
	mask    runtime.DerivationMask
	seenAll bool
}

func (p *derivationSetParser) add(token string) error {
	if token == derivationAll {
		return p.addAll()
	}
	if p.seenAll {
		return p.combinationError()
	}
	bit, ok := compileDerivationToken(token)
	if !ok {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid "+p.label+" value "+token)
	}
	if p.allowed&bit == 0 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, p.label+" cannot contain "+token)
	}
	p.mask |= bit
	return nil
}

func (p *derivationSetParser) addAll() error {
	if p.seenAll || p.mask != 0 {
		return p.combinationError()
	}
	p.seenAll = true
	return nil
}

func (p *derivationSetParser) combinationError() error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, p.label+" cannot combine #all with other values")
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
