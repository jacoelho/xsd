package runtime

// IdentityConstraintInfo is the runtime metadata needed to finish a selected
// identity tuple.
type IdentityConstraintInfo struct {
	Refer IdentityConstraintID
	Kind  IdentityKind
}

// IdentityConstraintIDs is an immutable ordered view of identity-constraint
// handles. Its backing slice is never exposed.
type IdentityConstraintIDs struct {
	values []IdentityConstraintID
}

func borrowedIdentityConstraintIDs(values []IdentityConstraintID) IdentityConstraintIDs {
	return IdentityConstraintIDs{values: values}
}

// Len returns the number of constraint handles.
func (r IdentityConstraintIDs) Len() int {
	return len(r.values)
}

// At returns the constraint handle at index.
func (r IdentityConstraintIDs) At(index int) (IdentityConstraintID, bool) {
	if index < 0 || index >= len(r.values) {
		return 0, false
	}
	return r.values[index], true
}

// IdentityPathRead is an immutable selector-path view.
type IdentityPathRead struct {
	steps      []IdentityStep
	descendant bool
	self       bool
}

// IdentityPathReads is an immutable ordered selector-path view.
type IdentityPathReads struct {
	values []IdentityPath
}

func borrowedIdentityPathReads(paths []IdentityPath) IdentityPathReads {
	return IdentityPathReads{values: paths}
}

// Len returns the number of selector paths.
func (r IdentityPathReads) Len() int {
	return len(r.values)
}

// At returns selector path index.
func (r IdentityPathReads) At(index int) (IdentityPathRead, bool) {
	if index < 0 || index >= len(r.values) {
		return IdentityPathRead{}, false
	}
	return borrowedIdentityPathRead(r.values[index]), true
}

func borrowedIdentityPathRead(path IdentityPath) IdentityPathRead {
	return IdentityPathRead{steps: path.Steps, descendant: path.Descendant, self: path.Self}
}

// StepCount returns the number of selector steps.
func (r IdentityPathRead) StepCount() int {
	return len(r.steps)
}

// Step returns selector step index.
func (r IdentityPathRead) Step(index int) (IdentityStep, bool) {
	if index < 0 || index >= len(r.steps) {
		return IdentityStep{}, false
	}
	return r.steps[index], true
}

// Descendant reports whether the path uses descendant matching.
func (r IdentityPathRead) Descendant() bool {
	return r.descendant
}

// Self reports whether the path selects the current node.
func (r IdentityPathRead) Self() bool {
	return r.self
}

// IdentityFieldPathRead is an immutable compiled field-path view.
type IdentityFieldPathRead struct {
	steps            []IdentityStep
	attribute        QName
	attrNamespace    NamespaceID
	descendant       bool
	self             bool
	attr             bool
	attrWildcard     bool
	attrNamespaceSet bool
}

func borrowedIdentityFieldPathRead(path IdentityFieldPath) IdentityFieldPathRead {
	return IdentityFieldPathRead{
		steps:            path.Steps,
		attribute:        path.Attribute,
		attrNamespace:    path.AttrNamespace,
		descendant:       path.Descendant,
		self:             path.Self,
		attr:             path.Attr,
		attrWildcard:     path.AttrWildcard,
		attrNamespaceSet: path.AttrNamespaceSet,
	}
}

// StepCount returns the number of element steps.
func (r IdentityFieldPathRead) StepCount() int {
	return len(r.steps)
}

// Step returns element step index.
func (r IdentityFieldPathRead) Step(index int) (IdentityStep, bool) {
	if index < 0 || index >= len(r.steps) {
		return IdentityStep{}, false
	}
	return r.steps[index], true
}

// Attribute returns the exact attribute name for the path.
func (r IdentityFieldPathRead) Attribute() QName {
	return r.attribute
}

// AttributeNamespace returns the namespace constraint for an attribute wildcard.
func (r IdentityFieldPathRead) AttributeNamespace() NamespaceID {
	return r.attrNamespace
}

// Descendant reports whether the path uses descendant matching.
func (r IdentityFieldPathRead) Descendant() bool {
	return r.descendant
}

// Self reports whether the path selects the current node.
func (r IdentityFieldPathRead) Self() bool {
	return r.self
}

// IsAttribute reports whether the path selects an attribute.
func (r IdentityFieldPathRead) IsAttribute() bool {
	return r.attr
}

// AttributeWildcard reports whether the path selects attributes by wildcard.
func (r IdentityFieldPathRead) AttributeWildcard() bool {
	return r.attrWildcard
}

// AttributeNamespaceSet reports whether an attribute wildcard constrains namespace.
func (r IdentityFieldPathRead) AttributeNamespaceSet() bool {
	return r.attrNamespaceSet
}

// CompiledIdentityFieldRead is an immutable compiled field lookup view.
type CompiledIdentityFieldRead struct {
	paths []IdentityFieldPath
	field int
}

// CompiledIdentityFieldReads is an immutable ordered compiled-field view.
type CompiledIdentityFieldReads struct {
	values []CompiledIdentityField
}

func borrowedCompiledIdentityFieldReads(fields []CompiledIdentityField) CompiledIdentityFieldReads {
	return CompiledIdentityFieldReads{values: fields}
}

// Len returns the number of compiled fields.
func (r CompiledIdentityFieldReads) Len() int {
	return len(r.values)
}

// At returns compiled field index.
func (r CompiledIdentityFieldReads) At(index int) (CompiledIdentityFieldRead, bool) {
	if index < 0 || index >= len(r.values) {
		return CompiledIdentityFieldRead{}, false
	}
	return borrowedCompiledIdentityFieldRead(r.values[index]), true
}

func borrowedCompiledIdentityFieldRead(field CompiledIdentityField) CompiledIdentityFieldRead {
	return CompiledIdentityFieldRead{paths: field.Paths, field: field.Field}
}

// Field returns the declared field index.
func (r CompiledIdentityFieldRead) Field() int {
	return r.field
}

// PathCount returns the number of compiled path alternatives.
func (r CompiledIdentityFieldRead) PathCount() int {
	return len(r.paths)
}

// Path returns compiled path index.
func (r CompiledIdentityFieldRead) Path(index int) (IdentityFieldPathRead, bool) {
	if index < 0 || index >= len(r.paths) {
		return IdentityFieldPathRead{}, false
	}
	return borrowedIdentityFieldPathRead(r.paths[index]), true
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

// ElementIdentityConstraintIDs returns an immutable view of the constraints
// attached to id.
func ElementIdentityConstraintIDs(reads [][]IdentityConstraintID, id ElementID) (IdentityConstraintIDs, bool) {
	if !ValidElementID(id, len(reads)) {
		return IdentityConstraintIDs{}, false
	}
	return borrowedIdentityConstraintIDs(reads[id]), true
}

func identityConstraintReadByIDPtr(reads []IdentityConstraintRead, id IdentityConstraintID) (*IdentityConstraintRead, bool) {
	if !ValidIdentityConstraintID(id, len(reads)) {
		return nil, false
	}
	return &reads[id], true
}

// IdentitySelectorPathReads returns immutable selector-path views for id.
func IdentitySelectorPathReads(reads []IdentityConstraintRead, id IdentityConstraintID) (IdentityPathReads, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return IdentityPathReads{}, false
	}
	return borrowedIdentityPathReads(ic.selector), true
}

// IdentityFieldCount returns the field count for id.
func IdentityFieldCount(reads []IdentityConstraintRead, id IdentityConstraintID) (int, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return 0, false
	}
	return ic.FieldCount(), true
}

// IdentityElementFieldReads returns immutable element-field views for id.
func IdentityElementFieldReads(reads []IdentityConstraintRead, id IdentityConstraintID) (CompiledIdentityFieldReads, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return CompiledIdentityFieldReads{}, false
	}
	return borrowedCompiledIdentityFieldReads(ic.elementFields), true
}

// IdentityAttributeFieldReads returns immutable matching attribute-field views
// for id and name.
func IdentityAttributeFieldReads(reads []IdentityConstraintRead, id IdentityConstraintID, name QName) (CompiledIdentityFieldReads, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return CompiledIdentityFieldReads{}, false
	}
	return borrowedCompiledIdentityFieldReads(ic.attributeFields[name]), true
}

// IdentityAttributeWildcardFieldReads returns immutable wildcard-field views for id.
func IdentityAttributeWildcardFieldReads(reads []IdentityConstraintRead, id IdentityConstraintID) (CompiledIdentityFieldReads, bool) {
	ic, ok := identityConstraintReadByIDPtr(reads, id)
	if !ok {
		return CompiledIdentityFieldReads{}, false
	}
	return borrowedCompiledIdentityFieldReads(ic.attributeWildcardFields), true
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

// FieldCount returns the declared identity field count.
func (r IdentityConstraintRead) FieldCount() int {
	return r.fieldCount
}

// Refer returns the referenced key for keyref constraints.
func (r IdentityConstraintRead) Refer() IdentityConstraintID {
	return r.refer
}

// Kind returns the identity constraint kind.
func (r IdentityConstraintRead) Kind() IdentityKind {
	return r.kind
}
