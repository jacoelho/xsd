package runtime

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildIdentityFieldLookupMirrorsFields(t *testing.T) {
	t.Parallel()

	elem := QName{Namespace: 1, Local: 1}
	attr := QName{Namespace: 1, Local: 2}
	otherAttr := QName{Namespace: 2, Local: 3}
	fields := []IdentityField{
		{Paths: []IdentityFieldPath{
			{Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, Attribute: attr},
			{Attr: true, Attribute: attr, Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, AttrWildcard: true},
			{Attr: true, AttrWildcard: true, AttrNamespaceSet: true, AttrNamespace: otherAttr.Namespace},
		}},
		{Paths: []IdentityFieldPath{
			{Attr: true, Attribute: otherAttr},
		}},
	}

	elementFields, attrFields, attrWildcardFields := BuildIdentityFieldLookup(fields)

	wantElementFields := []CompiledIdentityField{{
		Field: 0,
		Paths: []IdentityFieldPath{{Steps: []IdentityStep{{Name: elem}}}},
	}}
	wantAttrFields := map[QName][]CompiledIdentityField{
		attr: {{
			Field: 0,
			Paths: []IdentityFieldPath{
				{Attr: true, Attribute: attr},
				{Attr: true, Attribute: attr, Steps: []IdentityStep{{Name: elem}}},
			},
		}},
		otherAttr: {{
			Field: 1,
			Paths: []IdentityFieldPath{{Attr: true, Attribute: otherAttr}},
		}},
	}
	wantAttrWildcardFields := []CompiledIdentityField{{
		Field: 0,
		Paths: []IdentityFieldPath{
			{Attr: true, AttrWildcard: true},
			{Attr: true, AttrWildcard: true, AttrNamespaceSet: true, AttrNamespace: otherAttr.Namespace},
		},
	}}

	if !reflect.DeepEqual(elementFields, wantElementFields) {
		t.Fatalf("elementFields = %#v, want %#v", elementFields, wantElementFields)
	}
	if !reflect.DeepEqual(attrFields, wantAttrFields) {
		t.Fatalf("attrFields = %#v, want %#v", attrFields, wantAttrFields)
	}
	if !reflect.DeepEqual(attrWildcardFields, wantAttrWildcardFields) {
		t.Fatalf("attrWildcardFields = %#v, want %#v", attrWildcardFields, wantAttrWildcardFields)
	}
}

func TestBuildIdentityFieldLookupDoesNotAliasFieldPathSteps(t *testing.T) {
	t.Parallel()

	elem := QName{Namespace: 1, Local: 1}
	attr := QName{Namespace: 1, Local: 2}
	otherNamespace := NamespaceID(2)
	replacement := QName{Namespace: 3, Local: 4}
	fields := []IdentityField{{
		Paths: []IdentityFieldPath{
			{Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, Attribute: attr, Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, AttrWildcard: true, Steps: []IdentityStep{{Name: elem}}, AttrNamespaceSet: true, AttrNamespace: otherNamespace},
		},
	}}

	elementFields, attrFields, attrWildcardFields := BuildIdentityFieldLookup(fields)

	elementFields[0].Paths[0].Steps[0].Name = replacement
	attrFields[attr][0].Paths[0].Steps[0].Name = replacement
	attrWildcardFields[0].Paths[0].Steps[0].Name = replacement
	if fields[0].Paths[0].Steps[0].Name != elem ||
		fields[0].Paths[1].Steps[0].Name != elem ||
		fields[0].Paths[2].Steps[0].Name != elem {
		t.Fatalf("BuildIdentityFieldLookup aliased source path steps: %#v", fields)
	}
}

func TestValidateIdentityConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func([]IdentityConstraint, map[string]QName)
		wantErr string
	}{
		{
			name: "valid key and keyrefs",
		},
		{
			name: "invalid name",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Name = QName{Namespace: NamespaceID(99), Local: names["key"].Local}
			},
			wantErr: "identity constraint references invalid name",
		},
		{
			name: "non-keyref stores refer",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Refer = 1
			},
			wantErr: "non-keyref identity constraint stores refer",
		},
		{
			name: "keyref missing refer",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[1].Refer = NoIdentityConstraint
			},
			wantErr: "keyref identity constraint references invalid key",
		},
		{
			name: "keyref out-of-range refer",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[1].Refer = 99
			},
			wantErr: "keyref identity constraint references invalid key",
		},
		{
			name: "keyref references keyref",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[1].Refer = 2
			},
			wantErr: "keyref identity constraint references keyref",
		},
		{
			name: "keyref field count mismatch",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[1].Fields = append(identities[1].Fields, identityAttrField(names["id"]))
				refreshIdentityLookup(&identities[1])
			},
			wantErr: "keyref identity constraint field count differs from refer",
		},
		{
			name: "missing selector",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Selector = nil
			},
			wantErr: "identity constraint has no selector",
		},
		{
			name: "missing fields",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields = nil
				refreshIdentityLookup(&identities[0])
			},
			wantErr: "identity constraint has no fields",
		},
		{
			name: "field without paths",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields[0].Paths = nil
				refreshIdentityLookup(&identities[0])
			},
			wantErr: "identity field has no paths",
		},
		{
			name: "self selector stores inactive fields",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Selector[0] = IdentityPath{
					Self:       true,
					Descendant: true,
					Steps:      []IdentityStep{{Name: names["item"]}},
				}
			},
			wantErr: "identity selector references invalid name",
		},
		{
			name: "field path stores inactive attribute",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields[0].Paths[0] = IdentityFieldPath{
					Self:      true,
					Attr:      true,
					Attribute: names["id"],
				}
				refreshIdentityLookup(&identities[0])
			},
			wantErr: "identity field path has invalid shape",
		},
		{
			name: "attribute wildcard uses invalid namespace",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields = []IdentityField{{
					Paths: []IdentityFieldPath{{
						Attr:             true,
						AttrWildcard:     true,
						AttrNamespaceSet: true,
						AttrNamespace:    NamespaceID(99),
					}},
				}}
				refreshIdentityLookup(&identities[0])
			},
			wantErr: "identity field path has invalid shape",
		},
		{
			name: "lookup drift",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].AttributeFields = nil
			},
			wantErr: "identity constraint field lookup does not match fields",
		},
		{
			name: "nested lookup step drift",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields = []IdentityField{{
					Paths: []IdentityFieldPath{{
						Attribute: NoQName(),
						Steps:     []IdentityStep{{Name: names["child"]}},
					}},
				}}
				refreshIdentityLookup(&identities[0])
				identities[0].ElementFields[0].Paths[0].Steps[0].Name = names["item"]
			},
			wantErr: "identity constraint field lookup does not match fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			table, identities, names := identityValidationFixture(t)
			if tt.mutate != nil {
				tt.mutate(identities, names)
			}
			err := ValidateIdentityConstraints(&table, identities)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateIdentityConstraints() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateIdentityConstraints() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIdentityConstraintsRejectsNilNameTable(t *testing.T) {
	t.Parallel()

	table, identities, names := identityValidationFixture(t)
	_ = table
	_ = names
	err := ValidateIdentityConstraints(nil, identities)
	if err == nil || !strings.Contains(err.Error(), "identity constraints require name table") {
		t.Fatalf("ValidateIdentityConstraints(nil, identities) error = %v", err)
	}
}

func TestValidateIdentityConstraintsAllowsEmptyAttributeLookupMap(t *testing.T) {
	t.Parallel()

	table, _, names := identityValidationFixture(t)
	key := IdentityConstraint{
		Name:     names["key"],
		Selector: []IdentityPath{{Steps: []IdentityStep{{Name: names["item"]}}}},
		Fields: []IdentityField{{
			Paths: []IdentityFieldPath{{
				Attribute: NoQName(),
				Steps:     []IdentityStep{{Name: names["child"]}},
			}},
		}},
		Refer: NoIdentityConstraint,
		Kind:  IdentityKey,
	}
	refreshIdentityLookup(&key)
	key.AttributeFields = map[QName][]CompiledIdentityField{}

	if err := ValidateIdentityConstraints(&table, []IdentityConstraint{key}); err != nil {
		t.Fatalf("ValidateIdentityConstraints() error = %v", err)
	}
}

func TestIdentityConstraintReadProjectionHelpers(t *testing.T) {
	t.Parallel()

	_, identities, names := identityValidationFixture(t)
	identities[0].Fields[0].Paths = append(identities[0].Fields[0].Paths,
		IdentityFieldPath{Steps: []IdentityStep{{Name: names["child"]}}},
		IdentityFieldPath{Attr: true, AttrWildcard: true},
	)
	refreshIdentityLookup(&identities[0])
	want := cloneIdentityConstraintsForTest(identities)
	reads := newIdentityConstraintReads(identities)
	if !equalIdentityConstraintReadProjection(reads, identities) {
		t.Fatal("EqualIdentityConstraintReadProjection() rejected matching projection")
	}
	if equalIdentityConstraintReadProjection(reads[:1], identities) {
		t.Fatal("EqualIdentityConstraintReadProjection() accepted mismatched table length")
	}

	changed := cloneIdentityConstraintsForTest(want)
	changed[0].Selector[0].Steps[0].Name = names["child"]
	changed[0].Fields = nil
	changed[0].ElementFields[0].Paths[0].Steps[0].Name = names["child"]
	changed[0].AttributeFields[names["id"]][0].Paths[0].Attribute = names["child"]
	changed[0].AttributeWildcardFields = []CompiledIdentityField{{
		Field: 9,
		Paths: []IdentityFieldPath{{Attr: true, AttrWildcard: true}},
	}}
	changed[0].Kind = IdentityKeyRef
	changed[0].Refer = 1

	if equalIdentityConstraintReadProjection(reads, changed) {
		t.Fatal("EqualIdentityConstraintReadProjection() accepted mismatched projection")
	}
	if err := validateIdentityConstraintReadProjection(newIdentityConstraintReads(want), want); err != nil {
		t.Fatalf("ValidateIdentityConstraintReadProjection() error = %v", err)
	}
	if err := validateIdentityConstraintReadProjection(reads[:1], want); err == nil || err.Error() != "identity constraint read projection count does not match constraints" {
		t.Fatalf("ValidateIdentityConstraintReadProjection(short) error = %v, want count invariant", err)
	}
	if err := validateIdentityConstraintReadProjection(reads, changed); err == nil || err.Error() != "identity constraint read projection does not match constraints" {
		t.Fatalf("ValidateIdentityConstraintReadProjection(changed) error = %v, want mismatch invariant", err)
	}

	read, ok := identityConstraintReadByID(reads, 0)
	if !ok {
		t.Fatal("IdentityConstraintReadByID() rejected valid constraint")
	}
	selectors := read.SelectorPaths()
	if selectors.Len() != len(want[0].Selector) {
		t.Fatalf("SelectorPaths() count = %d; want %d", selectors.Len(), len(want[0].Selector))
	}
	if read.FieldCount() != len(want[0].Fields) {
		t.Fatalf("FieldCount() = %d, want %d", read.FieldCount(), len(want[0].Fields))
	}
	if read.Kind() != want[0].Kind || read.Refer() != want[0].Refer {
		t.Fatalf("identity info = kind %v refer %v, want %v %v",
			read.Kind(), read.Refer(), want[0].Kind, want[0].Refer)
	}

	elementFields := read.ElementFields()
	if elementFields.Len() != len(want[0].ElementFields) {
		t.Fatalf("ElementFields() count = %d; want %d", elementFields.Len(), len(want[0].ElementFields))
	}

	attributeFields := read.AttributeFields(names["id"])
	if attributeFields.Len() != len(want[0].AttributeFields[names["id"]]) {
		t.Fatalf("AttributeFields() count = %d; want %d", attributeFields.Len(), len(want[0].AttributeFields[names["id"]]))
	}

	wildcardFields := read.AttributeWildcardFields()
	if wildcardFields.Len() != len(want[0].AttributeWildcardFields) {
		t.Fatalf("AttributeWildcardFields() count = %d; want %d", wildcardFields.Len(), len(want[0].AttributeWildcardFields))
	}
}

func cloneIdentityConstraintsForTest(in []IdentityConstraint) []IdentityConstraint {
	out := make([]IdentityConstraint, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Selector = CloneIdentityPaths(in[i].Selector)
		out[i].Fields = CloneIdentityFields(in[i].Fields)
		out[i].ElementFields, out[i].AttributeFields, out[i].AttributeWildcardFields = BuildIdentityFieldLookup(out[i].Fields)
	}
	return out
}

func TestIdentityReadAccessors(t *testing.T) {
	t.Parallel()

	_, identities, names := identityValidationFixture(t)
	identities[0].Selector = append(identities[0].Selector, IdentityPath{Self: true})
	identities[0].Fields[0].Paths = append(identities[0].Fields[0].Paths,
		IdentityFieldPath{Steps: []IdentityStep{{Name: names["child"]}}},
		IdentityFieldPath{Attr: true, AttrWildcard: true},
	)
	refreshIdentityLookup(&identities[0])
	reads := newIdentityConstraintReads(identities)
	invalid := IdentityConstraintID(99)
	read, readOK := identityConstraintReadByID(reads, 0)
	if !readOK {
		t.Fatal("IdentityConstraintReadByID() rejected valid constraint")
	}
	if _, ok := identityConstraintReadByID(reads, invalid); ok {
		t.Fatal("IdentityConstraintReadByID() accepted invalid constraint")
	}

	constraints, constraintsOK := ElementIdentityConstraintIDs([][]IdentityConstraintID{{0, 1}}, 0)
	first, firstOK := constraints.At(0)
	if !constraintsOK || constraints.Len() != 2 || !firstOK || first != 0 {
		t.Fatalf("ElementIdentityConstraintIDs() = len %d first %d, %v; want 2, 0, true", constraints.Len(), first, constraintsOK)
	}
	if _, ok := ElementIdentityConstraintIDs(nil, 0); ok {
		t.Fatal("ElementIdentityConstraintIDs() accepted invalid element")
	}

	selectorPaths := read.SelectorPaths()
	if selectorPaths.Len() != len(identities[0].Selector) {
		t.Fatalf("IdentityConstraintRead.SelectorPaths() count = %d, want %d", selectorPaths.Len(), len(identities[0].Selector))
	}

	if count := read.FieldCount(); count != 1 {
		t.Fatalf("IdentityConstraintRead.FieldCount() = %d, want 1", count)
	}

	elementFields := read.ElementFields()
	if elementFields.Len() != len(identities[0].ElementFields) {
		t.Fatalf("IdentityConstraintRead.ElementFields() count = %d, want %d", elementFields.Len(), len(identities[0].ElementFields))
	}
	attributeFields := read.AttributeFields(names["id"])
	if attributeFields.Len() != len(identities[0].AttributeFields[names["id"]]) {
		t.Fatalf("IdentityConstraintRead.AttributeFields() count = %d, want %d", attributeFields.Len(), len(identities[0].AttributeFields[names["id"]]))
	}
	wildcardFields := read.AttributeWildcardFields()
	if wildcardFields.Len() != len(identities[0].AttributeWildcardFields) {
		t.Fatalf("IdentityConstraintRead.AttributeWildcardFields() count = %d, want %d", wildcardFields.Len(), len(identities[0].AttributeWildcardFields))
	}
	if read.Kind() != IdentityKey || read.Refer() != NoIdentityConstraint {
		t.Fatalf("IdentityConstraintRead identity info = %v, %v; want key, no reference", read.Kind(), read.Refer())
	}
}

func identityValidationFixture(t *testing.T) (NameTable, []IdentityConstraint, map[string]QName) {
	t.Helper()

	namespaces := []string{EmptyNamespaceURI}
	requiredNames := []ExpandedName{
		{Local: "key"},
		{Local: "kr1"},
		{Local: "kr2"},
		{Local: "item"},
		{Local: "child"},
		{Local: "id"},
	}
	table, err := NewNameTable(64, namespaces, requiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names := make(map[string]QName, len(requiredNames))
	for _, name := range requiredNames {
		q, ok := table.LookupQName(name.Namespace, name.Local)
		if !ok {
			t.Fatalf("LookupQName(%q, %q) failed", name.Namespace, name.Local)
		}
		names[name.Local] = q
	}

	key := identityConstraint(names["key"], IdentityKey, NoIdentityConstraint, names["item"], names["id"])
	kr1 := identityConstraint(names["kr1"], IdentityKeyRef, 0, names["item"], names["id"])
	kr2 := identityConstraint(names["kr2"], IdentityKeyRef, 0, names["item"], names["id"])
	return table, []IdentityConstraint{key, kr1, kr2}, names
}

func identityConstraint(name QName, kind IdentityKind, refer IdentityConstraintID, selectorName, attr QName) IdentityConstraint {
	ic := IdentityConstraint{
		Name:     name,
		Selector: []IdentityPath{{Steps: []IdentityStep{{Name: selectorName}}}},
		Fields:   []IdentityField{identityAttrField(attr)},
		Refer:    refer,
		Kind:     kind,
	}
	refreshIdentityLookup(&ic)
	return ic
}

func identityAttrField(attr QName) IdentityField {
	return IdentityField{Paths: []IdentityFieldPath{{Attr: true, Attribute: attr}}}
}

func refreshIdentityLookup(ic *IdentityConstraint) {
	ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = BuildIdentityFieldLookup(ic.Fields)
}
