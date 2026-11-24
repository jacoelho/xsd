package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// attributeWildcardDerivation identifies how an attribute wildcard was derived.
type attributeWildcardDerivation uint8

const (
	// attributeWildcardNone records a locally declared or absent wildcard.
	attributeWildcardNone attributeWildcardDerivation = iota
	// attributeWildcardRestriction records wildcard derivation by restriction.
	attributeWildcardRestriction
	// attributeWildcardExtension records wildcard derivation by extension.
	attributeWildcardExtension
)

// validAttributeWildcardDerivation reports whether kind is a known attribute
// wildcard derivation kind.
func validAttributeWildcardDerivation(kind attributeWildcardDerivation) bool {
	switch kind {
	case attributeWildcardNone, attributeWildcardRestriction, attributeWildcardExtension:
		return true
	default:
		return false
	}
}

// attributeUseSet is the runtime record for one attribute-use set.
type attributeUseSet struct {
	Index            map[QName]uint32
	Uses             []attributeUse
	Wildcard         WildcardID
	WildcardBase     WildcardID
	WildcardDeclared WildcardID
	WildcardDerive   attributeWildcardDerivation
}

// attributeUse is the construction record for one attribute use. Copies retain
// componentID for set union; publication discards this compiler-local identity.
// Prohibitions remain source-name directives and never enter this record.
type attributeUse struct {
	ValueConstraint *valueConstraint
	Name            QName
	Type            SimpleTypeID
	componentID     uint32
	Required        bool
}

// AttributeUseSetRead exposes validation-facing attribute-use-set behavior
// without exposing raw slot slices to the validator.
type AttributeUseSetRead struct {
	index            map[QName]uint32
	uses             []AttributeUseRead
	required         []uint32
	valueConstraints []uint32
	wildcard         WildcardID
	singleUse        bool
	hasIDUse         bool
}

// AttributeUseSlots is an immutable ordered view of attribute-use slots.
// Its backing slice is never exposed.
type AttributeUseSlots struct {
	values []uint32
}

// Len returns the number of slots.
func (r AttributeUseSlots) Len() int {
	return len(r.values)
}

// At returns slot index.
func (r AttributeUseSlots) At(index int) (uint32, bool) {
	if index < 0 || index >= len(r.values) {
		return 0, false
	}
	return r.values[index], true
}

func attributeUseSetReadHasSingleUse(s AttributeUseSetRead) bool {
	if len(s.uses) != 1 || len(s.index) != 1 {
		return false
	}
	slot, ok := s.index[s.uses[0].name]
	return ok && slot == 0
}

func newAttributeUseSetReads(names *nameTable, sets []attributeUseSet, values *valuepkg.Builder) ([]AttributeUseSetRead, error) {
	out := make([]AttributeUseSetRead, len(sets))
	for i := range sets {
		read, err := newAttributeUseSetRead(names, &sets[i], values)
		if err != nil {
			return nil, err
		}
		out[i] = read
	}
	return out, nil
}

// newAttributeUseSetRead runs on effective uses, after inheritance and
// restriction. HasIDUse comes only from completed value-builder input
// requirements; missing metadata is an invariant error, never a cue to scan
// raw type specifications or ancestry.
func newAttributeUseSetRead(names *nameTable, set *attributeUseSet, values *valuepkg.Builder) (AttributeUseSetRead, error) {
	uses := make([]AttributeUseRead, len(set.Uses))
	var required, valueConstraints []uint32
	var hasIDUse bool
	for j := range set.Uses {
		uses[j] = newAttributeUseRead(names, set.Uses[j])
		input, ok := values.InputRequirements(set.Uses[j].Type)
		if !ok {
			return AttributeUseSetRead{}, xsderrors.InternalInvariant("attribute use input metadata is invalid", nil)
		}
		hasIDUse = hasIDUse || input.CarriesID
		if set.Uses[j].Required {
			required = append(required, uint32(j))
		}
		if _, applies := uses[j].AbsentValueConstraint(); applies {
			valueConstraints = append(valueConstraints, uint32(j))
		}
	}
	read := AttributeUseSetRead{
		index:            maps.Clone(set.Index),
		uses:             uses,
		required:         slices.Clone(required),
		valueConstraints: slices.Clone(valueConstraints),
		wildcard:         set.Wildcard,
		hasIDUse:         hasIDUse,
	}
	read.singleUse = attributeUseSetReadHasSingleUse(read)
	return read, nil
}

// HasIDUse reports whether an effective declared use has an ID-derived type.
func (s AttributeUseSetRead) HasIDUse() bool {
	return s.hasIDUse
}

// UseCount returns the number of declared uses in the set.
func (s AttributeUseSetRead) UseCount() int {
	return len(s.uses)
}

// UseAt returns the declared use stored at slot.
func (s AttributeUseSetRead) UseAt(slot int) (AttributeUseRead, bool) {
	if slot < 0 || slot >= len(s.uses) {
		return AttributeUseRead{}, false
	}
	return s.uses[slot], true
}

// DeclaredUse returns the declared use matching name, if present.
func (s AttributeUseSetRead) DeclaredUse(name QName) (AttributeUseRead, int, bool) {
	if s.singleUse {
		use := s.uses[0]
		if use.name == name {
			return use, 0, true
		}
		return AttributeUseRead{}, -1, false
	}
	slot, ok := s.index[name]
	if !ok || !validUint32Index(slot, len(s.uses)) || s.uses[slot].name != name {
		return AttributeUseRead{}, -1, false
	}
	return s.uses[slot], int(slot), true
}

// Wildcard returns the attribute wildcard attached to the set.
func (s AttributeUseSetRead) Wildcard() WildcardID {
	return s.wildcard
}

// RequiredSlots returns an immutable view of required-use slots.
func (s AttributeUseSetRead) RequiredSlots() AttributeUseSlots {
	return AttributeUseSlots{values: s.required}
}

// ValueConstraintSlots returns slots whose constraints can supply absent values.
func (s AttributeUseSetRead) ValueConstraintSlots() AttributeUseSlots {
	return AttributeUseSlots{values: s.valueConstraints}
}

// AttributeUseRead exposes validation-facing facts for one declared attribute
// use without exposing compiler-owned storage.
type AttributeUseRead struct {
	quotedName string
	constraint ValueConstraintRead
	name       QName
	typ        SimpleTypeID
	required   bool
	kind       valueConstraintKind
}

// newAttributeUseRead returns an immutable validation read projection for one
// attribute use.
func newAttributeUseRead(names *nameTable, use attributeUse) AttributeUseRead {
	// Adopted WG correction 4363 (I-007 in docs/spec/INTERPRETATIONS.md)
	// makes XSI constraints inapplicable during
	// validation. Keep them on construction records for schema correctness and
	// derivation; publication permits only the four intrinsic XSI declarations.
	if names.Namespace(use.Name.Namespace) == vocab.XSINamespaceURI {
		use.ValueConstraint = nil
	}
	constraint, _ := newValueConstraintReadFromConstraint(use.ValueConstraint)
	return AttributeUseRead{
		name:       use.Name,
		typ:        use.Type,
		quotedName: names.Format(use.Name),
		constraint: constraint,
		kind:       use.ValueConstraint.kind(),
		required:   use.Required,
	}
}

// Name returns the runtime QName for the use.
func (u AttributeUseRead) Name() QName {
	return u.name
}

// TypeID returns the simple type used to validate the attribute.
func (u AttributeUseRead) TypeID() SimpleTypeID {
	return u.typ
}

// QuotedName returns the attribute's expanded name, quoted for diagnostics.
func (u AttributeUseRead) QuotedName() string {
	return u.quotedName
}

// Required reports whether the attribute must be present.
func (u AttributeUseRead) Required() bool {
	return u.required
}

// FixedValue returns the fixed value, if present.
func (u AttributeUseRead) FixedValue() (ValueConstraintRead, bool) {
	if u.kind != valueConstraintFixed {
		return ValueConstraintRead{}, false
	}
	return u.constraint, true
}

// AbsentValueConstraint returns the fixed/default value applied when the
// attribute is absent.
func (u AttributeUseRead) AbsentValueConstraint() (ValueConstraintRead, bool) {
	if !u.kind.present() {
		return ValueConstraintRead{}, false
	}
	return u.constraint, true
}

// AttributeDeclRead exposes validation-facing facts for one global attribute
// declaration without exposing compiler-owned storage.
type AttributeDeclRead struct {
	fixed    ValueConstraintRead
	name     QName
	typ      SimpleTypeID
	hasFixed bool
}

// newAttributeDeclRead returns an immutable validation read projection for one
// global attribute declaration.
func newAttributeDeclRead(decl attributeDecl) AttributeDeclRead {
	fixed, hasFixed := newValueConstraintReadFromConstraint(decl.ValueConstraint.fixedConstraint())
	return AttributeDeclRead{
		name:     decl.Name,
		typ:      decl.Type,
		fixed:    fixed,
		hasFixed: hasFixed,
	}
}

// newAttributeDeclReadsForDecls returns immutable validation read projections
// for frozen global attribute declarations.
func newAttributeDeclReadsForDecls(decls []attributeDecl) []AttributeDeclRead {
	out := make([]AttributeDeclRead, len(decls))
	for i := range decls {
		out[i] = newAttributeDeclRead(decls[i])
	}
	return out
}

// attributeDeclReadByID returns the validation read projection for id.
func attributeDeclReadByID(reads []AttributeDeclRead, id AttributeID) (AttributeDeclRead, bool) {
	if !validAttributeID(id, len(reads)) {
		return AttributeDeclRead{}, false
	}
	return reads[id], true
}

// Name returns the runtime QName for the declaration.
func (d AttributeDeclRead) Name() QName {
	return d.name
}

// TypeID returns the simple type used to validate the attribute.
func (d AttributeDeclRead) TypeID() SimpleTypeID {
	return d.typ
}

// FixedValue returns the fixed value, if present.
func (d AttributeDeclRead) FixedValue() (ValueConstraintRead, bool) {
	return d.fixed, d.hasFixed
}

// validateAttributeUseSetRecord validates the uses and index of an
// attribute-use set directly from runtime records. The publication audit
// checks the set's wildcard provenance separately, once per set.
func validateAttributeUseSetRecord(names *nameTable, rt *schemaBuild, set attributeUseSet) error {
	if len(set.Index) != len(set.Uses) {
		return errors.New("attribute use set index size does not match uses")
	}
	audit := attributeUseSetAudit{names: names, rt: rt, set: set}
	for i, use := range set.Uses {
		if err := audit.validateUse(i, use); err != nil {
			return err
		}
	}
	return nil
}

type attributeUseSetAudit struct {
	names   *nameTable
	rt      *schemaBuild
	set     attributeUseSet
	idAttrs int
}

func (a *attributeUseSetAudit) validateUse(index int, use attributeUse) error {
	carriesID, err := validateAttributeUseRuntime(a.names, a.rt, a.set.Index, index, use)
	if err != nil || !carriesID {
		return err
	}
	if use.ValueConstraint != nil {
		return errors.New("ID-typed attribute use stores value constraint")
	}
	a.idAttrs++
	if a.idAttrs > 1 {
		return errors.New("attribute use set stores multiple ID attributes")
	}
	return nil
}

// validateAttributeUseRestriction validates one derived attribute use against
// its base use. Attached constraints remain immutable during derivation calls.
func validateAttributeUseRestriction(
	rt *schemaBuild,
	base, derived attributeUse,
) error {
	if base.Required && !derived.Required {
		return errors.New("required attribute cannot become optional by restriction")
	}
	typeDerived := derived.Type == base.Type
	var err error
	if typeDerived {
		err = rt.spendTypeDerivationWork(1)
	} else {
		_, typeDerived, err = deriveTypeMask(rt, SimpleRef(derived.Type), SimpleRef(base.Type), rt.spendTypeDerivationWork)
	}
	if err != nil {
		return err
	}
	if !typeDerived {
		return errors.New("restricted attribute type is not derived from base")
	}
	if baseFixed := base.ValueConstraint.fixedConstraint(); baseFixed != nil &&
		!fixedValueConstraintEqual(newValueConstraintIdentity(baseFixed), newValueConstraintIdentity(derived.ValueConstraint.fixedConstraint())) {
		return errors.New("fixed attribute constraint must be preserved by restriction")
	}
	return nil
}

// validateAttributeUseSetRestriction validates pairwise use restrictions,
// new-use wildcard allowance and wildcard provenance.
func validateAttributeUseSetRestriction(
	rt *schemaBuild,
	base, derived attributeUseSet,
	binding attributeWildcardBinding,
) error {
	if err := validateBaseAttributeUseRestrictions(rt, base.Uses, derived); err != nil {
		return err
	}
	if err := validateNewRestrictedAttributeUses(rt, base, derived.Uses); err != nil {
		return err
	}
	return validateRestrictedAttributeWildcard(base, derived, binding)
}

// attributeWildcardBinding identifies whether restriction wildcard provenance
// is required for an explicit derivation.
type attributeWildcardBinding uint8

const (
	// attributeWildcardUnbound omits explicit wildcard provenance binding.
	attributeWildcardUnbound attributeWildcardBinding = iota
	// attributeWildcardBound requires explicit wildcard provenance binding.
	attributeWildcardBound
)

func validateBaseAttributeUseRestrictions(
	rt *schemaBuild,
	base []attributeUse,
	derived attributeUseSet,
) error {
	for _, use := range base {
		next, ok := derived.useNamed(use.Name)
		if use.Required && !ok {
			return errors.New("complex restriction omits required base attribute")
		}
		if ok {
			if err := validateAttributeUseRestriction(rt, use, next); err != nil {
				return wrapUnlessLimit(err, "complex restriction attribute use is invalid")
			}
		}
	}
	return nil
}

func validateNewRestrictedAttributeUses(
	rt *schemaBuild,
	base attributeUseSet,
	derived []attributeUse,
) error {
	for _, use := range derived {
		if _, ok := base.useNamed(use.Name); ok {
			continue
		}
		if base.Wildcard == NoWildcard {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
		wildcard, ok := wildcardByID(rt.Wildcards, base.Wildcard)
		if !ok || !wildcardAllowsNamespace(wildcard, use.Name.Namespace) {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
	}
	return nil
}

func validateRestrictedAttributeWildcard(
	base, derived attributeUseSet,
	binding attributeWildcardBinding,
) error {
	switch binding {
	case attributeWildcardUnbound:
		if derived.WildcardDerive != attributeWildcardNone {
			return errors.New("implicit complex type stores derived attribute wildcard provenance")
		}
		return nil
	case attributeWildcardBound:
		return validateAttributeWildcardDerivation(base, derived, attributeWildcardRestriction)
	default:
		return newBuildDefect("attribute wildcard binding is invalid")
	}
}

// The index is not trusted here: the record audit validates it separately.
func (s attributeUseSet) useNamed(name QName) (attributeUse, bool) {
	slot, ok := s.Index[name]
	if !ok || !validUint32Index(slot, len(s.Uses)) || s.Uses[slot].Name != name {
		return attributeUse{}, false
	}
	return s.Uses[slot], true
}

// validateAttributeUseSetExtension validates that every base attribute use is
// preserved unchanged by a complex-type extension.
func validateAttributeUseSetExtension(base, derived attributeUseSet) error {
	for _, use := range base.Uses {
		next, ok := derived.useNamed(use.Name)
		if !ok || !attributeUseExtensionEqual(use, next) {
			return errors.New("complex extension does not preserve base attribute use")
		}
	}
	return nil
}

func attributeUseExtensionEqual(a, b attributeUse) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		a.ValueConstraint.kind() == b.ValueConstraint.kind() &&
		valueConstraintIdentityEqual(newValueConstraintIdentity(a.ValueConstraint), newValueConstraintIdentity(b.ValueConstraint)) &&
		a.Required == b.Required
}

func validateAttributeUseRuntime(
	names *nameTable,
	rt *schemaBuild,
	index map[QName]uint32,
	i int,
	use attributeUse,
) (carriesID bool, err error) {
	carriesID, ok := rt.SimpleTypeCarriesID(use.Type)
	if names == nil || !names.ValidQName(use.Name) || !ok {
		return false, newBuildDefect("attribute use references invalid name or type")
	}
	slot, ok := newUint32Index(i)
	if !ok {
		return false, newBuildDefect("attribute use slot is invalid")
	}
	if indexed, ok := index[use.Name]; !ok || indexed != slot {
		return false, errors.New("attribute use index does not match use slice")
	}
	return carriesID, nil
}

// validateAttributeWildcardDerivation validates a derived use-set wildcard's
// binding to its owning type's base and derivation. The provenance the binding
// names is checked with every set by validateAttributeUseSetRuntime.
func validateAttributeWildcardDerivation(
	base, derived attributeUseSet,
	expected attributeWildcardDerivation,
) error {
	if derived.WildcardDerive != expected {
		return errors.New("attribute wildcard derivation does not match owning type")
	}
	if derived.WildcardBase != base.Wildcard {
		return errors.New("attribute wildcard base does not match owning type")
	}
	return nil
}

// validateAttributeWildcardProvenance validates a use-set wildcard against its
// stored base/declared/derivation provenance.
func validateAttributeWildcardProvenance(rt *schemaBuild, state attributeUseSet) error {
	if err := validateAttributeWildcardID(rt, state.Wildcard, "attribute use set references invalid wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.WildcardBase, "attribute use set references invalid base wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.WildcardDeclared, "attribute use set references invalid declared wildcard"); err != nil {
		return err
	}
	if !validAttributeWildcardDerivation(state.WildcardDerive) {
		return newBuildDefect("attribute use set has invalid wildcard derivation")
	}
	switch state.WildcardDerive {
	case attributeWildcardNone:
		if state.WildcardBase != NoWildcard || state.Wildcard != state.WildcardDeclared {
			return errors.New("attribute wildcard does not match declared wildcard")
		}
	case attributeWildcardRestriction:
		return validateAttributeWildcardRestriction(rt, state)
	case attributeWildcardExtension:
		return validateAttributeWildcardExtension(rt, state)
	}
	return nil
}

func validateAttributeWildcardRestriction(rt *schemaBuild, state attributeUseSet) error {
	if state.WildcardDeclared == NoWildcard {
		if state.Wildcard != NoWildcard {
			return errors.New("attribute wildcard restriction stores undeclared wildcard")
		}
		return nil
	}
	if state.WildcardBase == NoWildcard {
		return errors.New("attribute wildcard restriction has no base wildcard")
	}
	declared, ok := wildcardByID(rt.Wildcards, state.WildcardDeclared)
	if !ok {
		return newBuildDefect("attribute use set references invalid declared wildcard")
	}
	base, ok := wildcardByID(rt.Wildcards, state.WildcardBase)
	if !ok {
		return newBuildDefect("attribute use set references invalid base wildcard")
	}
	subset, err := wildcardNamespaceSubset(declared, base, admittedWildcardWork)
	if err != nil {
		return err
	}
	if !subset || state.Wildcard != state.WildcardDeclared {
		return errors.New("attribute wildcard restriction does not match provenance")
	}
	return nil
}

func validateAttributeWildcardExtension(rt *schemaBuild, state attributeUseSet) error {
	switch {
	case state.WildcardBase == NoWildcard:
		return validateAttributeWildcardExtensionWithoutBase(state)
	case state.WildcardDeclared == NoWildcard:
		return validateAttributeWildcardExtensionWithoutDeclaration(state)
	default:
		return validateAttributeWildcardExtensionUnion(rt, state)
	}
}

func validateAttributeWildcardExtensionWithoutBase(state attributeUseSet) error {
	if state.Wildcard != state.WildcardDeclared {
		return errors.New("attribute wildcard extension without base does not match declared wildcard")
	}
	return nil
}

func validateAttributeWildcardExtensionWithoutDeclaration(state attributeUseSet) error {
	if state.Wildcard != state.WildcardBase {
		return errors.New("attribute wildcard extension does not inherit base wildcard")
	}
	return nil
}

func validateAttributeWildcardExtensionUnion(rt *schemaBuild, state attributeUseSet) error {
	declared, ok := wildcardByID(rt.Wildcards, state.WildcardDeclared)
	if !ok {
		return newBuildDefect("attribute use set references invalid declared wildcard")
	}
	base, ok := wildcardByID(rt.Wildcards, state.WildcardBase)
	if !ok {
		return newBuildDefect("attribute use set references invalid base wildcard")
	}
	actual, ok := wildcardByID(rt.Wildcards, state.Wildcard)
	if !ok {
		return newBuildDefect("attribute use set references invalid wildcard")
	}
	union, err := unionWildcard(declared, base, declared.Process, admittedWildcardWork)
	if err != nil {
		return fmt.Errorf("attribute wildcard extension cannot be rederived: %w", err)
	}
	equal, err := wildcardNamespaceEqual(actual, union, admittedWildcardWork)
	if err != nil {
		return err
	}
	if !equal || actual.Process != union.Process {
		return errors.New("attribute wildcard extension does not match provenance")
	}
	return nil
}

// admittedWildcardWork charges nothing. Re-deriving a use set's wildcard
// repeats the subset or union comparison that finishRestriction or
// finishExtension charged to MaxSchemaDependencySteps when the set was
// compiled, and the equality check after a union visits no more namespaces
// than the union merged.
func admittedWildcardWork(int) error {
	return nil
}

func validateAttributeWildcardID(rt *schemaBuild, id WildcardID, msg string) error {
	if id == NoWildcard {
		return nil
	}
	if !validWildcardID(id, len(rt.Wildcards)) {
		return errors.New(msg)
	}
	return nil
}
