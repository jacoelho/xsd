package validate

import "github.com/jacoelho/xsd/internal/runtime"

// IdentityNames resolves namespace IDs used by identity path wildcard tests.
type IdentityNames interface {
	Namespace(id runtime.NamespaceID) string
}

// identityScopeRuntime supplies the constraints declared on an element.
type identityScopeRuntime interface {
	ElementIdentityConstraints(id runtime.ElementID) (runtime.IdentityConstraintIDs, bool)
}

type identitySelectorPathRuntime interface {
	IdentityNames
	IdentitySelectorPaths(id runtime.IdentityConstraintID) (runtime.IdentityPathReads, bool)
}

// identitySelectorRuntime supplies selector paths and field counts.
type identitySelectorRuntime interface {
	identitySelectorPathRuntime
	IdentityFieldCount(id runtime.IdentityConstraintID) (int, bool)
}

// identityElementFieldRuntime supplies element field paths and namespace names.
type identityElementFieldRuntime interface {
	IdentityNames
	IdentityElementFields(id runtime.IdentityConstraintID) (runtime.CompiledIdentityFieldReads, bool)
}

// identityAttributeFieldRuntime supplies attribute field paths and namespace names.
type identityAttributeFieldRuntime interface {
	IdentityNames
	IdentityAttributeFields(id runtime.IdentityConstraintID, name runtime.QName) (runtime.CompiledIdentityFieldReads, bool)
	IdentityAttributeWildcardFields(id runtime.IdentityConstraintID) (runtime.CompiledIdentityFieldReads, bool)
}

func identityCompiledFieldPathsMatch(
	names IdentityNames,
	namePath []runtime.RuntimeName,
	selectedDepth, currentDepth int,
	field runtime.CompiledIdentityFieldRead,
) bool {
	for i := range field.PathCount() {
		path, ok := field.Path(i)
		if ok && identityFieldPathMatches(names, namePath, selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

func identityCompiledAttributeFieldPathsMatch(
	names IdentityNames,
	namePath []runtime.RuntimeName,
	selectedDepth, currentDepth int,
	name runtime.QName,
	field runtime.CompiledIdentityFieldRead,
) bool {
	for i := range field.PathCount() {
		path, ok := field.Path(i)
		if ok && identityFieldAttributeMatches(path, name) &&
			identityFieldPathMatches(names, namePath, selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

type identityStepPath interface {
	StepCount() int
	Step(index int) (runtime.IdentityStep, bool)
	Descendant() bool
	Self() bool
}

func identityPathMatches[Path identityStepPath](names IdentityNames, namePath []runtime.RuntimeName, baseDepth, currentDepth int, path Path) bool {
	if path.Self() {
		return currentDepth == baseDepth
	}
	if currentDepth < baseDepth || baseDepth < 0 || currentDepth > len(namePath) {
		return false
	}
	rel := namePath[baseDepth:currentDepth]
	stepCount := path.StepCount()
	if path.Descendant() {
		if len(rel) < stepCount {
			return false
		}
		rel = rel[len(rel)-stepCount:]
	} else if len(rel) != stepCount {
		return false
	}
	for i := range stepCount {
		step, ok := path.Step(i)
		if !ok || !identityStepMatches(names, rel[i], step) {
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

func identityFieldAttributeMatches(path runtime.IdentityFieldPathRead, name runtime.QName) bool {
	if !path.IsAttribute() {
		return false
	}
	if !path.AttributeWildcard() {
		return path.Attribute() == name
	}
	return !path.AttributeNamespaceSet() || path.AttributeNamespace() == name.Namespace
}

func identityMatchExists(matches []IdentityFieldMatch, selection, field int) bool {
	for _, match := range matches {
		if match.Selection == selection && match.Field == field {
			return true
		}
	}
	return false
}

func identityFieldPathMatches(names IdentityNames, namePath []runtime.RuntimeName, selectedDepth, currentDepth int, path runtime.IdentityFieldPathRead) bool {
	return identityPathMatches(names, namePath, selectedDepth, currentDepth, path)
}

func identityNamespace(names IdentityNames, id runtime.NamespaceID) string {
	if names == nil {
		return ""
	}
	return names.Namespace(id)
}
