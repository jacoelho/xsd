package runtime

import (
	"errors"
	"maps"
	"slices"
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

// AttributeWildcardState stores a use-set's wildcard provenance. Absent
// wildcard references must be NoWildcard; WildcardID(0) is a valid runtime ID.
type AttributeWildcardState struct {
	Wildcard   WildcardID
	Base       WildcardID
	Declared   WildcardID
	Derivation AttributeWildcardDerivation
}

// AttributeUseSet is the runtime record for one attribute-use set.
type AttributeUseSet struct {
	Index            map[QName]uint32
	Uses             []AttributeUse
	Required         []uint32
	ValueConstraints []uint32
	Wildcard         WildcardID
	WildcardBase     WildcardID
	WildcardDeclared WildcardID
	WildcardDerive   AttributeWildcardDerivation
}

// AttributeUse is the runtime record for one attribute use.
type AttributeUse struct {
	Default    *ValueConstraint
	Fixed      *ValueConstraint
	Name       QName
	Type       SimpleTypeID
	Required   bool
	Prohibited bool
}

// AttributeUseValidation is the runtime projection needed to validate
// attribute-use metadata.
type AttributeUseValidation struct {
	Name       QName
	Type       SimpleTypeID
	Required   bool
	Prohibited bool
	HasDefault bool
	HasFixed   bool
}

// AttributeUseSetValidation is the runtime projection needed to validate an
// attribute-use set.
type AttributeUseSetValidation struct {
	Index            map[QName]uint32
	Uses             []AttributeUseValidation
	Required         []uint32
	ValueConstraints []uint32
	Wildcard         AttributeWildcardState
}

// NewAttributeWildcardStateForUseSet projects wildcard provenance from an
// attribute-use set.
func NewAttributeWildcardStateForUseSet(set AttributeUseSet) AttributeWildcardState {
	return AttributeWildcardState{
		Wildcard:   set.Wildcard,
		Base:       set.WildcardBase,
		Declared:   set.WildcardDeclared,
		Derivation: set.WildcardDerive,
	}
}

// NewAttributeUseValidationForUse projects one runtime attribute use into the
// shape needed for attribute-use-set invariant validation.
func NewAttributeUseValidationForUse(use AttributeUse) AttributeUseValidation {
	return AttributeUseValidation{
		Name:       use.Name,
		Type:       use.Type,
		Required:   use.Required,
		Prohibited: use.Prohibited,
		HasDefault: use.Default != nil,
		HasFixed:   use.Fixed != nil,
	}
}

// NewAttributeUseSetValidationForUseSet projects one runtime attribute-use set
// into the shape needed for invariant validation.
func NewAttributeUseSetValidationForUseSet(set AttributeUseSet) AttributeUseSetValidation {
	uses := make([]AttributeUseValidation, len(set.Uses))
	for i, use := range set.Uses {
		uses[i] = NewAttributeUseValidationForUse(use)
	}
	return CloneAttributeUseSetValidation(AttributeUseSetValidation{
		Index:            set.Index,
		Uses:             uses,
		Required:         set.Required,
		ValueConstraints: set.ValueConstraints,
		Wildcard:         NewAttributeWildcardStateForUseSet(set),
	})
}

// AttributeUseSetReadShape adapts a frozen attribute-use set to the
// validation-facing runtime read API.
type AttributeUseSetReadShape struct {
	Index            map[QName]uint32
	Uses             []AttributeUseReadShape
	Required         []uint32
	ValueConstraints []uint32
	Wildcard         WildcardID
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

// NewAttributeUseSetReadForTypeReads returns the immutable validation read
// projection for one attribute-use set using published simple-value type reads.
func NewAttributeUseSetReadForTypeReads(shape AttributeUseSetReadShape, simpleValueTypes []SimpleValueTypeRead) AttributeUseSetRead {
	out := AttributeUseSetRead{
		index:            maps.Clone(shape.Index),
		uses:             NewAttributeUseReadsForTypeReads(shape.Uses, simpleValueTypes),
		required:         slices.Clone(shape.Required),
		valueConstraints: slices.Clone(shape.ValueConstraints),
		wildcard:         shape.Wildcard,
	}
	out.singleUse = attributeUseSetReadHasSingleUse(out)
	return out
}

// NewAttributeUseSetReadForSimpleTypes returns the immutable validation read
// projection for one attribute-use set using published simple types.
func NewAttributeUseSetReadForSimpleTypes(shape AttributeUseSetReadShape, simpleTypes []SimpleType) AttributeUseSetRead {
	out := AttributeUseSetRead{
		index:            maps.Clone(shape.Index),
		uses:             NewAttributeUseReadsForSimpleTypes(shape.Uses, simpleTypes),
		required:         slices.Clone(shape.Required),
		valueConstraints: slices.Clone(shape.ValueConstraints),
		wildcard:         shape.Wildcard,
	}
	out.singleUse = attributeUseSetReadHasSingleUse(out)
	return out
}

func attributeUseSetReadHasSingleUse(s AttributeUseSetRead) bool {
	if len(s.uses) != 1 || len(s.index) != 1 {
		return false
	}
	slot, ok := s.index[s.uses[0].name]
	return ok && slot == 0
}

// NewAttributeUseSetReadsForTypeReads returns immutable validation read
// projections for attribute-use sets using published simple-value type reads.
func NewAttributeUseSetReadsForTypeReads(shapes []AttributeUseSetReadShape, simpleValueTypes []SimpleValueTypeRead) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(shapes))
	for i := range shapes {
		out[i] = NewAttributeUseSetReadForTypeReads(shapes[i], simpleValueTypes)
	}
	return out
}

// NewAttributeUseSetReadsForSimpleTypes returns immutable validation read
// projections for attribute-use sets using published simple types.
func NewAttributeUseSetReadsForSimpleTypes(shapes []AttributeUseSetReadShape, simpleTypes []SimpleType) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(shapes))
	for i := range shapes {
		out[i] = NewAttributeUseSetReadForSimpleTypes(shapes[i], simpleTypes)
	}
	return out
}

// NewAttributeUseSetReadForSetWithTypeReads returns the immutable validation
// read projection for one frozen attribute-use set using published
// simple-value type reads.
func NewAttributeUseSetReadForSetWithTypeReads(names *NameTable, set AttributeUseSet, simpleValueTypes []SimpleValueTypeRead) AttributeUseSetRead {
	return NewAttributeUseSetReadForTypeReads(attributeUseSetReadShapeForSet(names, set), simpleValueTypes)
}

// NewAttributeUseSetReadForSetWithSimpleTypes returns the immutable validation
// read projection for one frozen attribute-use set using published simple types.
func NewAttributeUseSetReadForSetWithSimpleTypes(names *NameTable, set AttributeUseSet, simpleTypes []SimpleType) AttributeUseSetRead {
	return NewAttributeUseSetReadForSimpleTypes(attributeUseSetReadShapeForSet(names, set), simpleTypes)
}

// NewAttributeUseSetReadsForSetsWithTypeReads returns immutable validation read
// projections for frozen attribute-use sets using published simple-value type
// reads.
func NewAttributeUseSetReadsForSetsWithTypeReads(names *NameTable, sets []AttributeUseSet, simpleValueTypes []SimpleValueTypeRead) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(sets))
	for i := range sets {
		out[i] = NewAttributeUseSetReadForSetWithTypeReads(names, sets[i], simpleValueTypes)
	}
	return out
}

// NewAttributeUseSetReadsForSetsWithSimpleTypes returns immutable validation
// read projections for frozen attribute-use sets using published simple types.
func NewAttributeUseSetReadsForSetsWithSimpleTypes(names *NameTable, sets []AttributeUseSet, simpleTypes []SimpleType) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(sets))
	for i := range sets {
		out[i] = NewAttributeUseSetReadForSetWithSimpleTypes(names, sets[i], simpleTypes)
	}
	return out
}

func moveAttributeUseSetReads(names *NameTable, sets []AttributeUseSet, simpleTypes []SimpleType) []AttributeUseSetRead {
	out := make([]AttributeUseSetRead, len(sets))
	for i := range sets {
		set := &sets[i]
		uses := make([]AttributeUseRead, len(set.Uses))
		for j := range set.Uses {
			uses[j] = NewAttributeUseReadForSimpleTypes(attributeUseReadShapeForUse(names, set.Uses[j]), simpleTypes)
		}
		out[i] = AttributeUseSetRead{
			index:            set.Index,
			uses:             uses,
			required:         set.Required,
			valueConstraints: set.ValueConstraints,
			wildcard:         set.Wildcard,
		}
		out[i].singleUse = attributeUseSetReadHasSingleUse(out[i])
	}
	return out
}

func attributeUseSetReadShapeForSet(names *NameTable, set AttributeUseSet) AttributeUseSetReadShape {
	uses := make([]AttributeUseReadShape, len(set.Uses))
	for i := range set.Uses {
		uses[i] = attributeUseReadShapeForUse(names, set.Uses[i])
	}
	return AttributeUseSetReadShape{
		Index:            set.Index,
		Uses:             uses,
		Required:         set.Required,
		ValueConstraints: set.ValueConstraints,
		Wildcard:         set.Wildcard,
	}
}

func attributeUseReadShapeForUse(names *NameTable, use AttributeUse) AttributeUseReadShape {
	fixed, hasFixed := NewValueConstraintReadFromConstraint(use.Fixed)
	def, hasDefault := NewValueConstraintReadFromConstraint(use.Default)
	return AttributeUseReadShape{
		Name:       use.Name,
		Type:       use.Type,
		Label:      names.Format(use.Name),
		Fixed:      fixed,
		Default:    def,
		Required:   use.Required,
		HasFixed:   hasFixed,
		HasDefault: hasDefault,
	}
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

// AttributeUseSetReadUseAtPtr returns the declared use stored at slot without copying it.
func AttributeUseSetReadUseAtPtr(s *AttributeUseSetRead, slot int) (*AttributeUseRead, bool) {
	if s == nil || slot < 0 || slot >= len(s.uses) {
		return nil, false
	}
	return &s.uses[slot], true
}

// AttributeUseSetReadDeclaredUsePtr returns the declared use matching name without copying it.
func AttributeUseSetReadDeclaredUsePtr(s *AttributeUseSetRead, name QName) (*AttributeUseRead, int, bool) {
	if s == nil {
		return nil, -1, false
	}
	if s.singleUse {
		if s.uses[0].name == name {
			return &s.uses[0], 0, true
		}
		return nil, -1, false
	}
	slot, ok := s.index[name]
	if !ok || !ValidUint32Index(slot, len(s.uses)) || s.uses[slot].name != name {
		return nil, -1, false
	}
	return &s.uses[slot], int(slot), true
}

// ForEachRequiredUse calls fn for each required-use slot.
func (s AttributeUseSetRead) ForEachRequiredUse(fn func(slot int, use AttributeUseRead) error) error {
	return ForEachRequiredAttributeUseSlot(s.required, len(s.uses), func(slot uint32) error {
		return fn(int(slot), s.uses[slot])
	})
}

// ForEachValueConstraintUse calls fn for each absent value-constraint slot.
func (s AttributeUseSetRead) ForEachValueConstraintUse(fn func(slot int, use AttributeUseRead) error) error {
	return ForEachValueConstraintAttributeUseSlot(s.valueConstraints, len(s.uses), func(slot uint32) error {
		return fn(int(slot), s.uses[slot])
	})
}

// Wildcard returns the attribute wildcard attached to the set.
func (s AttributeUseSetRead) Wildcard() WildcardID {
	return s.wildcard
}

// RequiredSlots returns required-use slots for direct runtime validation.
func (s AttributeUseSetRead) RequiredSlots() []uint32 {
	return s.required
}

// ValueConstraintSlots returns default/fixed-use slots for direct runtime validation.
func (s AttributeUseSetRead) ValueConstraintSlots() []uint32 {
	return s.valueConstraints
}

// EqualAttributeUseSetReads reports whether two attribute-use-set read
// projections expose the same validation-facing set.
func EqualAttributeUseSetReads(a, b AttributeUseSetRead) bool {
	return maps.Equal(a.index, b.index) &&
		slices.EqualFunc(a.uses, b.uses, EqualAttributeUseReads) &&
		slices.Equal(a.required, b.required) &&
		slices.Equal(a.valueConstraints, b.valueConstraints) &&
		a.wildcard == b.wildcard &&
		a.singleUse == b.singleUse
}

// EqualAttributeUseSetReadProjectionForSetsWithTypeReads reports whether reads
// expose the same validation-facing attribute-use sets as frozen runtime
// records using published simple-value type reads.
func EqualAttributeUseSetReadProjectionForSetsWithTypeReads(reads []AttributeUseSetRead, names *NameTable, sets []AttributeUseSet, simpleValueTypes []SimpleValueTypeRead) bool {
	if len(reads) != len(sets) {
		return false
	}
	for i := range reads {
		if !equalAttributeUseSetReadForSet(reads[i], names, sets[i], simpleValueTypes, nil) {
			return false
		}
	}
	return true
}

// EqualAttributeUseSetReadProjectionForSetsWithSimpleTypes reports whether reads
// expose the same validation-facing attribute-use sets as frozen runtime records
// using published simple types.
func EqualAttributeUseSetReadProjectionForSetsWithSimpleTypes(reads []AttributeUseSetRead, names *NameTable, sets []AttributeUseSet, simpleTypes []SimpleType) bool {
	if len(reads) != len(sets) {
		return false
	}
	for i := range reads {
		if !equalAttributeUseSetReadForSet(reads[i], names, sets[i], nil, simpleTypes) {
			return false
		}
	}
	return true
}

func equalAttributeUseSetReadForSet(
	read AttributeUseSetRead,
	names *NameTable,
	set AttributeUseSet,
	typeReads []SimpleValueTypeRead,
	simpleTypes []SimpleType,
) bool {
	if !maps.Equal(read.index, set.Index) ||
		!slices.Equal(read.required, set.Required) ||
		!slices.Equal(read.valueConstraints, set.ValueConstraints) ||
		read.wildcard != set.Wildcard ||
		len(read.uses) != len(set.Uses) {
		return false
	}
	expectedSingleUse := false
	if len(set.Uses) == 1 && len(set.Index) == 1 {
		slot, ok := set.Index[set.Uses[0].Name]
		expectedSingleUse = ok && slot == 0
	}
	if read.singleUse != expectedSingleUse {
		return false
	}
	for i := range set.Uses {
		if !equalAttributeUseReadForUse(read.uses[i], names, set.Uses[i], typeReads, simpleTypes) {
			return false
		}
	}
	return true
}

func equalAttributeUseReadForUse(
	read AttributeUseRead,
	names *NameTable,
	use AttributeUse,
	typeReads []SimpleValueTypeRead,
	simpleTypes []SimpleType,
) bool {
	hasFixed := use.Fixed != nil
	hasDefault := use.Default != nil
	if read.name != use.Name || read.typ != use.Type || read.required != use.Required ||
		read.hasFixed != hasFixed || read.hasDefault != hasDefault ||
		!formattedQNameEqual(names, use.Name, read.label) {
		return false
	}
	if hasFixed {
		fixed, _ := NewValueConstraintReadFromConstraint(use.Fixed)
		if !EqualValueConstraintReads(read.fixed, fixed) {
			return false
		}
	}
	if hasDefault {
		def, _ := NewValueConstraintReadFromConstraint(use.Default)
		if !EqualValueConstraintReads(read.defaultValue, def) {
			return false
		}
	}
	var fixedFast bool
	if typeReads != nil {
		shape := AttributeUseReadShape{Type: use.Type, HasFixed: hasFixed}
		fixedFast = attributeUseFixedStringFastForTypeReads(shape, typeReads)
	} else {
		shape := AttributeUseReadShape{Type: use.Type, HasFixed: hasFixed}
		fixedFast = attributeUseFixedStringFastForSimpleTypes(shape, simpleTypes)
	}
	return read.canValidateFixedStringFast == fixedFast
}

func formattedQNameEqual(names *NameTable, name QName, formatted string) bool {
	ns := names.Namespace(name.Namespace)
	local := names.Local(name.Local)
	if ns == "" {
		return formatted == local
	}
	return len(formatted) == len(ns)+len(local)+2 &&
		formatted[0] == '{' &&
		formatted[1:len(ns)+1] == ns &&
		formatted[len(ns)+1] == '}' &&
		formatted[len(ns)+2:] == local
}

// ValidateAttributeUseSetReadProjectionForSetsWithTypeReads validates
// attribute-use-set reads against frozen runtime records using published
// simple-value type reads.
func ValidateAttributeUseSetReadProjectionForSetsWithTypeReads(reads []AttributeUseSetRead, names *NameTable, sets []AttributeUseSet, simpleValueTypes []SimpleValueTypeRead) error {
	if len(reads) != len(sets) {
		return errors.New("attribute use set read projection count does not match use sets")
	}
	if !EqualAttributeUseSetReadProjectionForSetsWithTypeReads(reads, names, sets, simpleValueTypes) {
		return errors.New("attribute use read projection does not match use set")
	}
	return nil
}

// ValidateAttributeUseSetReadProjectionForSetsWithSimpleTypes validates
// attribute-use-set reads against frozen runtime records using published simple
// types.
func ValidateAttributeUseSetReadProjectionForSetsWithSimpleTypes(reads []AttributeUseSetRead, names *NameTable, sets []AttributeUseSet, simpleTypes []SimpleType) error {
	if len(reads) != len(sets) {
		return errors.New("attribute use set read projection count does not match use sets")
	}
	if !EqualAttributeUseSetReadProjectionForSetsWithSimpleTypes(reads, names, sets, simpleTypes) {
		return errors.New("attribute use read projection does not match use set")
	}
	return nil
}

// AttributeUseSetReadForComplexType returns the attribute-use-set read
// projection for a complex type.
func AttributeUseSetReadForComplexType(typeUseSetIDs []AttributeUseSetID, reads []AttributeUseSetRead, id ComplexTypeID) (AttributeUseSetRead, bool) {
	if !ValidComplexTypeID(id, len(typeUseSetIDs)) {
		return AttributeUseSetRead{}, false
	}
	setID := typeUseSetIDs[id]
	if !ValidAttributeUseSetID(setID, len(reads)) {
		return AttributeUseSetRead{}, false
	}
	return reads[setID], true
}

// AttributeUseSetReadForComplexTypePtr returns the attribute-use-set read
// projection for a complex type without copying it.
func AttributeUseSetReadForComplexTypePtr(typeUseSetIDs []AttributeUseSetID, reads []AttributeUseSetRead, id ComplexTypeID) (*AttributeUseSetRead, bool) {
	if !ValidComplexTypeID(id, len(typeUseSetIDs)) {
		return nil, false
	}
	setID := typeUseSetIDs[id]
	if !ValidAttributeUseSetID(setID, len(reads)) {
		return nil, false
	}
	return &reads[setID], true
}

// AttributeUseSetReadByType returns the attribute-use-set read projection for
// a runtime type. The booleans report complex-type presence and metadata
// validity, respectively.
func AttributeUseSetReadByType(typeUseSetIDs []AttributeUseSetID, reads []AttributeUseSetRead, typ TypeID) (AttributeUseSetRead, bool, bool) {
	id, ok := typ.Complex()
	if !ok {
		return AttributeUseSetRead{}, false, true
	}
	set, ok := AttributeUseSetReadForComplexType(typeUseSetIDs, reads, id)
	return set, true, ok
}

// AttributeUseSetReadByTypePtr returns the attribute-use-set read projection
// for a runtime type without copying it. The booleans report complex-type
// presence and metadata validity, respectively.
func AttributeUseSetReadByTypePtr(typeUseSetIDs []AttributeUseSetID, reads []AttributeUseSetRead, typ TypeID) (*AttributeUseSetRead, bool, bool) {
	id, ok := typ.Complex()
	if !ok {
		return nil, false, true
	}
	set, ok := AttributeUseSetReadForComplexTypePtr(typeUseSetIDs, reads, id)
	return set, true, ok
}

// AttributeUseReadShape is the runtime-read projection for one attribute use.
type AttributeUseReadShape struct {
	Label      string
	Fixed      ValueConstraintRead
	Default    ValueConstraintRead
	Name       QName
	Type       SimpleTypeID
	Required   bool
	HasFixed   bool
	HasDefault bool
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

// NewAttributeUseReadForTypeReads returns an immutable validation read
// projection for one attribute use using published simple-value type reads.
func NewAttributeUseReadForTypeReads(shape AttributeUseReadShape, simpleValueTypes []SimpleValueTypeRead) AttributeUseRead {
	return AttributeUseRead{
		name:                       shape.Name,
		typ:                        shape.Type,
		label:                      shape.Label,
		fixed:                      shape.Fixed,
		defaultValue:               shape.Default,
		required:                   shape.Required,
		hasFixed:                   shape.HasFixed,
		hasDefault:                 shape.HasDefault,
		canValidateFixedStringFast: attributeUseFixedStringFastForTypeReads(shape, simpleValueTypes),
	}
}

// NewAttributeUseReadForSimpleTypes returns an immutable validation read
// projection for one attribute use using published simple types.
func NewAttributeUseReadForSimpleTypes(shape AttributeUseReadShape, simpleTypes []SimpleType) AttributeUseRead {
	return AttributeUseRead{
		name:                       shape.Name,
		typ:                        shape.Type,
		label:                      shape.Label,
		fixed:                      shape.Fixed,
		defaultValue:               shape.Default,
		required:                   shape.Required,
		hasFixed:                   shape.HasFixed,
		hasDefault:                 shape.HasDefault,
		canValidateFixedStringFast: attributeUseFixedStringFastForSimpleTypes(shape, simpleTypes),
	}
}

// NewAttributeUseReadsForTypeReads returns immutable validation read
// projections for attribute uses using published simple-value type reads.
func NewAttributeUseReadsForTypeReads(shapes []AttributeUseReadShape, simpleValueTypes []SimpleValueTypeRead) []AttributeUseRead {
	out := make([]AttributeUseRead, len(shapes))
	for i := range shapes {
		out[i] = NewAttributeUseReadForTypeReads(shapes[i], simpleValueTypes)
	}
	return out
}

// NewAttributeUseReadsForSimpleTypes returns immutable validation read
// projections for attribute uses using published simple types.
func NewAttributeUseReadsForSimpleTypes(shapes []AttributeUseReadShape, simpleTypes []SimpleType) []AttributeUseRead {
	out := make([]AttributeUseRead, len(shapes))
	for i := range shapes {
		out[i] = NewAttributeUseReadForSimpleTypes(shapes[i], simpleTypes)
	}
	return out
}

func attributeUseFixedStringFastForTypeReads(shape AttributeUseReadShape, simpleValueTypes []SimpleValueTypeRead) bool {
	if !shape.HasFixed {
		return false
	}
	read, ok := simpleValueTypeReadByID(simpleValueTypes, shape.Type)
	if !ok {
		return false
	}
	return SimpleFixedStringFastPathForType(SimpleFixedStringTypeShape{
		Type:     read.Type,
		HasFixed: shape.HasFixed,
	})
}

func attributeUseFixedStringFastForSimpleTypes(shape AttributeUseReadShape, simpleTypes []SimpleType) bool {
	if !shape.HasFixed {
		return false
	}
	st, ok := UsableSimpleType(simpleTypes, shape.Type)
	if !ok {
		return false
	}
	return SimpleFixedStringFastPath(SimpleFixedStringFastPathShape{
		Bypass: SimpleValueBypass(SimpleValueBypassShape{
			Facets:    st.Facets.Present,
			Variety:   st.Variety,
			Primitive: st.Primitive,
			Builtin:   st.Builtin,
			Identity:  st.Identity,
			Fast:      st.Fast,
		}),
		Whitespace: st.Whitespace,
		HasFixed:   shape.HasFixed,
	})
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
	return AbsentValueConstraint(u.fixed, u.hasFixed, u.defaultValue, u.hasDefault)
}

// CanValidateFixedStringFast reports whether validation may compare the raw
// string directly against the fixed value.
func (u AttributeUseRead) CanValidateFixedStringFast() bool {
	return u.canValidateFixedStringFast
}

// EqualAttributeUseReads reports whether two attribute-use read projections
// expose the same validation-facing use.
func EqualAttributeUseReads(a, b AttributeUseRead) bool {
	if a.name != b.name ||
		a.typ != b.typ ||
		a.label != b.label ||
		a.required != b.required ||
		a.hasFixed != b.hasFixed ||
		a.hasDefault != b.hasDefault ||
		a.canValidateFixedStringFast != b.canValidateFixedStringFast {
		return false
	}
	if a.hasFixed && !EqualValueConstraintReads(a.fixed, b.fixed) {
		return false
	}
	if a.hasDefault && !EqualValueConstraintReads(a.defaultValue, b.defaultValue) {
		return false
	}
	return true
}

// AttributeDeclReadShape is the runtime-read projection for one global
// attribute declaration.
type AttributeDeclReadShape struct {
	Fixed    ValueConstraintRead
	Name     QName
	Type     SimpleTypeID
	HasFixed bool
}

// AttributeDeclRead exposes validation-facing facts for one global attribute
// declaration without exposing compiler-owned storage.
type AttributeDeclRead struct {
	fixed    ValueConstraintRead
	name     QName
	typ      SimpleTypeID
	hasFixed bool
}

// NewAttributeDeclRead returns an immutable validation read projection for one
// global attribute declaration.
func NewAttributeDeclRead(shape AttributeDeclReadShape) AttributeDeclRead {
	return AttributeDeclRead{
		name:     shape.Name,
		typ:      shape.Type,
		fixed:    shape.Fixed,
		hasFixed: shape.HasFixed,
	}
}

// NewAttributeDeclReads returns immutable validation read projections for
// global attribute declarations.
func NewAttributeDeclReads(shapes []AttributeDeclReadShape) []AttributeDeclRead {
	out := make([]AttributeDeclRead, len(shapes))
	for i := range shapes {
		out[i] = NewAttributeDeclRead(shapes[i])
	}
	return out
}

// NewAttributeDeclReadForDecl returns an immutable validation read projection
// for one frozen global attribute declaration.
func NewAttributeDeclReadForDecl(decl AttributeDecl) AttributeDeclRead {
	return NewAttributeDeclRead(attributeDeclReadShapeForDecl(decl))
}

// NewAttributeDeclReadsForDecls returns immutable validation read projections
// for frozen global attribute declarations.
func NewAttributeDeclReadsForDecls(decls []AttributeDecl) []AttributeDeclRead {
	out := make([]AttributeDeclRead, len(decls))
	for i := range decls {
		out[i] = NewAttributeDeclReadForDecl(decls[i])
	}
	return out
}

// AttributeDeclReadByID returns the validation read projection for id.
func AttributeDeclReadByID(reads []AttributeDeclRead, id AttributeID) (AttributeDeclRead, bool) {
	if !ValidAttributeID(id, len(reads)) {
		return AttributeDeclRead{}, false
	}
	return reads[id], true
}

func attributeDeclReadShapeForDecl(decl AttributeDecl) AttributeDeclReadShape {
	fixed, hasFixed := NewValueConstraintReadFromConstraint(decl.Fixed)
	return AttributeDeclReadShape{
		Name:     decl.Name,
		Type:     decl.Type,
		Fixed:    fixed,
		HasFixed: hasFixed,
	}
}

// EqualAttributeDeclReads reports whether two attribute declaration read
// projections expose the same validation-facing declaration.
func EqualAttributeDeclReads(a, b AttributeDeclRead) bool {
	if a.name != b.name || a.typ != b.typ || a.hasFixed != b.hasFixed {
		return false
	}
	return !a.hasFixed || EqualValueConstraintReads(a.fixed, b.fixed)
}

// EqualAttributeDeclReadProjection reports whether reads expose the same
// validation-facing declarations as shapes.
func EqualAttributeDeclReadProjection(reads []AttributeDeclRead, shapes []AttributeDeclReadShape) bool {
	if len(reads) != len(shapes) {
		return false
	}
	for i := range reads {
		if !EqualAttributeDeclReads(reads[i], NewAttributeDeclRead(shapes[i])) {
			return false
		}
	}
	return true
}

// EqualAttributeDeclReadProjectionForDecls reports whether reads expose the
// same validation-facing declarations as frozen global attribute declarations.
func EqualAttributeDeclReadProjectionForDecls(reads []AttributeDeclRead, decls []AttributeDecl) bool {
	if len(reads) != len(decls) {
		return false
	}
	for i := range reads {
		if !EqualAttributeDeclReads(reads[i], NewAttributeDeclReadForDecl(decls[i])) {
			return false
		}
	}
	return true
}

// ValidateAttributeDeclReadProjectionForDecls validates global attribute
// declaration read projections against frozen declarations.
func ValidateAttributeDeclReadProjectionForDecls(reads []AttributeDeclRead, decls []AttributeDecl) error {
	if len(reads) != len(decls) {
		return errors.New("attribute declaration read projection count does not match declarations")
	}
	if !EqualAttributeDeclReadProjectionForDecls(reads, decls) {
		return errors.New("attribute declaration read projection does not match declaration")
	}
	return nil
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

// AttributeUseRestrictionValidation is the runtime projection needed to
// validate one restricted attribute use against its base use.
type AttributeUseRestrictionValidation struct {
	Fixed      ValueConstraintIdentity
	Name       QName
	Type       SimpleTypeID
	Required   bool
	Prohibited bool
}

// AttributeUseExtensionValidation is the runtime projection needed to prove
// that complex-type extension preserved inherited attribute uses unchanged.
type AttributeUseExtensionValidation struct {
	Default    ValueConstraintIdentity
	Fixed      ValueConstraintIdentity
	Name       QName
	Type       SimpleTypeID
	Required   bool
	Prohibited bool
}

// NewAttributeUseRestrictionValidationForUse projects one runtime attribute
// use into the shape needed for restriction validation.
func NewAttributeUseRestrictionValidationForUse(use AttributeUse) AttributeUseRestrictionValidation {
	return AttributeUseRestrictionValidation{
		Fixed:      NewValueConstraintIdentity(use.Fixed),
		Name:       use.Name,
		Type:       use.Type,
		Required:   use.Required,
		Prohibited: use.Prohibited,
	}
}

// NewAttributeUseRestrictionValidationsForUses projects runtime attribute uses
// into the shapes needed for restriction validation.
func NewAttributeUseRestrictionValidationsForUses(uses []AttributeUse) []AttributeUseRestrictionValidation {
	out := make([]AttributeUseRestrictionValidation, len(uses))
	for i, use := range uses {
		out[i] = NewAttributeUseRestrictionValidationForUse(use)
	}
	return out
}

// NewAttributeUseExtensionValidationForUse projects one runtime attribute use
// into the shape needed for extension preservation validation.
func NewAttributeUseExtensionValidationForUse(use AttributeUse) AttributeUseExtensionValidation {
	return AttributeUseExtensionValidation{
		Default:    NewValueConstraintIdentity(use.Default),
		Fixed:      NewValueConstraintIdentity(use.Fixed),
		Name:       use.Name,
		Type:       use.Type,
		Required:   use.Required,
		Prohibited: use.Prohibited,
	}
}

// NewAttributeUseExtensionValidationsForUses projects runtime attribute uses
// into the shapes needed for extension preservation validation.
func NewAttributeUseExtensionValidationsForUses(uses []AttributeUse) []AttributeUseExtensionValidation {
	out := make([]AttributeUseExtensionValidation, len(uses))
	for i, use := range uses {
		out[i] = NewAttributeUseExtensionValidationForUse(use)
	}
	return out
}

// NoAttributeWildcardState returns provenance for an absent attribute wildcard.
func NoAttributeWildcardState() AttributeWildcardState {
	return AttributeWildcardState{
		Wildcard: NoWildcard,
		Base:     NoWildcard,
		Declared: NoWildcard,
	}
}

// AttributeWildcardRuntime supplies wildcard metadata by ID.
type AttributeWildcardRuntime interface {
	Wildcard(id WildcardID) (Wildcard, bool)
}

// AttributeUseSetRuntime supplies metadata needed to validate attribute-use set
// runtime invariants.
type AttributeUseSetRuntime interface {
	AttributeWildcardRuntime
	SimpleTypeIdentityRuntime
}

// ValidateAttributeUseSetRuntime validates attribute-use set metadata that can
// be expressed in runtime vocabulary.
func ValidateAttributeUseSetRuntime(names *NameTable, rt AttributeUseSetRuntime, set AttributeUseSetValidation) error {
	if err := ValidateAttributeWildcardProvenance(rt, set.Wildcard); err != nil {
		return err
	}
	if len(set.Index) != len(set.Uses) {
		return errors.New("attribute use set index size does not match uses")
	}
	idAttrs := 0
	for i, use := range set.Uses {
		identity, err := validateAttributeUseRuntime(names, rt, set.Index, i, use)
		if err != nil {
			return err
		}
		if identity == SimpleIdentityID {
			if use.HasDefault || use.HasFixed {
				return errors.New("ID-typed attribute use stores value constraint")
			}
			idAttrs++
			if idAttrs > 1 {
				return errors.New("attribute use set stores multiple ID attributes")
			}
		}
	}
	required, valueConstraints, err := attributeUseSetDerivedSlots(set.Uses)
	if err != nil {
		return err
	}
	if !slices.Equal(set.Required, required) {
		return errors.New("attribute use set required slots do not match uses")
	}
	if !slices.Equal(set.ValueConstraints, valueConstraints) {
		return errors.New("attribute use set value constraint slots do not match uses")
	}
	for _, slot := range set.Required {
		if !ValidUint32Index(slot, len(set.Uses)) || !set.Uses[slot].Required {
			return errors.New("attribute use set required slot is invalid")
		}
	}
	for _, slot := range set.ValueConstraints {
		if !ValidUint32Index(slot, len(set.Uses)) || (!set.Uses[slot].HasDefault && !set.Uses[slot].HasFixed) {
			return errors.New("attribute use set value constraint slot is invalid")
		}
	}
	return nil
}

// ValidateAttributeUseSetRecord validates attribute-use set metadata directly
// from runtime records.
func ValidateAttributeUseSetRecord(names *NameTable, rt AttributeUseSetRuntime, set AttributeUseSet) error {
	if err := ValidateAttributeWildcardProvenance(rt, NewAttributeWildcardStateForUseSet(set)); err != nil {
		return err
	}
	if len(set.Index) != len(set.Uses) {
		return errors.New("attribute use set index size does not match uses")
	}
	idAttrs := 0
	requiredSlot := 0
	valueConstraintSlot := 0
	for i, use := range set.Uses {
		validation := NewAttributeUseValidationForUse(use)
		identity, err := validateAttributeUseRuntime(names, rt, set.Index, i, validation)
		if err != nil {
			return err
		}
		if identity == SimpleIdentityID {
			if validation.HasDefault || validation.HasFixed {
				return errors.New("ID-typed attribute use stores value constraint")
			}
			idAttrs++
			if idAttrs > 1 {
				return errors.New("attribute use set stores multiple ID attributes")
			}
		}
		slot, ok := uint32Index(i)
		if !ok {
			return errors.New("attribute use slot is invalid")
		}
		if validation.Required {
			if requiredSlot >= len(set.Required) || set.Required[requiredSlot] != slot {
				return errors.New("attribute use set required slots do not match uses")
			}
			requiredSlot++
		}
		if validation.HasDefault || validation.HasFixed {
			if valueConstraintSlot >= len(set.ValueConstraints) || set.ValueConstraints[valueConstraintSlot] != slot {
				return errors.New("attribute use set value constraint slots do not match uses")
			}
			valueConstraintSlot++
		}
	}
	if requiredSlot != len(set.Required) {
		return errors.New("attribute use set required slots do not match uses")
	}
	if valueConstraintSlot != len(set.ValueConstraints) {
		return errors.New("attribute use set value constraint slots do not match uses")
	}
	return nil
}

// ForEachRequiredAttributeUseSlot validates and iterates required-use slots.
func ForEachRequiredAttributeUseSlot(required []uint32, useCount int, fn func(uint32) error) error {
	for _, slot := range required {
		if !ValidUint32Index(slot, useCount) {
			return errors.New("attribute use set required slot is invalid")
		}
		if err := fn(slot); err != nil {
			return err
		}
	}
	return nil
}

// ForEachValueConstraintAttributeUseSlot validates and iterates absent
// value-constraint use slots.
func ForEachValueConstraintAttributeUseSlot(slots []uint32, useCount int, fn func(uint32) error) error {
	for _, slot := range slots {
		if !ValidUint32Index(slot, useCount) {
			return errors.New("attribute use set value-constraint slot is invalid")
		}
		if err := fn(slot); err != nil {
			return err
		}
	}
	return nil
}

// LookupAttributeUseSlot resolves an attribute-use name through the frozen
// attribute-use index and verifies that the indexed use still names the same
// attribute.
func LookupAttributeUseSlot(index map[QName]uint32, useCount int, name QName, useName func(uint32) (QName, bool)) (uint32, bool) {
	slot, ok := index[name]
	if !ok || !ValidUint32Index(slot, useCount) || useName == nil {
		return 0, false
	}
	got, ok := useName(slot)
	if !ok || got != name {
		return 0, false
	}
	return slot, true
}

// ValidateAttributeUseRestriction validates one derived attribute use against
// its base use.
func ValidateAttributeUseRestriction(
	rt TypeDerivationRuntime,
	base, derived AttributeUseRestrictionValidation,
) error {
	if derived.Prohibited {
		if base.Required {
			return errors.New("required attribute cannot be prohibited by restriction")
		}
		return nil
	}
	if base.Required && !derived.Required {
		return errors.New("required attribute cannot become optional by restriction")
	}
	if _, ok := TypeDerivationMask(rt, SimpleRef(derived.Type), SimpleRef(base.Type)); !ok {
		return errors.New("restricted attribute type is not derived from base")
	}
	if base.Fixed.Present {
		if !FixedValueConstraintEqual(base.Fixed, derived.Fixed) {
			return errors.New("fixed attribute constraint must be preserved by restriction")
		}
	}
	return nil
}

// ValidateAttributeUseSetRestriction validates attribute-use-set restriction
// rules owned by complex-type restriction. It delegates pairwise use
// restriction, new-use wildcard allowance, and wildcard provenance.
func ValidateAttributeUseSetRestriction(
	rt interface {
		TypeDerivationRuntime
		AttributeWildcardRuntime
	},
	base, derived []AttributeUseRestrictionValidation,
	baseWildcard, derivedWildcard AttributeWildcardState,
	bindWildcard bool,
) error {
	for _, use := range base {
		next, ok := attributeUseRestrictionByName(derived, use.Name)
		if use.Required && !ok {
			return errors.New("complex restriction omits required base attribute")
		}
		if ok {
			if err := ValidateAttributeUseRestriction(rt, use, next); err != nil {
				return errors.New("complex restriction attribute use is invalid")
			}
		}
	}
	for _, use := range derived {
		if _, ok := attributeUseRestrictionByName(base, use.Name); ok {
			continue
		}
		if baseWildcard.Wildcard == NoWildcard {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
		wildcard, ok := rt.Wildcard(baseWildcard.Wildcard)
		if !ok || !WildcardAllowsNamespace(wildcard, use.Name.Namespace) {
			return errors.New("complex restriction adds attribute outside base wildcard")
		}
	}
	if !bindWildcard {
		if derivedWildcard.Derivation != AttributeWildcardNone {
			return errors.New("implicit complex type stores derived attribute wildcard provenance")
		}
		return nil
	}
	return ValidateAttributeWildcardDerivation(rt, baseWildcard, derivedWildcard, AttributeWildcardRestriction)
}

func attributeUseRestrictionByName(uses []AttributeUseRestrictionValidation, name QName) (AttributeUseRestrictionValidation, bool) {
	for _, use := range uses {
		if use.Name == name {
			return use, true
		}
	}
	return AttributeUseRestrictionValidation{}, false
}

// ValidateAttributeUseSetExtension validates that every base attribute use is
// preserved unchanged by a complex-type extension.
func ValidateAttributeUseSetExtension(base, derived []AttributeUseExtensionValidation) error {
	for _, use := range base {
		next, ok := attributeUseExtensionByName(derived, use.Name)
		if !ok || !attributeUseExtensionEqual(use, next) {
			return errors.New("complex extension does not preserve base attribute use")
		}
	}
	return nil
}

func attributeUseExtensionByName(uses []AttributeUseExtensionValidation, name QName) (AttributeUseExtensionValidation, bool) {
	for _, use := range uses {
		if use.Name == name {
			return use, true
		}
	}
	return AttributeUseExtensionValidation{}, false
}

func attributeUseExtensionEqual(a, b AttributeUseExtensionValidation) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		ValueConstraintIdentityEqual(a.Default, b.Default) &&
		ValueConstraintIdentityEqual(a.Fixed, b.Fixed) &&
		a.Required == b.Required &&
		a.Prohibited == b.Prohibited
}

func validateAttributeUseRuntime(
	names *NameTable,
	rt AttributeUseSetRuntime,
	index map[QName]uint32,
	i int,
	use AttributeUseValidation,
) (SimpleIdentityKind, error) {
	identity, ok := rt.SimpleTypeIdentity(use.Type)
	if names == nil || !names.ValidQName(use.Name) || !ok {
		return SimpleIdentityNone, errors.New("attribute use references invalid name or type")
	}
	slot, ok := uint32Index(i)
	if !ok {
		return SimpleIdentityNone, errors.New("attribute use slot is invalid")
	}
	if indexed, ok := index[use.Name]; !ok || indexed != slot {
		return SimpleIdentityNone, errors.New("attribute use index does not match use slice")
	}
	if use.Prohibited {
		return SimpleIdentityNone, errors.New("attribute use set stores prohibited use")
	}
	if use.HasDefault && use.HasFixed {
		return SimpleIdentityNone, errors.New("attribute use stores both default and fixed value constraints")
	}
	return identity, nil
}

func attributeUseSetDerivedSlots(uses []AttributeUseValidation) ([]uint32, []uint32, error) {
	required := make([]uint32, 0, len(uses))
	valueConstraints := make([]uint32, 0, len(uses))
	for i, use := range uses {
		slot, ok := uint32Index(i)
		if !ok {
			return nil, nil, errors.New("attribute use slot is invalid")
		}
		if use.Required {
			required = append(required, slot)
		}
		if use.HasDefault || use.HasFixed {
			valueConstraints = append(valueConstraints, slot)
		}
	}
	return required, valueConstraints, nil
}

func uint32Index(i int) (uint32, bool) {
	if i < 0 || uint64(i) > uint64(invalidID) {
		return 0, false
	}
	return uint32(i), true
}

// ValidateAttributeWildcardDerivation validates a derived use-set wildcard
// against the wildcard provenance inherited from its owning type.
func ValidateAttributeWildcardDerivation(
	rt AttributeWildcardRuntime,
	base, derived AttributeWildcardState,
	expected AttributeWildcardDerivation,
) error {
	if derived.Derivation != expected {
		return errors.New("attribute wildcard derivation does not match owning type")
	}
	if derived.Base != base.Wildcard {
		return errors.New("attribute wildcard base does not match owning type")
	}
	return ValidateAttributeWildcardProvenance(rt, derived)
}

// ValidateAttributeWildcardProvenance validates a use-set wildcard against its
// stored base/declared/derivation provenance.
func ValidateAttributeWildcardProvenance(rt AttributeWildcardRuntime, state AttributeWildcardState) error {
	if err := validateAttributeWildcardID(rt, state.Wildcard, "attribute use set references invalid wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.Base, "attribute use set references invalid base wildcard"); err != nil {
		return err
	}
	if err := validateAttributeWildcardID(rt, state.Declared, "attribute use set references invalid declared wildcard"); err != nil {
		return err
	}
	if !ValidAttributeWildcardDerivation(state.Derivation) {
		return errors.New("attribute use set has invalid wildcard derivation")
	}
	switch state.Derivation {
	case AttributeWildcardNone:
		if state.Base != NoWildcard || state.Wildcard != state.Declared {
			return errors.New("attribute wildcard does not match declared wildcard")
		}
	case AttributeWildcardRestriction:
		return validateAttributeWildcardRestriction(rt, state)
	case AttributeWildcardExtension:
		return validateAttributeWildcardExtension(rt, state)
	}
	return nil
}

func validateAttributeWildcardRestriction(rt AttributeWildcardRuntime, state AttributeWildcardState) error {
	if state.Declared == NoWildcard {
		if state.Wildcard != NoWildcard {
			return errors.New("attribute wildcard restriction stores undeclared wildcard")
		}
		return nil
	}
	if state.Base == NoWildcard {
		return errors.New("attribute wildcard restriction has no base wildcard")
	}
	declared, ok := rt.Wildcard(state.Declared)
	if !ok {
		return errors.New("attribute use set references invalid declared wildcard")
	}
	base, ok := rt.Wildcard(state.Base)
	if !ok {
		return errors.New("attribute use set references invalid base wildcard")
	}
	if !WildcardSubset(declared, base) || state.Wildcard != state.Declared {
		return errors.New("attribute wildcard restriction does not match provenance")
	}
	return nil
}

func validateAttributeWildcardExtension(rt AttributeWildcardRuntime, state AttributeWildcardState) error {
	switch {
	case state.Base == NoWildcard:
		if state.Wildcard != state.Declared {
			return errors.New("attribute wildcard extension without base does not match declared wildcard")
		}
	case state.Declared == NoWildcard:
		if state.Wildcard != state.Base {
			return errors.New("attribute wildcard extension does not inherit base wildcard")
		}
	default:
		declared, ok := rt.Wildcard(state.Declared)
		if !ok {
			return errors.New("attribute use set references invalid declared wildcard")
		}
		base, ok := rt.Wildcard(state.Base)
		if !ok {
			return errors.New("attribute use set references invalid base wildcard")
		}
		actual, ok := rt.Wildcard(state.Wildcard)
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
	}
	return nil
}

func validateAttributeWildcardID(rt AttributeWildcardRuntime, id WildcardID, msg string) error {
	if id == NoWildcard {
		return nil
	}
	if _, ok := rt.Wildcard(id); !ok {
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
