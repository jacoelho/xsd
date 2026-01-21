package validator

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *streamRun) resolveXsiType(scopeDepth int, xsiTypeValue string, declaredType *grammar.CompiledType, elemBlock types.DerivationSet) (*grammar.CompiledType, error) {
	xsiTypeQName, err := r.parseQNameValue(xsiTypeValue, scopeDepth)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", strings.TrimSpace(xsiTypeValue), err)
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

	if len(declaredType.MemberTypes) > 0 {
		if r.isUnionMemberType(xsiType, declaredType) {
			return xsiType, nil
		}
		return nil, fmt.Errorf("type '%s' is not a member type of union '%s'",
			xsiTypeQName.String(), declaredType.QName.Local)
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
	xsiTypeQName, err := r.parseQNameValue(xsiTypeValue, scopeDepth)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", strings.TrimSpace(xsiTypeValue), err)
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
