package validate

import "github.com/jacoelho/xsd/internal/runtime"

// IdentityNames resolves namespace IDs used by identity path wildcard tests.
type IdentityNames interface {
	Namespace(id runtime.NamespaceID) string
}

// IdentityRuntime exposes identity-constraint metadata needed during validation.
type IdentityRuntime interface {
	IdentityNames
	ForEachElementIdentityConstraint(id runtime.ElementID, fn func(runtime.IdentityConstraintID) bool)
	ForEachIdentitySelector(id runtime.IdentityConstraintID, fn func(runtime.IdentityPath) bool) bool
	IdentityFieldCount(id runtime.IdentityConstraintID) (int, bool)
	ForEachIdentityElementField(id runtime.IdentityConstraintID, fn func(runtime.CompiledIdentityField) bool) bool
	ForEachIdentityAttributeField(id runtime.IdentityConstraintID, name runtime.QName, fn func(runtime.CompiledIdentityField) bool) bool
	ForEachIdentityAttributeWildcardField(id runtime.IdentityConstraintID, fn func(runtime.CompiledIdentityField) bool) bool
	IdentityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool)
}

// IdentitySelectorMatches reports whether selector paths select the current element.
func IdentitySelectorMatches(names IdentityNames, namePath []runtime.RuntimeName, scopeDepth, currentDepth int, paths []runtime.IdentityPath) bool {
	for _, path := range paths {
		pattern := identityPathPattern{descendant: path.Descendant, self: path.Self, steps: path.Steps}
		if identityPathMatches(names, namePath, scopeDepth, currentDepth, pattern) {
			return true
		}
	}
	return false
}

// IdentityFieldPathsMatch reports whether field paths match the current element.
func IdentityFieldPathsMatch(names IdentityNames, namePath []runtime.RuntimeName, selectedDepth, currentDepth int, paths []runtime.IdentityFieldPath) bool {
	for _, path := range paths {
		if identityFieldPathMatches(names, namePath, selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

// IdentityAttributeFieldPathsMatch reports whether field paths match the current attribute.
func IdentityAttributeFieldPathsMatch(
	names IdentityNames,
	namePath []runtime.RuntimeName,
	selectedDepth, currentDepth int,
	name runtime.QName,
	paths []runtime.IdentityFieldPath,
) bool {
	for _, path := range paths {
		if identityFieldAttributeMatches(path, name) && identityFieldPathMatches(names, namePath, selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

type identityPathPattern struct {
	steps      []runtime.IdentityStep
	descendant bool
	self       bool
}

func identityPathMatches(names IdentityNames, namePath []runtime.RuntimeName, baseDepth, currentDepth int, path identityPathPattern) bool {
	if path.self {
		return currentDepth == baseDepth
	}
	if currentDepth < baseDepth || baseDepth < 0 || currentDepth > len(namePath) {
		return false
	}
	rel := namePath[baseDepth:currentDepth]
	if path.descendant {
		if len(rel) < len(path.steps) {
			return false
		}
		rel = rel[len(rel)-len(path.steps):]
	} else if len(rel) != len(path.steps) {
		return false
	}
	for i := range path.steps {
		if !identityStepMatches(names, rel[i], path.steps[i]) {
			return false
		}
	}
	return true
}

func identityStepMatches(names IdentityNames, rn runtime.RuntimeName, step runtime.IdentityStep) bool {
	if !step.Wildcard {
		return rn.Known && rn.Name == step.Name
	}
	if !step.NamespaceSet {
		return true
	}
	if rn.Known {
		return rn.Name.Namespace == step.Namespace
	}
	return rn.NS == identityNamespace(names, step.Namespace)
}

func identityFieldAttributeMatches(path runtime.IdentityFieldPath, name runtime.QName) bool {
	if !path.Attr {
		return false
	}
	if !path.AttrWildcard {
		return path.Attribute == name
	}
	return !path.AttrNamespaceSet || path.AttrNamespace == name.Namespace
}

func identityMatchExists(matches []IdentityFieldMatch, selection, field int) bool {
	for _, match := range matches {
		if match.Selection == selection && match.Field == field {
			return true
		}
	}
	return false
}

func identityFieldPathMatches(names IdentityNames, namePath []runtime.RuntimeName, selectedDepth, currentDepth int, path runtime.IdentityFieldPath) bool {
	if path.Self {
		return currentDepth == selectedDepth
	}
	pattern := identityPathPattern{descendant: path.Descendant, steps: path.Steps}
	return identityPathMatches(names, namePath, selectedDepth, currentDepth, pattern)
}

func identityNamespace(names IdentityNames, id runtime.NamespaceID) string {
	if names == nil {
		return ""
	}
	return names.Namespace(id)
}
