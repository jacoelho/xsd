package wildcardpolicy

import "fmt"

// NamespaceTargetPlaceholder marks a namespace token that resolves to targetNamespace.
const NamespaceTargetPlaceholder = "##targetNamespace"

type NamespaceConstraintKind uint8

const (
	NamespaceAny NamespaceConstraintKind = iota
	NamespaceOther
	NamespaceTargetNamespace
	NamespaceLocal
	NamespaceList
	NamespaceNotAbsent
)

type NamespaceConstraint struct {
	TargetNS string
	List     []string
	Kind     NamespaceConstraintKind
}

// AllowsNamespace reports whether the namespace is permitted by a model wildcard constraint.
func AllowsNamespace(constraint NamespaceConstraintKind, list []string, targetNS, ns string) bool {
	switch constraint {
	case NamespaceAny:
		return true
	case NamespaceLocal:
		return ns == ""
	case NamespaceTargetNamespace:
		return ns == targetNS
	case NamespaceOther:
		return ns != targetNS && ns != ""
	case NamespaceNotAbsent:
		return ns != ""
	case NamespaceList:
		for _, allowed := range list {
			if resolveNamespaceToken(allowed, targetNS) == ns {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func resolveNamespaceToken(ns, targetNS string) string {
	if ns == NamespaceTargetPlaceholder {
		return targetNS
	}
	return ns
}

// NamespaceConstraintSubset reports whether derived is a subset of base.
func NamespaceConstraintSubset(derived, base NamespaceConstraint) bool {
	switch derived.Kind {
	case NamespaceAny:
		return base.Kind == NamespaceAny
	case NamespaceOther:
		if base.Kind == NamespaceAny {
			return true
		}
		if base.Kind == NamespaceOther && derived.TargetNS == base.TargetNS {
			return true
		}
		if base.Kind == NamespaceNotAbsent {
			return true
		}
		return false
	case NamespaceNotAbsent:
		switch base.Kind {
		case NamespaceAny, NamespaceNotAbsent:
			return true
		case NamespaceOther:
			return base.TargetNS == ""
		default:
			return false
		}
	case NamespaceTargetNamespace:
		return AllowsNamespace(base.Kind, base.List, base.TargetNS, derived.TargetNS)
	case NamespaceLocal:
		return AllowsNamespace(base.Kind, base.List, base.TargetNS, "")
	case NamespaceList:
		for _, ns := range derived.List {
			if !AllowsNamespace(base.Kind, base.List, base.TargetNS, resolveNamespaceToken(ns, derived.TargetNS)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type ProcessContents uint8

const (
	ProcessStrict ProcessContents = iota
	ProcessLax
	ProcessSkip
)

// ProcessContentsStrongerOrEqual reports whether derived is as strict as base.
func ProcessContentsStrongerOrEqual(derived, base ProcessContents) bool {
	switch base {
	case ProcessStrict:
		return derived == ProcessStrict
	case ProcessLax:
		return derived == ProcessLax || derived == ProcessStrict
	case ProcessSkip:
		return true
	default:
		return false
	}
}

// ResolveSymbolByProcessContents applies strict/lax/skip symbol resolution policy.
func ResolveSymbolByProcessContents(pc ProcessContents, hasSymbol bool, resolve func() bool) (bool, bool, error) {
	switch pc {
	case ProcessSkip:
		return false, false, nil
	case ProcessLax, ProcessStrict:
		if hasSymbol && resolve != nil && resolve() {
			return true, false, nil
		}
		if pc == ProcessStrict {
			return false, true, nil
		}
		return false, false, nil
	default:
		return false, false, fmt.Errorf("unknown wildcard processContents %d", pc)
	}
}

type RuntimeNamespaceConstraintKind uint8

const (
	RuntimeNamespaceAny RuntimeNamespaceConstraintKind = iota
	RuntimeNamespaceOther
	RuntimeNamespaceEnumeration
	RuntimeNamespaceNotAbsent
)

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
