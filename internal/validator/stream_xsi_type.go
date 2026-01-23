package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *streamRun) resolveXsiType(scopeDepth int, xsiTypeValue string, declaredType *grammar.CompiledType, elemBlock types.DerivationSet) (*grammar.CompiledType, error) {
	normalized := types.NormalizeWhiteSpace(xsiTypeValue, types.GetBuiltin(types.TypeName("QName")))
	xsiTypeQName, err := r.parseQNameValue(normalized, scopeDepth)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", types.TrimXMLWhitespace(xsiTypeValue), err)
	}

	xsiType := r.lookupType(xsiTypeQName)
	if xsiType == nil {
		return nil, fmt.Errorf("type '%s' not found in schema", xsiTypeQName.String())
	}

	if xsiType.Abstract {
		return nil, fmt.Errorf("type '%s' is abstract and cannot be used in xsi:type", xsiTypeQName.String())
	}

	if xsiType.QName == declaredType.QName {
		return xsiType, nil
	}

	if !r.typeDerivesFrom(xsiType, declaredType) {
		return nil, fmt.Errorf("type '%s' is not derived from '%s'",
			xsiTypeQName.String(), declaredType.QName.Local)
	}

	combinedBlock := elemBlock.Add(types.DerivationMethod(declaredType.Block))
	if blocked, method := r.isTypeSubstitutionBlocked(xsiType, declaredType, combinedBlock); blocked {
		return nil, fmt.Errorf("type '%s' cannot substitute '%s': %s derivation is blocked",
			xsiTypeQName.String(), declaredType.QName.Local, method)
	}

	return xsiType, nil
}

func (r *streamRun) resolveXsiTypeOnly(scopeDepth int, xsiTypeValue string) (*grammar.CompiledType, error) {
	normalized := types.NormalizeWhiteSpace(xsiTypeValue, types.GetBuiltin(types.TypeName("QName")))
	xsiTypeQName, err := r.parseQNameValue(normalized, scopeDepth)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", types.TrimXMLWhitespace(xsiTypeValue), err)
	}

	xsiType := r.lookupType(xsiTypeQName)
	if xsiType == nil {
		return nil, fmt.Errorf("type '%s' not found in schema", xsiTypeQName.String())
	}

	if xsiType.Abstract {
		return nil, fmt.Errorf("type '%s' is abstract and cannot be used in xsi:type", xsiTypeQName.String())
	}

	return xsiType, nil
}
