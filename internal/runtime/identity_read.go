package runtime

// IdentityConstraintInfo is the runtime metadata needed to finish a selected
// identity tuple.
type IdentityConstraintInfo struct {
	Refer IdentityConstraintID
	Kind  IdentityKind
}

// IdentityConstraintRead exposes validation-facing identity-constraint
// behavior without exposing raw compiled identity metadata.
type IdentityConstraintRead struct {
	attributeFields         map[QName][]CompiledIdentityField
	selector                []IdentityPath
	elementFields           []CompiledIdentityField
	attributeWildcardFields []CompiledIdentityField
	refer                   IdentityConstraintID
	kind                    IdentityKind
	fieldCount              int
}

// NewIdentityConstraintRead returns an immutable validation read projection for
// one identity constraint.
func NewIdentityConstraintRead(identity IdentityConstraint) IdentityConstraintRead {
	return IdentityConstraintRead{
		selector:                CloneIdentityPaths(identity.Selector),
		elementFields:           cloneCompiledIdentityFields(identity.ElementFields),
		attributeFields:         cloneCompiledIdentityFieldMap(identity.AttributeFields),
		attributeWildcardFields: cloneCompiledIdentityFields(identity.AttributeWildcardFields),
		refer:                   identity.Refer,
		kind:                    identity.Kind,
		fieldCount:              len(identity.Fields),
	}
}

// NewIdentityConstraintReads returns immutable validation read projections for
// identity constraints.
func NewIdentityConstraintReads(identities []IdentityConstraint) []IdentityConstraintRead {
	out := make([]IdentityConstraintRead, len(identities))
	for i := range identities {
		out[i] = NewIdentityConstraintRead(identities[i])
	}
	return out
}

func moveIdentityConstraintReads(identities []IdentityConstraint) []IdentityConstraintRead {
	out := make([]IdentityConstraintRead, len(identities))
	for i := range identities {
		identity := &identities[i]
		out[i] = IdentityConstraintRead{
			selector:                identity.Selector,
			elementFields:           identity.ElementFields,
			attributeFields:         identity.AttributeFields,
			attributeWildcardFields: identity.AttributeWildcardFields,
			refer:                   identity.Refer,
			kind:                    identity.Kind,
			fieldCount:              len(identity.Fields),
		}
	}
	return out
}

// ForEachElementIdentityConstraint visits the identity constraints attached to
// an element until fn returns false.
func ForEachElementIdentityConstraint(reads [][]IdentityConstraintID, id ElementID, fn func(IdentityConstraintID) bool) {
	if !ValidElementID(id, len(reads)) {
		return
	}
	for _, constraint := range reads[id] {
		if !fn(constraint) {
			return
		}
	}
}

// IdentityConstraintReadByID returns the validation read projection for id.
func IdentityConstraintReadByID(reads []IdentityConstraintRead, id IdentityConstraintID) (IdentityConstraintRead, bool) {
	if !ValidIdentityConstraintID(id, len(reads)) {
		return IdentityConstraintRead{}, false
	}
	return reads[id], true
}

func identityConstraintReadByIDPtr(reads []IdentityConstraintRead, id IdentityConstraintID) (*IdentityConstraintRead, bool) {
	if !ValidIdentityConstraintID(id, len(reads)) {
		return nil, false
	}
	return &reads[id], true
}

// IdentitySelectorPaths returns selector path reads for id.
func IdentitySelectorPaths(reads []IdentityConstraintRead, id IdentityConstraintID) ([]IdentityPath, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return nil, false
	}
	return ic.selector, true
}

// ForEachIdentitySelector visits selector paths for id until fn returns false.
func ForEachIdentitySelector(reads []IdentityConstraintRead, id IdentityConstraintID, fn func(IdentityPath) bool) bool {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return false
	}
	ic.ForEachSelector(fn)
	return true
}

// IdentityFieldCount returns the field count for id.
func IdentityFieldCount(reads []IdentityConstraintRead, id IdentityConstraintID) (int, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return 0, false
	}
	return ic.FieldCount(), true
}

// IdentityElementFields returns element-field reads for id.
func IdentityElementFields(reads []IdentityConstraintRead, id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return nil, false
	}
	return ic.elementFields, true
}

// ForEachIdentityElementField visits element fields for id until fn returns
// false.
func ForEachIdentityElementField(reads []IdentityConstraintRead, id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return false
	}
	ic.ForEachElementField(fn)
	return true
}

// IdentityAttributeFields returns matching attribute-field reads for id and
// name.
func IdentityAttributeFields(reads []IdentityConstraintRead, id IdentityConstraintID, name QName) ([]CompiledIdentityField, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return nil, false
	}
	return ic.attributeFields[name], true
}

// ForEachIdentityAttributeField visits attribute fields for id and name until
// fn returns false.
func ForEachIdentityAttributeField(reads []IdentityConstraintRead, id IdentityConstraintID, name QName, fn func(CompiledIdentityField) bool) bool {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return false
	}
	ic.ForEachAttributeField(name, fn)
	return true
}

// IdentityAttributeWildcardFields returns wildcard attribute-field reads for
// id.
func IdentityAttributeWildcardFields(reads []IdentityConstraintRead, id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return nil, false
	}
	return ic.attributeWildcardFields, true
}

// ForEachIdentityAttributeWildcardField visits attribute wildcard fields for id
// until fn returns false.
func ForEachIdentityAttributeWildcardField(reads []IdentityConstraintRead, id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return false
	}
	ic.ForEachAttributeWildcardField(fn)
	return true
}

// IdentityConstraintInfoByID returns the validation metadata for id.
func IdentityConstraintInfoByID(reads []IdentityConstraintRead, id IdentityConstraintID) (IdentityConstraintInfo, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return IdentityConstraintInfo{}, false
	}
	return IdentityConstraintInfo{
		Refer: ic.Refer(),
		Kind:  ic.Kind(),
	}, true
}

// EqualIdentityConstraintReadProjection reports whether reads expose the same
// validation-facing identity metadata as identities.
func EqualIdentityConstraintReadProjection(reads []IdentityConstraintRead, identities []IdentityConstraint) bool {
	if len(reads) != len(identities) {
		return false
	}
	for i, read := range reads {
		if !EqualIdentityConstraintRead(read, identities[i]) {
			return false
		}
	}
	return true
}

// EqualIdentityConstraintRead reports whether read exposes the validation
// projection for identity.
func EqualIdentityConstraintRead(read IdentityConstraintRead, identity IdentityConstraint) bool {
	return read.refer == identity.Refer &&
		read.kind == identity.Kind &&
		read.fieldCount == len(identity.Fields) &&
		equalIdentityPaths(read.selector, identity.Selector) &&
		equalCompiledIdentityFields(read.elementFields, identity.ElementFields) &&
		equalCompiledIdentityFieldMaps(read.attributeFields, identity.AttributeFields) &&
		equalCompiledIdentityFields(read.attributeWildcardFields, identity.AttributeWildcardFields)
}

// ForEachSelector visits each selector path until fn returns false.
func (r IdentityConstraintRead) ForEachSelector(fn func(IdentityPath) bool) {
	for _, path := range r.selector {
		if !fn(path) {
			return
		}
	}
}

// FieldCount returns the declared identity field count.
func (r IdentityConstraintRead) FieldCount() int {
	return r.fieldCount
}

// ForEachElementField visits each element-field lookup until fn returns false.
func (r IdentityConstraintRead) ForEachElementField(fn func(CompiledIdentityField) bool) {
	for _, field := range r.elementFields {
		if !fn(field) {
			return
		}
	}
}

// ForEachAttributeField visits each matching attribute-field lookup until fn
// returns false.
func (r IdentityConstraintRead) ForEachAttributeField(name QName, fn func(CompiledIdentityField) bool) {
	for _, field := range r.attributeFields[name] {
		if !fn(field) {
			return
		}
	}
}

// ForEachAttributeWildcardField visits each attribute-wildcard-field lookup
// until fn returns false.
func (r IdentityConstraintRead) ForEachAttributeWildcardField(fn func(CompiledIdentityField) bool) {
	for _, field := range r.attributeWildcardFields {
		if !fn(field) {
			return
		}
	}
}

// Refer returns the referenced key for keyref constraints.
func (r IdentityConstraintRead) Refer() IdentityConstraintID {
	return r.refer
}

// Kind returns the identity constraint kind.
func (r IdentityConstraintRead) Kind() IdentityKind {
	return r.kind
}
