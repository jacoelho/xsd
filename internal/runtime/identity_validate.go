package runtime

import (
	"errors"
	"slices"
)

// ValidateIdentityConstraints validates frozen identity-constraint metadata.
func ValidateIdentityConstraints(names *NameTable, identities []IdentityConstraint) error {
	if names == nil {
		return errors.New("identity constraints require name table")
	}
	for i := range identities {
		if err := validateIdentityConstraint(names, identities, identities[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentityConstraintOwnership(elements []ElementDecl, identityCount int) error {
	owned := make([]bool, identityCount)
	for _, element := range elements {
		for _, id := range element.Identity {
			if !ValidIdentityConstraintID(id, identityCount) {
				return errors.New("element declaration references invalid identity constraint")
			}
			if owned[id] {
				return errors.New("identity constraint is attached more than once")
			}
			owned[id] = true
		}
	}
	for _, attached := range owned {
		if !attached {
			return errors.New("identity constraint is not attached to an element declaration")
		}
	}
	return nil
}

// ValidateIdentityConstraintReadProjection validates identity-constraint read
// projections against frozen identity metadata.
func ValidateIdentityConstraintReadProjection(reads []IdentityConstraintRead, identities []IdentityConstraint) error {
	if len(reads) != len(identities) {
		return errors.New("identity constraint read projection count does not match constraints")
	}
	if !EqualIdentityConstraintReadProjection(reads, identities) {
		return errors.New("identity constraint read projection does not match constraints")
	}
	return nil
}

func equalIdentityPaths(a, b []IdentityPath) bool {
	return slices.EqualFunc(a, b, func(x, y IdentityPath) bool {
		return x.Descendant == y.Descendant &&
			x.Self == y.Self &&
			slices.Equal(x.Steps, y.Steps)
	})
}

func validateIdentityConstraint(names *NameTable, identities []IdentityConstraint, ic IdentityConstraint) error {
	if !names.ValidQName(ic.Name) {
		return errors.New("identity constraint references invalid name")
	}
	if len(ic.Selector) == 0 {
		return errors.New("identity constraint has no selector")
	}
	if len(ic.Fields) == 0 {
		return errors.New("identity constraint has no fields")
	}
	switch ic.Kind {
	case IdentityUnique, IdentityKey:
		if ic.Refer != NoIdentityConstraint {
			return errors.New("non-keyref identity constraint stores refer")
		}
	case IdentityKeyRef:
		if !ValidIdentityConstraintID(ic.Refer, len(identities)) {
			return errors.New("keyref identity constraint references invalid key")
		}
		ref := identities[ic.Refer]
		if ref.Kind == IdentityKeyRef {
			return errors.New("keyref identity constraint references keyref")
		}
		if len(ic.Fields) != len(ref.Fields) {
			return errors.New("keyref identity constraint field count differs from refer")
		}
	default:
		return errors.New("identity constraint has invalid kind")
	}
	for _, path := range ic.Selector {
		if !validIdentityPath(names, path) {
			return errors.New("identity selector references invalid name")
		}
	}
	for _, field := range ic.Fields {
		if len(field.Paths) == 0 {
			return errors.New("identity field has no paths")
		}
		for _, path := range field.Paths {
			if !validIdentityFieldPath(names, path) {
				return errors.New("identity field path has invalid shape")
			}
		}
	}
	elementFields, attrFields, attrWildcardFields := BuildIdentityFieldLookup(ic.Fields)
	if !equalCompiledIdentityFields(ic.ElementFields, elementFields) ||
		!equalCompiledIdentityFieldMaps(ic.AttributeFields, attrFields) ||
		!equalCompiledIdentityFields(ic.AttributeWildcardFields, attrWildcardFields) {
		return errors.New("identity constraint field lookup does not match fields")
	}
	return nil
}

func equalCompiledIdentityFields(a, b []CompiledIdentityField) bool {
	return slices.EqualFunc(a, b, func(x, y CompiledIdentityField) bool {
		return x.Field == y.Field && equalIdentityFieldPaths(x.Paths, y.Paths)
	})
}

func equalIdentityFieldPaths(a, b []IdentityFieldPath) bool {
	return slices.EqualFunc(a, b, func(x, y IdentityFieldPath) bool {
		return x.Attribute == y.Attribute &&
			x.AttrNamespace == y.AttrNamespace &&
			x.Descendant == y.Descendant &&
			x.Self == y.Self &&
			x.Attr == y.Attr &&
			x.AttrWildcard == y.AttrWildcard &&
			x.AttrNamespaceSet == y.AttrNamespaceSet &&
			slices.Equal(x.Steps, y.Steps)
	})
}

func equalCompiledIdentityFieldMaps(a, b map[QName][]CompiledIdentityField) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !equalCompiledIdentityFields(av, bv) {
			return false
		}
	}
	return true
}

func validIdentityPath(names *NameTable, path IdentityPath) bool {
	if path.Self {
		return !path.Descendant && len(path.Steps) == 0
	}
	return validIdentitySteps(names, path.Steps)
}

func validIdentityFieldPath(names *NameTable, path IdentityFieldPath) bool {
	if path.Self {
		return !path.Descendant &&
			len(path.Steps) == 0 &&
			!path.Attr &&
			!path.AttrWildcard &&
			!path.AttrNamespaceSet &&
			path.Attribute == NoQName() &&
			path.AttrNamespace == EmptyNamespaceID
	}
	if !validIdentityAttributePath(names, path) {
		return false
	}
	return validIdentitySteps(names, path.Steps)
}

func validIdentityAttributePath(names *NameTable, path IdentityFieldPath) bool {
	if !path.Attr {
		return !path.AttrWildcard &&
			!path.AttrNamespaceSet &&
			path.Attribute == NoQName() &&
			path.AttrNamespace == EmptyNamespaceID
	}
	if !path.AttrWildcard {
		return !path.AttrNamespaceSet &&
			path.AttrNamespace == EmptyNamespaceID &&
			names.ValidQName(path.Attribute)
	}
	if path.Attribute != NoQName() {
		return false
	}
	if !path.AttrNamespaceSet {
		return path.AttrNamespace == EmptyNamespaceID
	}
	return names.ValidNamespaceID(path.AttrNamespace)
}

func validIdentitySteps(names *NameTable, steps []IdentityStep) bool {
	for _, step := range steps {
		if !validIdentityStep(names, step) {
			return false
		}
	}
	return true
}

func validIdentityStep(names *NameTable, step IdentityStep) bool {
	if !step.Wildcard {
		return !step.NamespaceSet &&
			step.Namespace == EmptyNamespaceID &&
			names.ValidQName(step.Name)
	}
	if step.Name != NoQName() {
		return false
	}
	if !step.NamespaceSet {
		return step.Namespace == EmptyNamespaceID
	}
	return names.ValidNamespaceID(step.Namespace)
}
