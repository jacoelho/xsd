package schemaast

import "fmt"

// ResolveSymbolByProcessContents applies strict/lax/skip symbol resolution policy.
func ResolveSymbolByProcessContents(pc ProcessContents, hasSymbol bool, resolve func() bool) (bool, bool, error) {
	switch pc {
	case Skip:
		return false, false, nil
	case Lax, Strict:
		if hasSymbol && resolve != nil && resolve() {
			return true, false, nil
		}
		if pc == Strict {
			return false, true, nil
		}
		return false, false, nil
	default:
		return false, false, fmt.Errorf("unknown wildcard processContents %d", pc)
	}
}

// RuntimeNamespaceConstraintKind enumerates runtime namespace constraint kind values.
type RuntimeNamespaceConstraintKind uint8

const (
	RuntimeNamespaceAny RuntimeNamespaceConstraintKind = iota
	RuntimeNamespaceOther
	RuntimeNamespaceEnumeration
	RuntimeNamespaceNotAbsent
)

// RuntimeNamespaceConstraint represents wildcard matching in lowered runtime form.
type RuntimeNamespaceConstraint struct {
	Kind      RuntimeNamespaceConstraintKind
	HasTarget bool
	HasLocal  bool
}

// AllowsRuntimeNamespace applies wildcard namespace policy in lowered runtime form.
func AllowsRuntimeNamespace(constraint RuntimeNamespaceConstraint, targetMatch, localMatch, enumMatch bool) bool {
	switch constraint.Kind {
	case RuntimeNamespaceAny:
		return true
	case RuntimeNamespaceOther:
		return !targetMatch && !localMatch
	case RuntimeNamespaceNotAbsent:
		return !localMatch
	case RuntimeNamespaceEnumeration:
		if constraint.HasLocal && localMatch {
			return true
		}
		if constraint.HasTarget && targetMatch {
			return true
		}
		return enumMatch
	default:
		return false
	}
}
