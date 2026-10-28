package schema

import "slices"

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

// IsSubsetOf reports whether every constraint handle in r occurs in base.
func (r IdentityConstraintIDs) IsSubsetOf(base IdentityConstraintIDs) bool {
	for _, id := range r.values {
		if !slices.Contains(base.values, id) {
			return false
		}
	}
	return true
}

// IdentityNamespaceResolver resolves schema namespace IDs to their URI.
// Runtime identity paths use URI comparison when an instance name is not
// interned in the schema name table.
type IdentityNamespaceResolver interface {
	Namespace(id NamespaceID) string
}

// IdentityPathProgramRead is the immutable execution projection of one
// selector or field path. It is built once with the published schema and
// shared by every validation session using that schema.
//
//nolint:recvcheck // Matches avoids copying this immutable program on the hot validation path; accessors remain value reads.
type IdentityPathProgramRead struct {
	steps                 []IdentityStep
	descendant            bool
	self                  bool
	attribute             QName
	attributeNamespace    NamespaceID
	attributeNamespaceSet bool
	attributePath         bool
	attributeWildcard     bool
}

func identitySelectorPathProgramRead(path IdentityPath) IdentityPathProgramRead {
	return IdentityPathProgramRead{
		steps:      slices.Clone(path.Steps),
		descendant: path.Descendant,
		self:       path.Self,
	}
}

func identityFieldPathProgramRead(path IdentityFieldPath) IdentityPathProgramRead {
	return IdentityPathProgramRead{
		steps:                 slices.Clone(path.Steps),
		attribute:             path.Attribute,
		attributeNamespace:    path.AttrNamespace,
		attributeNamespaceSet: path.AttrNamespaceSet,
		descendant:            path.Descendant,
		self:                  path.Self,
		attributePath:         path.Attr,
		attributeWildcard:     path.AttrWildcard,
	}
}

// StepCount returns the number of element steps in the path.
func (p IdentityPathProgramRead) StepCount() int {
	return len(p.steps)
}

// Step returns one path step.
func (p IdentityPathProgramRead) Step(index int) (IdentityStep, bool) {
	if index < 0 || index >= len(p.steps) {
		return IdentityStep{}, false
	}
	return p.steps[index], true
}

// FinalStep returns the terminal element step when the path has one.
func (p IdentityPathProgramRead) FinalStep() (IdentityStep, bool) {
	count := p.StepCount()
	if count == 0 {
		return IdentityStep{}, false
	}
	return p.Step(count - 1)
}

// Descendant reports whether the path uses descendant matching.
func (p IdentityPathProgramRead) Descendant() bool { return p.descendant }

// Self reports whether the path selects the current node.
func (p IdentityPathProgramRead) Self() bool { return p.self }

// IsAttribute reports whether the path selects an attribute.
func (p IdentityPathProgramRead) IsAttribute() bool { return p.attributePath }

// Attribute returns the exact attribute name for the path.
func (p IdentityPathProgramRead) Attribute() QName { return p.attribute }

// AttributeNamespaceSet reports whether an attribute wildcard constrains namespace.
func (p IdentityPathProgramRead) AttributeNamespaceSet() bool {
	return p.attributeNamespaceSet
}

// AttributeWildcard reports whether the path selects attributes by wildcard.
func (p IdentityPathProgramRead) AttributeWildcard() bool { return p.attributeWildcard }

// Matches reports whether the path matches namePath from baseDepth to
// currentDepth.
//
//nolint:gocognit // Keep depth validation and step dispatch on one hot path.
func (p *IdentityPathProgramRead) Matches(
	names IdentityNamespaceResolver,
	namePath []RuntimeName,
	baseDepth, currentDepth int,
) bool {
	if p.self {
		return currentDepth == baseDepth
	}
	if !p.descendant && len(p.steps) == 0 {
		return baseDepth >= 0 && currentDepth == baseDepth && currentDepth <= len(namePath)
	}
	var rel []RuntimeName
	var ok bool
	if p.descendant {
		rel, ok = identityDescendantPath(namePath, baseDepth, currentDepth, p.StepCount())
	} else {
		rel, ok = identityDirectPath(namePath, baseDepth, currentDepth, p.StepCount())
	}
	if !ok {
		return false
	}
	for index := range p.StepCount() {
		step, ok := p.Step(index)
		if !ok || !identityProgramStepMatches(names, rel[index], step) {
			return false
		}
	}
	return true
}

func identityDirectPath(namePath []RuntimeName, baseDepth, currentDepth, stepCount int) ([]RuntimeName, bool) {
	if currentDepth < baseDepth || baseDepth < 0 || currentDepth > len(namePath) {
		return nil, false
	}
	rel := namePath[baseDepth:currentDepth]
	return rel, len(rel) == stepCount
}

func identityDescendantPath(namePath []RuntimeName, baseDepth, currentDepth, stepCount int) ([]RuntimeName, bool) {
	if currentDepth < baseDepth || baseDepth < 0 || currentDepth > len(namePath) {
		return nil, false
	}
	rel := namePath[baseDepth:currentDepth]
	if len(rel) < stepCount {
		return nil, false
	}
	return rel[len(rel)-stepCount:], true
}

// AttributeMatches reports whether name matches the path's attribute branch.
func (p IdentityPathProgramRead) AttributeMatches(names IdentityNamespaceResolver, name RuntimeName) bool {
	if !p.attributePath {
		return false
	}
	if !p.attributeWildcard {
		return name.Known && p.attribute == name.Name
	}
	if !p.attributeNamespaceSet {
		return true
	}
	if name.Known {
		return p.attributeNamespace == name.Name.Namespace
	}
	return names.Namespace(p.attributeNamespace) == name.NS
}

func identityProgramStepMatches(names IdentityNamespaceResolver, name RuntimeName, step IdentityStep) bool {
	if !step.Wildcard {
		return name.Known && name.Name == step.Name
	}
	if !step.NamespaceSet {
		return true
	}
	if name.Known {
		return name.Name.Namespace == step.Namespace
	}
	return name.NS == names.Namespace(step.Namespace)
}

// IdentityPathProgramReads is an immutable ordered view of path programs.
type IdentityPathProgramReads struct {
	values []IdentityPathProgramRead
}

// Len returns the number of path programs.
func (r IdentityPathProgramReads) Len() int { return len(r.values) }

// At returns a path program by index.
func (r IdentityPathProgramReads) At(index int) (IdentityPathProgramRead, bool) {
	if index < 0 || index >= len(r.values) {
		return IdentityPathProgramRead{}, false
	}
	return r.values[index], true
}

// IdentityCompiledFieldProgramRead is an immutable compiled field-path group.
type IdentityCompiledFieldProgramRead struct {
	paths []IdentityPathProgramRead
	field int
}

// IdentityCompiledFieldProgramReads is an immutable ordered field-program view.
type IdentityCompiledFieldProgramReads struct {
	values []IdentityCompiledFieldProgramRead
}

// Len returns the number of compiled field programs.
func (r IdentityCompiledFieldProgramReads) Len() int { return len(r.values) }

// At returns a compiled field program by index.
func (r IdentityCompiledFieldProgramReads) At(index int) (IdentityCompiledFieldProgramRead, bool) {
	if index < 0 || index >= len(r.values) {
		return IdentityCompiledFieldProgramRead{}, false
	}
	return r.values[index], true
}

// Field returns the declared field index.
func (r IdentityCompiledFieldProgramRead) Field() int { return r.field }

// PathCount returns the number of path alternatives.
func (r IdentityCompiledFieldProgramRead) PathCount() int { return len(r.paths) }

// Path returns one path alternative.
func (r IdentityCompiledFieldProgramRead) Path(index int) (IdentityPathProgramRead, bool) {
	if index < 0 || index >= len(r.paths) {
		return IdentityPathProgramRead{}, false
	}
	return r.paths[index], true
}

// IdentityConstraintProgramRead is the immutable dispatch program for one
// identity constraint.
type IdentityConstraintProgramRead struct {
	selectors          []IdentityPathProgramRead
	elementFields      []IdentityCompiledFieldProgramRead
	attributeFields    map[QName][]IdentityCompiledFieldProgramRead
	attributeWildcards []IdentityCompiledFieldProgramRead
	refer              IdentityConstraintID
	kind               IdentityKind
	fieldCount         int
}

// Selectors returns selector path programs.
func (p IdentityConstraintProgramRead) Selectors() IdentityPathProgramReads {
	return IdentityPathProgramReads{values: p.selectors}
}

// ElementFields returns element field-path programs.
func (p IdentityConstraintProgramRead) ElementFields() IdentityCompiledFieldProgramReads {
	return IdentityCompiledFieldProgramReads{values: p.elementFields}
}

// AttributeFields returns exact-name attribute field-path programs.
func (p IdentityConstraintProgramRead) AttributeFields(name QName) IdentityCompiledFieldProgramReads {
	return IdentityCompiledFieldProgramReads{values: p.attributeFields[name]}
}

// AttributeWildcardFields returns wildcard attribute field-path programs.
func (p IdentityConstraintProgramRead) AttributeWildcardFields() IdentityCompiledFieldProgramReads {
	return IdentityCompiledFieldProgramReads{values: p.attributeWildcards}
}

// Refer returns the referenced key for keyref constraints.
func (p IdentityConstraintProgramRead) Refer() IdentityConstraintID { return p.refer }

// Kind returns the identity constraint kind.
func (p IdentityConstraintProgramRead) Kind() IdentityKind { return p.kind }

// FieldCount returns the declared identity field count.
func (p IdentityConstraintProgramRead) FieldCount() int { return p.fieldCount }

// IdentitySelectorDispatchRead is one immutable selector dispatch candidate.
type IdentitySelectorDispatchRead struct {
	path       IdentityPathProgramRead
	constraint IdentityConstraintID
}

// Constraint returns the owning identity constraint.
func (r IdentitySelectorDispatchRead) Constraint() IdentityConstraintID { return r.constraint }

// Path returns the selector path program.
func (r IdentitySelectorDispatchRead) Path() IdentityPathProgramRead { return r.path }

// IdentitySelectorDispatchReads is an immutable selector candidate view.
type IdentitySelectorDispatchReads struct {
	values []IdentitySelectorDispatchRead
}

// Len returns the number of selector candidates.
func (r IdentitySelectorDispatchReads) Len() int { return len(r.values) }

// At returns a selector candidate by index.
func (r IdentitySelectorDispatchReads) At(index int) (IdentitySelectorDispatchRead, bool) {
	if index < 0 || index >= len(r.values) {
		return IdentitySelectorDispatchRead{}, false
	}
	return r.values[index], true
}

// IdentityFieldDispatchRead is one immutable element-field dispatch candidate.
type IdentityFieldDispatchRead struct {
	path       IdentityPathProgramRead
	field      int
	constraint IdentityConstraintID
}

// Constraint returns the owning identity constraint.
func (r IdentityFieldDispatchRead) Constraint() IdentityConstraintID { return r.constraint }

// Path returns the field path program.
func (r IdentityFieldDispatchRead) Path() IdentityPathProgramRead { return r.path }

// Field returns the declared field index.
func (r IdentityFieldDispatchRead) Field() int { return r.field }

// IdentityFieldDispatchReads is an immutable field candidate view.
type IdentityFieldDispatchReads struct {
	values []IdentityFieldDispatchRead
}

// Len returns the number of field candidates.
func (r IdentityFieldDispatchReads) Len() int { return len(r.values) }

// At returns a field candidate by index.
func (r IdentityFieldDispatchReads) At(index int) (IdentityFieldDispatchRead, bool) {
	if index < 0 || index >= len(r.values) {
		return IdentityFieldDispatchRead{}, false
	}
	return r.values[index], true
}

type identitySelectorDispatchIndex struct {
	exact     map[QName][]IdentitySelectorDispatchRead
	namespace map[string][]IdentitySelectorDispatchRead
	any       []IdentitySelectorDispatchRead
}

type identityFieldDispatchIndex struct {
	exact     map[QName][]IdentityFieldDispatchRead
	namespace map[string][]IdentityFieldDispatchRead
	any       []IdentityFieldDispatchRead
}

// IdentityDispatchRead is the immutable identity selector and field index
// derived while publishing a schema. Validation sessions retain only their
// active scope membership and scratch slices.
type IdentityDispatchRead struct {
	programs    []IdentityConstraintProgramRead
	selectors   identitySelectorDispatchIndex
	elements    identityFieldDispatchIndex
	elementSelf []IdentityFieldDispatchRead
}

// ConstraintCount returns the number of published identity constraints.
func (d IdentityDispatchRead) ConstraintCount() int { return len(d.programs) }

// Program returns the immutable dispatch program for an identity constraint.
func (d IdentityDispatchRead) Program(id IdentityConstraintID) (IdentityConstraintProgramRead, bool) {
	if !ValidIdentityConstraintID(id, len(d.programs)) {
		return IdentityConstraintProgramRead{}, false
	}
	return d.programs[id], true
}

// SelectorExact returns selector candidates keyed by an interned name.
func (d IdentityDispatchRead) SelectorExact(name QName) IdentitySelectorDispatchReads {
	return IdentitySelectorDispatchReads{values: d.selectors.exact[name]}
}

// SelectorNamespace returns selector candidates keyed by namespace URI.
func (d IdentityDispatchRead) SelectorNamespace(namespace string) IdentitySelectorDispatchReads {
	return IdentitySelectorDispatchReads{values: d.selectors.namespace[namespace]}
}

// SelectorAny returns selectors without a terminal name test.
func (d IdentityDispatchRead) SelectorAny() IdentitySelectorDispatchReads {
	return IdentitySelectorDispatchReads{values: d.selectors.any}
}

// ElementExact returns element field candidates keyed by an interned name.
func (d IdentityDispatchRead) ElementExact(name QName) IdentityFieldDispatchReads {
	return IdentityFieldDispatchReads{values: d.elements.exact[name]}
}

// ElementNamespace returns element field candidates keyed by namespace URI.
func (d IdentityDispatchRead) ElementNamespace(namespace string) IdentityFieldDispatchReads {
	return IdentityFieldDispatchReads{values: d.elements.namespace[namespace]}
}

// ElementAny returns element field candidates without a terminal name test.
func (d IdentityDispatchRead) ElementAny() IdentityFieldDispatchReads {
	return IdentityFieldDispatchReads{values: d.elements.any}
}

// ElementSelf returns element field candidates matching the current node.
func (d IdentityDispatchRead) ElementSelf() IdentityFieldDispatchReads {
	return IdentityFieldDispatchReads{values: d.elementSelf}
}

func newIdentityDispatchRead(identities []IdentityConstraint, names nameReadView) IdentityDispatchRead {
	dispatch := IdentityDispatchRead{programs: make([]IdentityConstraintProgramRead, len(identities))}
	constraint := IdentityConstraintID(0)
	for _, identity := range identities {
		program := newIdentityConstraintProgramRead(identity)
		dispatch.programs[constraint] = program
		addIdentityConstraintDispatch(&dispatch, constraint, program, names)
		constraint++
	}
	return dispatch
}

func addIdentityConstraintDispatch(d *IdentityDispatchRead, constraint IdentityConstraintID, program IdentityConstraintProgramRead, names nameReadView) {
	addIdentitySelectorDispatchBranches(d, constraint, program.selectors, names)
	addIdentityFieldDispatchBranches(d, constraint, program.elementFields, names)
}

// addIdentitySelectorDispatchBranches indexes selector branches by their
// terminal step. A self branch matches only at its scope element, so it is
// not indexed: validation reads it from the constraint program of a scope
// opened at the current element.
func addIdentitySelectorDispatchBranches(d *IdentityDispatchRead, constraint IdentityConstraintID, paths []IdentityPathProgramRead, names nameReadView) {
	for _, path := range paths {
		if path.Self() {
			continue
		}
		branch := IdentitySelectorDispatchRead{path: path, constraint: constraint}
		if step, ok := path.FinalStep(); ok {
			addIdentitySelectorDispatch(&d.selectors, step, branch, names)
		} else {
			d.selectors.any = append(d.selectors.any, branch)
		}
	}
}

func addIdentityFieldDispatchBranches(d *IdentityDispatchRead, constraint IdentityConstraintID, fields []IdentityCompiledFieldProgramRead, names nameReadView) {
	for _, field := range fields {
		for _, path := range field.paths {
			branch := IdentityFieldDispatchRead{path: path, field: field.field, constraint: constraint}
			if step, ok := path.FinalStep(); ok {
				addIdentityFieldDispatch(&d.elements, step, branch, names)
			} else {
				d.elementSelf = append(d.elementSelf, branch)
			}
		}
	}
}

func addIdentitySelectorDispatch(index *identitySelectorDispatchIndex, step IdentityStep, branch IdentitySelectorDispatchRead, names nameReadView) {
	if !step.Wildcard {
		if index.exact == nil {
			index.exact = make(map[QName][]IdentitySelectorDispatchRead)
		}
		index.exact[step.Name] = append(index.exact[step.Name], branch)
		return
	}
	if !step.NamespaceSet {
		index.any = append(index.any, branch)
		return
	}
	if index.namespace == nil {
		index.namespace = make(map[string][]IdentitySelectorDispatchRead)
	}
	key := names.Namespace(step.Namespace)
	index.namespace[key] = append(index.namespace[key], branch)
}

func addIdentityFieldDispatch(index *identityFieldDispatchIndex, step IdentityStep, branch IdentityFieldDispatchRead, names nameReadView) {
	if !step.Wildcard {
		if index.exact == nil {
			index.exact = make(map[QName][]IdentityFieldDispatchRead)
		}
		index.exact[step.Name] = append(index.exact[step.Name], branch)
		return
	}
	if !step.NamespaceSet {
		index.any = append(index.any, branch)
		return
	}
	if index.namespace == nil {
		index.namespace = make(map[string][]IdentityFieldDispatchRead)
	}
	key := names.Namespace(step.Namespace)
	index.namespace[key] = append(index.namespace[key], branch)
}

//nolint:gocognit // Keep ordered field partition construction and bounded publication storage under one owner.
func newIdentityConstraintProgramRead(identity IdentityConstraint) IdentityConstraintProgramRead {
	program := IdentityConstraintProgramRead{
		selectors:  newIdentitySelectorPathProgramReads(identity.Selector),
		refer:      identity.Refer,
		kind:       identity.Kind,
		fieldCount: len(identity.Fields),
	}
	for fieldIndex, field := range identity.Fields {
		var elements, wildcards []IdentityPathProgramRead
		var exact map[QName][]IdentityPathProgramRead
		for _, path := range field.Paths {
			compiled := identityFieldPathProgramRead(path)
			switch {
			case !path.Attr:
				elements = append(elements, compiled)
			case path.AttrWildcard:
				wildcards = append(wildcards, compiled)
			default:
				if exact == nil {
					exact = make(map[QName][]IdentityPathProgramRead)
				}
				exact[path.Attribute] = append(exact[path.Attribute], compiled)
			}
		}
		if len(elements) != 0 {
			program.elementFields = append(program.elementFields, IdentityCompiledFieldProgramRead{field: fieldIndex, paths: tightIdentityProgramSlice(elements)})
		}
		if len(wildcards) != 0 {
			program.attributeWildcards = append(program.attributeWildcards, IdentityCompiledFieldProgramRead{field: fieldIndex, paths: tightIdentityProgramSlice(wildcards)})
		}
		for name, paths := range exact {
			if program.attributeFields == nil {
				program.attributeFields = make(map[QName][]IdentityCompiledFieldProgramRead)
			}
			program.attributeFields[name] = append(program.attributeFields[name], IdentityCompiledFieldProgramRead{field: fieldIndex, paths: tightIdentityProgramSlice(paths)})
		}
	}
	program.elementFields = tightIdentityProgramSlice(program.elementFields)
	program.attributeWildcards = tightIdentityProgramSlice(program.attributeWildcards)
	if program.attributeFields != nil {
		attributes := make(map[QName][]IdentityCompiledFieldProgramRead, len(program.attributeFields))
		for name, fields := range program.attributeFields {
			attributes[name] = tightIdentityProgramSlice(fields)
		}
		program.attributeFields = attributes
	}
	return program
}

// Publication retains only occupied groups and paths, never append growth slack.
func tightIdentityProgramSlice[T any](values []T) []T {
	if len(values) == cap(values) {
		return values
	}
	tight := make([]T, len(values))
	copy(tight, values)
	return tight
}

func newIdentitySelectorPathProgramReads(paths []IdentityPath) []IdentityPathProgramRead {
	programs := make([]IdentityPathProgramRead, len(paths))
	for index, path := range paths {
		programs[index] = identitySelectorPathProgramRead(path)
	}
	return programs
}

// ElementIdentityConstraintIDs returns an immutable view of the constraints
// attached to id.
func ElementIdentityConstraintIDs(reads [][]IdentityConstraintID, id ElementID) (IdentityConstraintIDs, bool) {
	if !ValidElementID(id, len(reads)) {
		return IdentityConstraintIDs{}, false
	}
	return borrowedIdentityConstraintIDs(reads[id]), true
}
