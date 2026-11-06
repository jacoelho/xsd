package schema

import (
	"errors"
	"slices"

	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// valueConstraint is a prevalidated default or fixed value constraint.
// A nil *valueConstraint means absent. Once attached to a runtime component,
// the value is immutable; declarations and uses may share the same pointer.
// The cached value's type is the exact owning simple type, including an outer
// union, never the member that accepted the text; publication rejects a
// member-typed cache even when that member accepts the same lexical value.
type valueConstraint struct {
	qnameContext  *valueConstraintQNameContext
	Lexical       string
	Canonical     string
	ResolvedNames []resolvedValueName
	Value         valuepkg.Value
}

// valueConstraintValidation is the runtime projection needed to validate cached
// value-constraint shape and owner type references.
type valueConstraintValidation struct {
	Lexical          string
	Canonical        string
	Value            valuepkg.Value
	HasResolvedNames bool
}

// ValueConstraintRead exposes prevalidated default/fixed value data to
// validation without exposing compiler-owned pointer storage.
type ValueConstraintRead struct {
	qnameContext *valueConstraintQNameContext
	application  string
	canonical    string
	value        valuepkg.Value
}

// newValueConstraintRead returns the immutable validation read projection for
// one prevalidated value constraint.
func newValueConstraintRead(application, canonical string, value valuepkg.Value) ValueConstraintRead {
	return ValueConstraintRead{
		application: application,
		canonical:   canonical,
		value:       value,
	}
}

// newValueConstraintReadFromConstraint returns the immutable validation read
// projection for one prevalidated value constraint.
func newValueConstraintReadFromConstraint(vc *valueConstraint) (ValueConstraintRead, bool) {
	if vc == nil {
		return ValueConstraintRead{}, false
	}
	application := valueConstraintApplication(vc.Lexical, vc.Canonical, vc.Value)
	read := newValueConstraintRead(application, vc.Canonical, vc.Value)
	read.qnameContext = cloneValueConstraintQNameContext(vc.qnameContext)
	return read, true
}

func valueConstraintApplication(lexical, canonical string, value valuepkg.Value) string {
	if value.HasQualifiedNames() {
		// Expanded QName/NOTATION projections are not lexical XML values.
		return lexical
	}
	return canonical
}

// ApplicationText returns the lexical spelling for applying the constraint.
// Context-free values use canonical text; resolved names retain source spelling.
func (v ValueConstraintRead) ApplicationText() string {
	return v.application
}

// CanonicalText returns the canonical text used for fixed-value comparison.
func (v ValueConstraintRead) CanonicalText() string {
	return v.canonical
}

// Value returns the cached canonical value.
func (v ValueConstraintRead) Value() valuepkg.Value {
	return v.value
}

// ResolveQName resolves a QName in the schema namespace context captured for
// this constraint. It is repeatable and does not consume proof entries.
func (v ValueConstraintRead) ResolveQName(lexical string) (valuepkg.ExpandedName, bool) {
	return v.qnameContext.ResolveQName(lexical)
}

// ElementValueConstraints exposes prevalidated default/fixed values attached to
// one element declaration.
type ElementValueConstraints struct {
	constraint elementConstraintRead
	owner      TypeID
}

// OwnerType returns the declaration type that validated the cached value.
func (c ElementValueConstraints) OwnerType() TypeID {
	return c.owner
}

// HasAny reports whether either default or fixed is present.
func (c ElementValueConstraints) HasAny() bool {
	return c.constraint.kind.present()
}

// FixedValue returns the fixed value, if present.
func (c ElementValueConstraints) FixedValue() (ValueConstraintRead, bool) {
	if c.constraint.kind != declarationValueConstraintFixed {
		return ValueConstraintRead{}, false
	}
	return c.constraint.value, true
}

// DefaultValueConstraint returns the default value, if present.
func (c ElementValueConstraints) DefaultValueConstraint() (ValueConstraintRead, bool) {
	if c.constraint.kind != declarationValueConstraintDefault {
		return ValueConstraintRead{}, false
	}
	return c.constraint.value, true
}

// resolvedValueName records one QName resolution proof entry captured while
// validating a value constraint.
type resolvedValueName struct {
	Lexical string
	NS      string
	Local   string
}

// valueConstraintNameReplay replays QName resolution proofs captured while
// validating a value constraint.
type valueConstraintNameReplay struct {
	entries []resolvedValueName
	used    []bool
}

// newValueConstraintNameReplay validates resolved QName proof entries and
// returns replay state for datatype validation.
func newValueConstraintNameReplay(entries []resolvedValueName) (valueConstraintNameReplay, error) {
	seen := make(map[string]resolvedValueName, len(entries))
	for _, entry := range entries {
		parts := resolvedValueNameLexicalParts(entry.Lexical)
		if !parts.Valid || entry.Local != parts.Local || !lex.IsNCName(entry.Local) {
			return valueConstraintNameReplay{}, errors.New("resolved name proof is not deterministic")
		}
		prev, ok := seen[entry.Lexical]
		if !ok {
			seen[entry.Lexical] = entry
			continue
		}
		if prev.NS != entry.NS || prev.Local != entry.Local {
			return valueConstraintNameReplay{}, errors.New("resolved name proof is not deterministic")
		}
	}
	return valueConstraintNameReplay{
		entries: entries,
		used:    make([]bool, len(entries)),
	}, nil
}

// ResolveQName replays one captured QName resolution.
func (r *valueConstraintNameReplay) ResolveQName(lexical string) (valuepkg.ExpandedName, bool) {
	parts := resolvedValueNameLexicalParts(lexical)
	if !parts.Valid || parts.Prefixed && parts.Prefix == "" {
		return valuepkg.ExpandedName{}, false
	}
	for i, resolved := range r.entries {
		if r.used[i] || resolved.Lexical != lexical {
			continue
		}
		if resolved.Local != parts.Local || !lex.IsNCName(resolved.Local) {
			return valuepkg.ExpandedName{}, false
		}
		r.used[i] = true
		return valuepkg.ExpandedName{Namespace: resolved.NS, Local: resolved.Local}, true
	}
	return valuepkg.ExpandedName{}, false
}

func resolvedValueNameLexicalParts(lexical string) lex.QNameParts {
	trimmed := lex.TrimXMLWhitespaceString(lexical)
	if trimmed == "" {
		return lex.QNameParts{}
	}
	return lex.SplitQName(trimmed)
}

// constraintIdentity is the equality projection used when runtime rules
// must prove that an inherited value constraint was preserved unchanged.
type constraintIdentity struct {
	qnameContext  *valueConstraintQNameContext
	Lexical       string
	Canonical     string
	ResolvedNames []resolvedValueName
	Value         valuepkg.Value
	Present       bool
}

// newValueConstraintIdentity returns the equality projection for a
// prevalidated value constraint.
func newValueConstraintIdentity(vc *valueConstraint) constraintIdentity {
	if vc == nil {
		return constraintIdentity{}
	}
	return cloneValueConstraintIdentity(constraintIdentity{
		ResolvedNames: vc.ResolvedNames,
		Lexical:       vc.Lexical,
		Canonical:     vc.Canonical,
		Value:         vc.Value,
		qnameContext:  vc.qnameContext,
		Present:       true,
	})
}

// valueConstraintIdentityEqual reports whether two value-constraint identity
// projections preserve the same validated constraint.
func valueConstraintIdentityEqual(a, b constraintIdentity) bool {
	if a.Present != b.Present {
		return false
	}
	if !a.Present {
		return true
	}
	if a.Lexical != b.Lexical || a.Canonical != b.Canonical || !slices.Equal(a.ResolvedNames, b.ResolvedNames) {
		return false
	}
	if !equalValueConstraintQNameContexts(a.qnameContext, b.qnameContext) {
		return false
	}
	if a.Value.Type() == valuepkg.NoType || b.Value.Type() == valuepkg.NoType {
		return a.Value.Type() == b.Value.Type()
	}
	return a.Value.Equal(b.Value)
}

// fixedValueConstraintEqual reports whether two fixed value constraints carry
// the same actual value. Exact lexical preservation is checked separately by
// valueConstraintIdentityEqual.
func fixedValueConstraintEqual(base, derived constraintIdentity) bool {
	if base.Present != derived.Present {
		return false
	}
	if !base.Present {
		return true
	}
	return fixedValueEqual(base, derived)
}

// fixedValueEqual compares untyped (mixed or anySimpleType-free) constraints
// as strings and typed ones only in value space; internal/value owns typed
// equality, so no lexical or canonical fallback applies.
func fixedValueEqual(base, derived constraintIdentity) bool {
	if base.Value.Type() == valuepkg.NoType || derived.Value.Type() == valuepkg.NoType {
		return base.Canonical == derived.Canonical
	}
	return base.Value.Equal(derived.Value)
}

// newValueConstraintValidation returns the runtime validation projection for a
// prevalidated value constraint.
func newValueConstraintValidation(vc *valueConstraint) valueConstraintValidation {
	if vc == nil {
		return valueConstraintValidation{}
	}
	return valueConstraintValidation{
		Lexical:          vc.Lexical,
		Canonical:        vc.Canonical,
		Value:            vc.Value,
		HasResolvedNames: len(vc.ResolvedNames) != 0,
	}
}

// valueConstraintSimpleType is the simple-type projection needed for cached
// value-constraint owner matching.
type valueConstraintSimpleType struct {
	Union          []SimpleTypeID
	ListItem       SimpleTypeID
	Variety        valuepkg.Variety
	Primitive      valuepkg.PrimitiveKind
	HasEnumeration bool
}

// valueConstraintComplexType is the complex-type projection needed to
// determine the owner type for an element value constraint.
type valueConstraintComplexType struct {
	Content     contentModelID
	TextType    SimpleTypeID
	ContentKind complexContentVariety
}

// newValueConstraintComplexTypeForComplexType projects a runtime complex type
// into the shape needed to determine an element value-constraint owner.
func newValueConstraintComplexTypeForComplexType(ct complexTypeMetadata) valueConstraintComplexType {
	return valueConstraintComplexType{
		Content:     ct.Content,
		TextType:    ct.TextType,
		ContentKind: ct.ContentKind,
	}
}

// elementValueConstraintType returns the simple type that must own an element's
// value constraint. NoSimpleType means the constraint is allowed only as mixed
// lexical text.
func elementValueConstraintType(
	rt *schemaBuild,
	analysis *contentModelAnalysis,
	typ TypeID,
) (SimpleTypeID, error) {
	if id, ok := typ.Simple(); ok {
		return id, nil
	}
	id, ok := typ.Complex()
	if !ok || rt == nil {
		return NoSimpleType, errors.New("element value constraint references invalid type")
	}
	ct, ok := rt.ValueConstraintComplexType(id)
	if !ok {
		return NoSimpleType, errors.New("element value constraint references invalid type")
	}
	switch {
	case ct.ContentKind.Simple():
		return ct.TextType, nil
	case ct.ContentKind.Mixed():
		return mixedElementValueConstraintType(analysis, ct.Content)
	default:
		return NoSimpleType, errors.New("element value constraint requires simple content")
	}
}

func mixedElementValueConstraintType(analysis *contentModelAnalysis, content contentModelID) (SimpleTypeID, error) {
	if analysis == nil {
		return NoSimpleType, errors.New("content model analysis is nil")
	}
	emptiable, err := analysis.ModelEmptiable(content)
	if err != nil {
		return NoSimpleType, err
	}
	if !emptiable {
		return NoSimpleType, errors.New("element value constraint requires simple content")
	}
	return NoSimpleType, nil
}

// validateValueConstraintShape validates cached value-constraint metadata that
// is independent of datatype replay.
func validateValueConstraintShape(rt *schemaBuild, vc valueConstraintValidation, expected SimpleTypeID) error {
	if expected == NoSimpleType {
		if vc.Value.CanonicalText() != vc.Canonical {
			return errors.New("canonical value mismatch")
		}
		if vc.Value.Type() != valuepkg.NoType ||
			vc.Canonical != vc.Lexical ||
			vc.Value.IDs() != "" ||
			vc.Value.IDRefs() != "" ||
			vc.Value.HasIdentity() ||
			vc.HasResolvedNames {
			return errors.New("mixed value constraint is not untyped lexical text")
		}
		return nil
	}
	if rt == nil {
		return errors.New("value type does not match owner type")
	}
	_, ok := rt.valueBuilder.InputRequirements(vc.Value.Type())
	if !ok {
		return errors.New("value type does not match owner type")
	}
	if vc.Value.Type() != expected {
		return errors.New("value type does not match owner type")
	}
	if vc.Value.CanonicalText() != vc.Canonical {
		return errors.New("canonical value mismatch")
	}
	return nil
}
