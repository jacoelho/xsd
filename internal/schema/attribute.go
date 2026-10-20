package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

// AttributeWildcardDerivation identifies how an attribute wildcard was derived.
type AttributeWildcardDerivation uint8

const (
	// AttributeWildcardNone records a locally declared or absent wildcard.
	AttributeWildcardNone AttributeWildcardDerivation = iota
	// AttributeWildcardRestriction records wildcard derivation by restriction.
	AttributeWildcardRestriction
	// AttributeWildcardExtension records wildcard derivation by extension.
	AttributeWildcardExtension
)

// ValidAttributeWildcardDerivation reports whether kind is a known attribute
// wildcard derivation kind.
func ValidAttributeWildcardDerivation(kind AttributeWildcardDerivation) bool {
	switch kind {
	case AttributeWildcardNone, AttributeWildcardRestriction, AttributeWildcardExtension:
		return true
	default:
		return false
	}
}

// AttributeUseSet is the runtime record for one attribute-use set.
type AttributeUseSet struct {
	Index            map[QName]uint32
	Uses             []AttributeUse
	Wildcard         WildcardID
	WildcardBase     WildcardID
	WildcardDeclared WildcardID
	WildcardDerive   AttributeWildcardDerivation
}

// AttributeUse is the construction record for one attribute use. Copies retain
// componentID for set union; publication discards this compiler-local identity.
type AttributeUse struct {
	Default     *ValueConstraint
	Fixed       *ValueConstraint
	Name        QName
	Type        SimpleTypeID
	componentID uint32
	Required    bool
	Prohibited  bool
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

func newAttributeUseSetReads(names *NameTable, sets []AttributeUseSet, simpleTypes []SimpleType) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(sets))
	for i := range sets {
		set := &sets[i]
		uses := make([]AttributeUseRead, len(set.Uses))
		var required, valueConstraints []uint32
		for j := range set.Uses {
			uses[j] = newAttributeUseReadForSimpleTypes(names, set.Uses[j], simpleTypes)
			if set.Uses[j].Required {
				required = append(required, uint32(j))
			}
			if _, applies := uses[j].AbsentValueConstraint(); applies {
				valueConstraints = append(valueConstraints, uint32(j))
			}
		}
		out[i] = AttributeUseSetRead{
			index:            maps.Clone(set.Index),
			uses:             uses,
			required:         slices.Clone(required),
			valueConstraints: slices.Clone(valueConstraints),
			wildcard:         set.Wildcard,
		}
		out[i].singleUse = attributeUseSetReadHasSingleUse(out[i])
	}
	return out
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
	if !ok || !ValidUint32Index(slot, len(s.uses)) || s.uses[slot].name != name {
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
	label                      string
	fixed                      ValueConstraintRead
	defaultValue               ValueConstraintRead
	name                       QName
	typ                        SimpleTypeID
	required                   bool
	hasFixed                   bool
	hasDefault                 bool
	canValidateFixedStringFast bool
}

// newAttributeUseReadForSimpleTypes returns an immutable validation read
// projection for one attribute use using published simple types.
func newAttributeUseReadForSimpleTypes(names *NameTable, use AttributeUse, simpleTypes []SimpleType) AttributeUseRead {
	// Adopted WG correction 4363 makes XSI constraints inapplicable during
	// validation. Keep them on construction records for schema correctness and
	// derivation; publication permits only the four intrinsic XSI declarations.
	if names.Namespace(use.Name.Namespace) == vocab.XSINamespaceURI {
		use.Default = nil
		use.Fixed = nil
	}
	fixed, hasFixed := newValueConstraintReadFromConstraint(use.Fixed)
	def, hasDefault := newValueConstraintReadFromConstraint(use.Default)
	return AttributeUseRead{
		name:                       use.Name,
		typ:                        use.Type,
		label:                      names.Format(use.Name),
		fixed:                      fixed,
		defaultValue:               def,
		required:                   use.Required,
		hasFixed:                   hasFixed,
		hasDefault:                 hasDefault,
		canValidateFixedStringFast: attributeUseFixedStringFastForSimpleTypes(use, simpleTypes),
	}
}

func attributeUseFixedStringFastForSimpleTypes(use AttributeUse, simpleTypes []SimpleType) bool {
	if use.Fixed == nil {
		return false
	}
	st, ok := UsableSimpleType(simpleTypes, use.Type)
	if !ok {
		return false
	}
	// A raw lexical comparison is sound only when the type admits the source
	// spelling unchanged and has no typed identity or constraining facets.
	spec := st.ValueSpec
	facets := st.ValueSpec.Facets
	return spec.Variety == valuepkg.Atomic &&
		spec.Primitive == valuepkg.PrimitiveString &&
		spec.Builtin == valuepkg.BuiltinNone &&
		spec.Identity == valuepkg.IdentityNone &&
		spec.Whitespace == valuepkg.WhitespacePreserve &&
		facets.Present == 0 && facets.Fixed == 0 &&
		len(facets.Enumeration) == 0 && len(facets.Patterns) == 0
}

// Name returns the runtime QName for the use.
func (u AttributeUseRead) Name() QName {
	return u.name
}

// TypeID returns the simple type used to validate the attribute.
func (u AttributeUseRead) TypeID() SimpleTypeID {
	return u.typ
}

// Label returns the formatted attribute name for diagnostics.
func (u AttributeUseRead) Label() string {
	return u.label
}

// Required reports whether the attribute must be present.
func (u AttributeUseRead) Required() bool {
	return u.required
}

// FixedValue returns the fixed value, if present.
func (u AttributeUseRead) FixedValue() (ValueConstraintRead, bool) {
	return u.fixed, u.hasFixed
}

// AbsentValueConstraint returns the fixed/default value applied when the
// attribute is absent.
func (u AttributeUseRead) AbsentValueConstraint() (ValueConstraintRead, bool) {
	if u.hasFixed {
		return u.fixed, true
	}
	if u.hasDefault {
		return u.defaultValue, true
	}
	return ValueConstraintRead{}, false
}

// CanValidateFixedStringFast reports whether validation may compare the raw
// string directly against the fixed value.
func (u AttributeUseRead) CanValidateFixedStringFast() bool {
	return u.canValidateFixedStringFast
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
func newAttributeDeclRead(decl AttributeDecl) AttributeDeclRead {
	fixed, hasFixed := newValueConstraintReadFromConstraint(decl.Fixed)
	return AttributeDeclRead{
		name:     decl.Name,
		typ:      decl.Type,
		fixed:    fixed,
		hasFixed: hasFixed,
	}
}

// newAttributeDeclReadsForDecls returns immutable validation read projections
// for frozen global attribute declarations.
func newAttributeDeclReadsForDecls(decls []AttributeDecl) []AttributeDeclRead {
	out := make([]AttributeDeclRead, len(decls))
	for i := range decls {
		out[i] = newAttributeDeclRead(decls[i])
	}
	return out
}

// attributeDeclReadByID returns the validation read projection for id.
func attributeDeclReadByID(reads []AttributeDeclRead, id AttributeID) (AttributeDeclRead, bool) {
	if !ValidAttributeID(id, len(reads)) {
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

// ValidateAttributeUseSetRecord validates attribute-use set metadata directly
// from runtime records.
func ValidateAttributeUseSetRecord(names *NameTable, rt *schemaBuild, set AttributeUseSet) error {
	if err := ValidateAttributeWildcardProvenance(rt, set); err != nil {
		return err
	}
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
	names   *NameTable
	rt      *schemaBuild
	set     AttributeUseSet
	idAttrs int
}

func (a *attributeUseSetAudit) validateUse(index int, use AttributeUse) error {
	identity, err := validateAttributeUseRuntime(a.names, a.rt, a.set.Index, index, use)
	if err != nil {
		return err
	}
	return a.validateIDUse(identity, use)
}

func (a *attributeUseSetAudit) validateIDUse(identity valuepkg.IdentityKind, use AttributeUse) error {
	if identity != valuepkg.IdentityID {
		return nil
	}
	if use.Default != nil || use.Fixed != nil {
		return errors.New("ID-typed attribute use stores value constraint")
	}
	a.idAttrs++
	if a.idAttrs > 1 {
		return errors.New("attribute use set stores multiple ID attributes")
	}
	return nil
}

// ValidateAttributeUseRestriction validates one derived attribute use against
// its base use. Attached constraints remain immutable during derivation calls.
func ValidateAttributeUseRestriction(
	rt *schemaBuild,
	base, derived AttributeUse,
) error {
	switch {
	case derived.Prohibited && base.Required:
		return errors.New("required attribute cannot be prohibited by restriction")
	case derived.Prohibited:
		return nil
	case base.Required && !derived.Required:
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
	if base.Fixed != nil && !FixedValueConstraintEqual(NewValueConstraintIdentity(base.Fixed), NewValueConstraintIdentity(derived.Fixed)) {
		return errors.New("fixed attribute constraint must be preserved by restriction")
	}
	return nil
}

// ValidateAttributeUseSetRestriction validates pairwise use restrictions,
// new-use wildcard allowance and wildcard provenance.
func ValidateAttributeUseSetRestriction(
	rt *schemaBuild,
	base, derived AttributeUseSet,
	binding AttributeWildcardBinding,
) error {
	if err := validateBaseAttributeUseRestrictions(rt, base.Uses, derived.Uses); err != nil {
		return err
	}
	if err := validateNewRestrictedAttributeUses(rt, base.Uses, derived.Uses, base); err != nil {
		return err
	}
	return validateRestrictedAttributeWildcard(rt, base, derived, binding)
}

// AttributeWildcardBinding identifies whether restriction wildcard provenance
// is required for an explicit derivation.
type AttributeWildcardBinding uint8

const (
	// AttributeWildcardUnbound omits explicit wildcard provenance binding.
	AttributeWildcardUnbound AttributeWildcardBinding = iota
	// AttributeWildcardBound requires explicit wildcard provenance binding.
	AttributeWildcardBound
)

func validateBaseAttributeUseRestrictions(
	rt *schemaBuild,
	base, derived []AttributeUse,
) error {
	for _, use := range base {
		next, ok := attributeUseByName(derived, use.Name)
		if use.Required && !ok {
			return errors.New("complex restriction omits required base attribute")
		}
		if ok {
			if err := ValidateAttributeUseRestriction(rt, use, next); err != nil {
				return fmt.Errorf("complex restriction attribute use is invalid: %w", err)
			}
		}
	}
	return nil
}

func validateNewRestrictedAttributeUses(
	rt *schemaBuild,
	base, derived []AttributeUse,
	baseWildcard AttributeUseSet,
) error {
	for _, use := range derived {
		if _, ok := attributeUseByName(base, use.Name); ok {
			continue
		}
		if baseWildcard.Wildcard == NoWildcard {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
		wildcard, ok := WildcardByID(rt.Wildcards, baseWildcard.Wildcard)
		if !ok || !WildcardAllowsNamespace(wildcard, use.Name.Namespace) {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
	}
	return nil
}

func validateRestrictedAttributeWildcard(
	rt *schemaBuild,
	base, derived AttributeUseSet,
	binding AttributeWildcardBinding,
) error {
	switch binding {
	case AttributeWildcardUnbound:
		if derived.WildcardDerive != AttributeWildcardNone {
			return errors.New("implicit complex type stores derived attribute wildcard provenance")
		}
		return nil
	case AttributeWildcardBound:
		return ValidateAttributeWildcardDerivation(rt, base, derived, AttributeWildcardRestriction)
	default:
		return errors.New("attribute wildcard binding is invalid")
	}
}

func attributeUseByName(uses []AttributeUse, name QName) (AttributeUse, bool) {
	for _, use := range uses {
		if use.Name == name {
			return use, true
		}
	}
	return AttributeUse{}, false
}

// ValidateAttributeUseSetExtension validates that every base attribute use is
// preserved unchanged by a complex-type extension.
func ValidateAttributeUseSetExtension(base, derived AttributeUseSet) error {
	for _, use := range base.Uses {
		next, ok := attributeUseByName(derived.Uses, use.Name)
		if !ok || !attributeUseExtensionEqual(use, next) {
			return errors.New("complex extension does not preserve base attribute use")
		}
	}
	return nil
}

func attributeUseExtensionEqual(a, b AttributeUse) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		ValueConstraintIdentityEqual(NewValueConstraintIdentity(a.Default), NewValueConstraintIdentity(b.Default)) &&
		ValueConstraintIdentityEqual(NewValueConstraintIdentity(a.Fixed), NewValueConstraintIdentity(b.Fixed)) &&
		a.Required == b.Required &&
		a.Prohibited == b.Prohibited
}

func validateAttributeUseRuntime(
	names *NameTable,
	rt *schemaBuild,
	index map[QName]uint32,
	i int,
	use AttributeUse,
) (valuepkg.IdentityKind, error) {
	identity, ok := rt.SimpleTypeIdentity(use.Type)
	if names == nil || !names.ValidQName(use.Name) || !ok {
		return valuepkg.IdentityNone, errors.New("attribute use references invalid name or type")
	}
	slot, ok := uint32Index(i)
	if !ok {
		return valuepkg.IdentityNone, errors.New("attribute use slot is invalid")
	}
	if indexed, ok := index[use.Name]; !ok || indexed != slot {
		return valuepkg.IdentityNone, errors.New("attribute use index does not match use slice")
	}
	if use.Prohibited {
		return valuepkg.IdentityNone, errors.New("attribute use set stores prohibited use")
	}
	if use.Default != nil && use.Fixed != nil {
		return valuepkg.IdentityNone, errors.New("attribute use stores both default and fixed value constraints")
	}
	return identity, nil
}

func uint32Index(i int) (uint32, bool) {
	if i < 0 || uint64(i) > uint64(invalidID) {
		return 0, false
	}
	return uint32(i), true
}

// ValidateAttributeWildcardDerivation validates a derived use-set wildcard's
// binding to its owning type's base and derivation.
func ValidateAttributeWildcardDerivation(
	rt *schemaBuild,
	base, derived AttributeUseSet,
	expected AttributeWildcardDerivation,
) error {
	if derived.WildcardDerive != expected {
		return errors.New("attribute wildcard derivation does not match owning type")
	}
	if derived.WildcardBase != base.Wildcard {
		return errors.New("attribute wildcard base does not match owning type")
	}
	return ValidateAttributeWildcardProvenance(rt, derived)
}

// ValidateAttributeWildcardProvenance validates a use-set wildcard against its
// stored base/declared/derivation provenance.
func ValidateAttributeWildcardProvenance(rt *schemaBuild, state AttributeUseSet) error {
	if err := validateAttributeWildcardID(rt, state.Wildcard, "attribute use set references invalid wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.WildcardBase, "attribute use set references invalid base wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.WildcardDeclared, "attribute use set references invalid declared wildcard"); err != nil {
		return err
	}
	if !ValidAttributeWildcardDerivation(state.WildcardDerive) {
		return errors.New("attribute use set has invalid wildcard derivation")
	}
	switch state.WildcardDerive {
	case AttributeWildcardNone:
		if state.WildcardBase != NoWildcard || state.Wildcard != state.WildcardDeclared {
			return errors.New("attribute wildcard does not match declared wildcard")
		}
	case AttributeWildcardRestriction:
		return validateAttributeWildcardRestriction(rt, state)
	case AttributeWildcardExtension:
		return validateAttributeWildcardExtension(rt, state)
	}
	return nil
}

func validateAttributeWildcardRestriction(rt *schemaBuild, state AttributeUseSet) error {
	if state.WildcardDeclared == NoWildcard {
		if state.Wildcard != NoWildcard {
			return errors.New("attribute wildcard restriction stores undeclared wildcard")
		}
		return nil
	}
	if state.WildcardBase == NoWildcard {
		return errors.New("attribute wildcard restriction has no base wildcard")
	}
	declared, ok := WildcardByID(rt.Wildcards, state.WildcardDeclared)
	if !ok {
		return errors.New("attribute use set references invalid declared wildcard")
	}
	base, ok := WildcardByID(rt.Wildcards, state.WildcardBase)
	if !ok {
		return errors.New("attribute use set references invalid base wildcard")
	}
	if !wildcardNamespaceSubset(declared, base) || state.Wildcard != state.WildcardDeclared {
		return errors.New("attribute wildcard restriction does not match provenance")
	}
	return nil
}

func validateAttributeWildcardExtension(rt *schemaBuild, state AttributeUseSet) error {
	switch {
	case state.WildcardBase == NoWildcard:
		return validateAttributeWildcardExtensionWithoutBase(state)
	case state.WildcardDeclared == NoWildcard:
		return validateAttributeWildcardExtensionWithoutDeclaration(state)
	default:
		return validateAttributeWildcardExtensionUnion(rt, state)
	}
}

func validateAttributeWildcardExtensionWithoutBase(state AttributeUseSet) error {
	if state.Wildcard != state.WildcardDeclared {
		return errors.New("attribute wildcard extension without base does not match declared wildcard")
	}
	return nil
}

func validateAttributeWildcardExtensionWithoutDeclaration(state AttributeUseSet) error {
	if state.Wildcard != state.WildcardBase {
		return errors.New("attribute wildcard extension does not inherit base wildcard")
	}
	return nil
}

func validateAttributeWildcardExtensionUnion(rt *schemaBuild, state AttributeUseSet) error {
	declared, ok := WildcardByID(rt.Wildcards, state.WildcardDeclared)
	if !ok {
		return errors.New("attribute use set references invalid declared wildcard")
	}
	base, ok := WildcardByID(rt.Wildcards, state.WildcardBase)
	if !ok {
		return errors.New("attribute use set references invalid base wildcard")
	}
	actual, ok := WildcardByID(rt.Wildcards, state.Wildcard)
	if !ok {
		return errors.New("attribute use set references invalid wildcard")
	}
	union, err := UnionWildcard(declared, base, declared.Process)
	if err != nil {
		return errors.New("attribute wildcard extension cannot be rederived")
	}
	if !wildcardsEqual(actual, union) {
		return errors.New("attribute wildcard extension does not match provenance")
	}
	return nil
}

func validateAttributeWildcardID(rt *schemaBuild, id WildcardID, msg string) error {
	if id == NoWildcard {
		return nil
	}
	if _, ok := WildcardByID(rt.Wildcards, id); !ok {
		return errors.New(msg)
	}
	return nil
}

func wildcardsEqual(a, b Wildcard) bool {
	return a.Mode == b.Mode &&
		a.Process == b.Process &&
		a.OtherThan == b.OtherThan &&
		slices.Equal(a.Namespaces, b.Namespaces)
}
