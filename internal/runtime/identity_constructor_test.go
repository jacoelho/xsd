package runtime

import "testing"

func TestNewIdentityConstraintDoesNotAliasInputPaths(t *testing.T) {
	t.Parallel()

	selectorName := QName{Namespace: 1, Local: 1}
	elementName := QName{Namespace: 1, Local: 2}
	attrName := QName{Namespace: 1, Local: 3}
	parentName := QName{Namespace: 1, Local: 4}
	replacement := QName{Namespace: 2, Local: 5}
	selector := []IdentityPath{{
		Steps: []IdentityStep{{Name: selectorName}},
	}}
	fields := []IdentityField{{
		Paths: []IdentityFieldPath{
			{Steps: []IdentityStep{{Name: elementName}}},
			{Attr: true, Attribute: attrName, Steps: []IdentityStep{{Name: parentName}}},
		},
	}}

	ic := NewIdentityConstraint(IdentityKey, QName{Namespace: 1, Local: 6}, NoIdentityConstraint, selector, fields)

	selector[0].Steps[0].Name = replacement
	fields[0].Paths[0].Steps[0].Name = replacement
	fields[0].Paths[1].Steps[0].Name = replacement

	if ic.Selector[0].Steps[0].Name != selectorName {
		t.Fatalf("identity selector aliased input path steps: %#v", ic.Selector)
	}
	if ic.Fields[0].Paths[0].Steps[0].Name != elementName || ic.Fields[0].Paths[1].Steps[0].Name != parentName {
		t.Fatalf("identity fields aliased input path steps: %#v", ic.Fields)
	}
	if ic.ElementFields[0].Paths[0].Steps[0].Name != elementName {
		t.Fatalf("compiled element fields aliased input path steps: %#v", ic.ElementFields)
	}
	if ic.AttributeFields[attrName][0].Paths[0].Steps[0].Name != parentName {
		t.Fatalf("compiled attribute fields aliased input path steps: %#v", ic.AttributeFields)
	}
}
