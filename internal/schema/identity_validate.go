package schema

import "errors"

// validateIdentityConstraints validates canonical identity-constraint metadata.
func validateIdentityConstraints(names *nameTable, identities []identityConstraintSpec) error {
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

func validateIdentityConstraintOwnership(elements []elementDecl, identityCount int) error {
	owned := make([]bool, identityCount)
	for _, element := range elements {
		if err := markOwnedIdentityConstraints(owned, element.Identity); err != nil {
			return err
		}
	}
	for _, attached := range owned {
		if !attached {
			return errors.New("identity constraint is not attached to an element declaration")
		}
	}
	return nil
}

func markOwnedIdentityConstraints(owned []bool, identities []IdentityConstraintID) error {
	for _, id := range identities {
		if !validIdentityConstraintID(id, len(owned)) {
			return errors.New("element declaration references invalid identity constraint")
		}
		if owned[id] {
			return errors.New("identity constraint is attached more than once")
		}
		owned[id] = true
	}
	return nil
}

func validateIdentityConstraint(names *nameTable, identities []identityConstraintSpec, ic identityConstraintSpec) error {
	if !names.ValidQName(ic.Name) {
		return errors.New("identity constraint references invalid name")
	}
	if len(ic.Selector) == 0 {
		return errors.New("identity constraint has no selector")
	}
	if len(ic.Fields) == 0 {
		return errors.New("identity constraint has no fields")
	}
	if err := validateIdentityConstraintKind(identities, ic); err != nil {
		return err
	}
	if err := validateIdentitySelectorPaths(names, ic.Selector); err != nil {
		return err
	}
	return validateIdentityFieldPaths(names, ic.Fields)
}

func validateIdentityConstraintKind(identities []identityConstraintSpec, ic identityConstraintSpec) error {
	switch ic.Kind {
	case IdentityUnique, IdentityKey:
		if ic.Refer != NoIdentityConstraint {
			return errors.New("non-keyref identity constraint stores refer")
		}
		return nil
	case IdentityKeyRef:
		return validateIdentityKeyRef(identities, ic)
	default:
		return errors.New("identity constraint has invalid kind")
	}
}

func validateIdentityKeyRef(identities []identityConstraintSpec, ic identityConstraintSpec) error {
	if !validIdentityConstraintID(ic.Refer, len(identities)) {
		return errors.New("keyref identity constraint references invalid key")
	}
	ref := identities[ic.Refer]
	if ref.Kind == IdentityKeyRef {
		return errors.New("keyref identity constraint references keyref")
	}
	if len(ic.Fields) != len(ref.Fields) {
		return errors.New("keyref identity constraint field count differs from refer")
	}
	return nil
}

func validateIdentitySelectorPaths(names *nameTable, paths []identityPath) error {
	for _, path := range paths {
		if !validIdentityPath(names, path) {
			return errors.New("identity selector references invalid name")
		}
	}
	return nil
}

func validateIdentityFieldPaths(names *nameTable, fields []identityField) error {
	for _, field := range fields {
		if len(field.Paths) == 0 {
			return errors.New("identity field has no paths")
		}
		if err := validateIdentityFieldPathSet(names, field.Paths); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentityFieldPathSet(names *nameTable, paths []identityFieldPath) error {
	for _, path := range paths {
		if !validIdentityFieldPath(names, path) {
			return errors.New("identity field path has invalid shape")
		}
	}
	return nil
}

func validIdentityPath(names *nameTable, path identityPath) bool {
	return validIdentitySteps(names, path.Steps)
}

func validIdentityFieldPath(names *nameTable, path identityFieldPath) bool {
	if !validIdentityAttributePath(names, path) {
		return false
	}
	return validIdentitySteps(names, path.Steps)
}

func validIdentityAttributePath(names *nameTable, path identityFieldPath) bool {
	if !path.Attr {
		return !path.AttrWildcard &&
			!path.AttrNamespaceSet &&
			path.Attribute == noQName() &&
			path.AttrNamespace == emptyNamespaceID
	}
	if !path.AttrWildcard {
		return !path.AttrNamespaceSet &&
			path.AttrNamespace == emptyNamespaceID &&
			names.ValidQName(path.Attribute)
	}
	if path.Attribute != noQName() {
		return false
	}
	if !path.AttrNamespaceSet {
		return path.AttrNamespace == emptyNamespaceID
	}
	return names.ValidNamespaceID(path.AttrNamespace)
}

func validIdentitySteps(names *nameTable, steps []IdentityStep) bool {
	for _, step := range steps {
		if !validIdentityStep(names, step) {
			return false
		}
	}
	return true
}

func validIdentityStep(names *nameTable, step IdentityStep) bool {
	if !step.Wildcard {
		return !step.NamespaceSet &&
			step.Namespace == emptyNamespaceID &&
			names.ValidQName(step.Name)
	}
	if step.Name != noQName() {
		return false
	}
	if !step.NamespaceSet {
		return step.Namespace == emptyNamespaceID
	}
	return names.ValidNamespaceID(step.Namespace)
}
