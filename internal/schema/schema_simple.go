package schema

import (
	"github.com/jacoelho/xsd/internal/value"
)

// SimpleTypeIdentityRuntime exposes the value-owned document-identity
// projection at schema validation boundaries.
type SimpleTypeIdentityRuntime interface {
	SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool)
}

func simpleValueFromValue(validated value.Value) SimpleValue {
	return SimpleValue{
		Canonical: validated.CanonicalText(),
		IDs:       validated.IDs(),
		IDRefs:    validated.IDRefs(),
		Identity:  validated.IdentityKey(),
		Type:      validated.Type(),
	}
}

// SimpleTypeIdentity returns compiler-owned identity metadata.
func (rt *schemaBuild) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	st, ok := SimpleTypeByID(rt.SimpleTypes, id)
	if !ok {
		return SimpleIdentityNone, false
	}
	if rt.valueBuilder != nil {
		if view, complete := rt.valueBuilder.TypeView(id); complete {
			return view.Identity, true
		}
	}
	return st.ValueSpec.Identity, true
}

// ValidateSimpleValue validates a lexical simple value using frozen runtime reads.
func (rt *Schema) ValidateSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	return rt.ValidateSimpleValueWithScratch(id, lexical, resolve, needs, nil)
}

// ValidateSimpleValueWithScratch validates a lexical simple value while reusing
// caller-owned string-pattern buffers.
func (rt *Schema) ValidateSimpleValueWithScratch(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed, scratch *value.Scratch) (SimpleValue, error) {
	if rt == nil || rt.program.Value == nil {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	var want value.Needs
	if needs.Has(SimpleNeedCanonical) {
		want |= value.NeedCanonical
	}
	if needs.Has(SimpleNeedIdentity) {
		want |= value.NeedIdentity
	}
	resolver := value.Resolver{Notation: rt.NotationDeclared}
	if resolve != nil {
		resolver.QName = resolve
	}
	validated, err := rt.program.Value.Validate(id, lexical, resolver, want, scratch)
	if err != nil {
		return SimpleValue{}, err
	}
	return simpleValueFromValue(validated), nil
}
