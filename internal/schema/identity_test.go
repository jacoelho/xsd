package schema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

func TestIdentityFieldProgramPartitions(t *testing.T) {
	t.Parallel()

	elem := QName{Namespace: 1, Local: 1}
	attr := QName{Namespace: 1, Local: 2}
	otherAttr := QName{Namespace: 2, Local: 3}
	fields := []IdentityField{
		{Paths: []IdentityFieldPath{
			{Steps: []IdentityStep{{Name: elem}}, Attribute: NoQName()},
			{Attr: true, Attribute: attr},
			{Attr: true, Attribute: attr, Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, AttrWildcard: true, Attribute: NoQName()},
			{Attr: true, AttrWildcard: true, Attribute: NoQName(), AttrNamespaceSet: true, AttrNamespace: otherAttr.Namespace},
		}},
		{Paths: []IdentityFieldPath{
			{Attr: true, Attribute: otherAttr},
			{Attr: true, Attribute: attr, Descendant: true, Steps: []IdentityStep{{Name: elem}}},
		}},
	}
	identity := NewIdentityConstraint(IdentityKey, elem, NoIdentityConstraint, nil, fields)
	program := newIdentityConstraintProgramRead(identity)
	wantElements := []IdentityCompiledFieldProgramRead{{
		field: 0,
		paths: []IdentityPathProgramRead{{steps: []IdentityStep{{Name: elem}}, attribute: NoQName()}},
	}}
	wantAttributes := map[QName][]IdentityCompiledFieldProgramRead{
		attr: {
			{field: 0, paths: []IdentityPathProgramRead{
				{attributePath: true, attribute: attr},
				{attributePath: true, attribute: attr, steps: []IdentityStep{{Name: elem}}},
			}},
			{field: 1, paths: []IdentityPathProgramRead{
				{attributePath: true, attribute: attr, descendant: true, steps: []IdentityStep{{Name: elem}}},
			}},
		},
		otherAttr: {{field: 1, paths: []IdentityPathProgramRead{{attributePath: true, attribute: otherAttr}}}},
	}
	wantWildcards := []IdentityCompiledFieldProgramRead{{
		field: 0,
		paths: []IdentityPathProgramRead{
			{attributePath: true, attributeWildcard: true, attribute: NoQName()},
			{attributePath: true, attributeWildcard: true, attribute: NoQName(), attributeNamespaceSet: true, attributeNamespace: otherAttr.Namespace},
		},
	}}
	if got := program.ElementFields().values; !reflect.DeepEqual(got, wantElements) {
		t.Fatalf("element fields = %#v, want %#v", got, wantElements)
	}
	if got := program.attributeFields; !reflect.DeepEqual(got, wantAttributes) {
		t.Fatalf("attribute fields = %#v, want %#v", got, wantAttributes)
	}
	if got := program.AttributeWildcardFields().values; !reflect.DeepEqual(got, wantWildcards) {
		t.Fatalf("wildcard fields = %#v, want %#v", got, wantWildcards)
	}
}

func TestIdentityFieldProgramDoesNotAliasConstructionSteps(t *testing.T) {
	t.Parallel()

	elem := QName{Namespace: 1, Local: 1}
	attr := QName{Namespace: 1, Local: 2}
	otherNamespace := NamespaceID(2)
	replacement := QName{Namespace: 3, Local: 4}
	identity := NewIdentityConstraint(IdentityKey, elem, NoIdentityConstraint, nil, []IdentityField{{
		Paths: []IdentityFieldPath{
			{Steps: []IdentityStep{{Name: elem}}, Attribute: NoQName()},
			{Attr: true, Attribute: attr, Steps: []IdentityStep{{Name: elem}}},
			{Attr: true, AttrWildcard: true, Attribute: NoQName(), Steps: []IdentityStep{{Name: elem}}, AttrNamespaceSet: true, AttrNamespace: otherNamespace},
		},
	}})
	program := newIdentityConstraintProgramRead(identity)
	for index := range identity.Fields[0].Paths {
		identity.Fields[0].Paths[index].Steps[0].Name = replacement
	}
	for _, fields := range []IdentityCompiledFieldProgramReads{
		program.ElementFields(), program.AttributeFields(attr), program.AttributeWildcardFields(),
	} {
		field, ok := fields.At(0)
		if !ok {
			t.Fatal("published field is missing")
		}
		path, ok := field.Path(0)
		if !ok {
			t.Fatal("published field path is missing")
		}
		if step, ok := path.Step(0); !ok || step.Name != elem {
			t.Fatalf("published field step = %v/%t, want original element", step, ok)
		}
	}
}

func TestIdentityFieldProgramRetainsOnlyOccupiedGroupsAndPaths(t *testing.T) {
	t.Parallel()

	table, _, names := identityValidationFixture(t)
	wantNames := []QName{names["item"], names["child"], names["key"], names["kr1"], names["kr2"]}
	wantDescendant := []bool{false, true, false, true, false}
	fields := make([]IdentityField, 5)
	for fieldIndex := range fields {
		for index, name := range wantNames {
			descendant := index == 1 || index == 3
			fields[fieldIndex].Paths = append(fields[fieldIndex].Paths,
				IdentityFieldPath{Steps: []IdentityStep{{Name: name}}, Attribute: NoQName(), Descendant: descendant},
				IdentityFieldPath{Steps: []IdentityStep{{Name: name}}, Attr: true, Attribute: names["id"], Descendant: descendant},
				IdentityFieldPath{Steps: []IdentityStep{{Name: name}}, Attr: true, Attribute: NoQName(), AttrWildcard: true, AttrNamespaceSet: true, AttrNamespace: EmptyNamespaceID, Descendant: descendant},
			)
		}
	}
	identity := NewIdentityConstraint(IdentityKey, names["key"], NoIdentityConstraint,
		[]IdentityPath{{Steps: []IdentityStep{{Name: names["item"]}}}}, fields,
	)
	if err := ValidateIdentityConstraints(&table, []IdentityConstraint{identity}); err != nil {
		t.Fatalf("canonical field fixture is invalid: %v", err)
	}
	program := newIdentityConstraintProgramRead(identity)
	partitions := []struct {
		name      string
		fields    IdentityCompiledFieldProgramReads
		attribute bool
		wildcard  bool
	}{
		{name: "element", fields: program.ElementFields()},
		{name: "exact attribute", fields: program.AttributeFields(names["id"]), attribute: true},
		{name: "wildcard attribute", fields: program.AttributeWildcardFields(), attribute: true, wildcard: true},
	}
	for _, partition := range partitions {
		if partition.fields.Len() != 5 || cap(partition.fields.values) != 5 {
			t.Fatalf("%s groups len/cap = %d/%d, want 5/5", partition.name, partition.fields.Len(), cap(partition.fields.values))
		}
		for _, wantField := range []int{0, 1, 2, 3, 4} {
			field, ok := partition.fields.At(wantField)
			if !ok || field.Field() != wantField || field.PathCount() != 5 || cap(field.paths) != 5 {
				t.Fatalf("%s group %d = field %d, paths %d/%d, found %t; want same field and 5/5", partition.name, wantField, field.Field(), field.PathCount(), cap(field.paths), ok)
			}
			for _, index := range []int{0, 1, 2, 3, 4} {
				path, ok := field.Path(index)
				if !ok {
					t.Fatalf("%s field %d path %d is missing", partition.name, wantField, index)
				}
				step, ok := path.Step(0)
				if !ok || path.StepCount() != 1 || step.Name != wantNames[index] || path.Descendant() != wantDescendant[index] || path.Self() {
					t.Fatalf("%s field %d path %d lost its ordered name or path flags", partition.name, wantField, index)
				}
				if path.IsAttribute() != partition.attribute || path.AttributeWildcard() != partition.wildcard || path.AttributeNamespaceSet() != partition.wildcard {
					t.Fatalf("%s field %d path %d lost its attribute flags", partition.name, wantField, index)
				}
				wantAttribute := NoQName()
				if partition.attribute && !partition.wildcard {
					wantAttribute = names["id"]
				}
				if path.Attribute() != wantAttribute || path.attributeNamespace != EmptyNamespaceID {
					t.Fatalf("%s field %d path %d lost its attribute name or namespace", partition.name, wantField, index)
				}
			}
		}
	}
}

func TestValidateIdentityConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func([]IdentityConstraint, map[string]QName)
		wantErr string
		exact   bool
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
			name: "invalid kind",
			mutate: func(identities []IdentityConstraint, _ map[string]QName) {
				identities[0].Kind = IdentityKind(99)
			},
			wantErr: "identity constraint has invalid kind",
			exact:   true,
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
			},
			wantErr: "identity constraint has no fields",
		},
		{
			name: "field without paths",
			mutate: func(identities []IdentityConstraint, names map[string]QName) {
				identities[0].Fields[0].Paths = nil
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
			},
			wantErr: "identity field path has invalid shape",
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
			if err == nil || (tt.exact && err.Error() != tt.wantErr) || !strings.Contains(err.Error(), tt.wantErr) {
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

func TestIdentityReadAccessors(t *testing.T) {
	t.Parallel()

	table, identities, names := identityValidationFixture(t)
	identities[0].Selector = append(identities[0].Selector, IdentityPath{Self: true})
	identities[0].Fields[0].Paths = append(identities[0].Fields[0].Paths,
		IdentityFieldPath{Steps: []IdentityStep{{Name: names["child"]}}},
		IdentityFieldPath{Attr: true, AttrWildcard: true},
	)
	identities[0] = NewIdentityConstraint(identities[0].Kind, identities[0].Name, identities[0].Refer, identities[0].Selector, identities[0].Fields)
	dispatch := newIdentityDispatchRead(identities, newNameReadView(&table))
	invalid := IdentityConstraintID(99)
	read, readOK := dispatch.Program(0)
	if !readOK {
		t.Fatal("IdentityDispatchRead.Program() rejected valid constraint")
	}
	if _, ok := dispatch.Program(invalid); ok {
		t.Fatal("IdentityDispatchRead.Program() accepted invalid constraint")
	}

	constraints, constraintsOK := ElementIdentityConstraintIDs([][]IdentityConstraintID{{0, 1}}, 0)
	first, firstOK := constraints.At(0)
	if !constraintsOK || constraints.Len() != 2 || !firstOK || first != 0 {
		t.Fatalf("ElementIdentityConstraintIDs() = len %d first %d, %v; want 2, 0, true", constraints.Len(), first, constraintsOK)
	}
	if _, ok := ElementIdentityConstraintIDs(nil, 0); ok {
		t.Fatal("ElementIdentityConstraintIDs() accepted invalid element")
	}

	selectorPaths := read.Selectors()
	if selectorPaths.Len() != 2 {
		t.Fatalf("IdentityConstraintProgramRead.Selectors() count = %d, want %d", selectorPaths.Len(), 2)
	}

	if count := read.FieldCount(); count != 1 {
		t.Fatalf("IdentityConstraintProgramRead.FieldCount() = %d, want 1", count)
	}

	elementFields := read.ElementFields()
	if elementFields.Len() != 1 {
		t.Fatalf("IdentityConstraintProgramRead.ElementFields() count = %d, want %d", elementFields.Len(), 1)
	}
	attributeFields := read.AttributeFields(names["id"])
	if attributeFields.Len() != 1 {
		t.Fatalf("IdentityConstraintProgramRead.AttributeFields() count = %d, want %d", attributeFields.Len(), 1)
	}
	wildcardFields := read.AttributeWildcardFields()
	if wildcardFields.Len() != 1 {
		t.Fatalf("IdentityConstraintProgramRead.AttributeWildcardFields() count = %d, want %d", wildcardFields.Len(), 1)
	}
	if read.Kind() != IdentityKey || read.Refer() != NoIdentityConstraint {
		t.Fatalf("IdentityConstraintProgramRead identity info = %v, %v; want key, no reference", read.Kind(), read.Refer())
	}
}

func TestPublishedIdentityReadBounds(t *testing.T) {
	t.Parallel()
	const text = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="rows"><xs:complexType><xs:sequence>
<xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:sequence>
<xs:element name="id" type="xs:string"/>
</xs:sequence></xs:complexType></xs:element>
</xs:sequence></xs:complexType><xs:key name="key">
<xs:selector xpath="row"/><xs:field xpath="id"/>
</xs:key></xs:element></xs:schema>`
	rt, err := Compile(Options{}, []source.Source{source.Bytes("bounds.xsd", []byte(text))})
	if err != nil {
		t.Fatal(err)
	}
	rootName, rootKnown := rt.LookupQName("", "rows")
	if !rootKnown {
		t.Fatal("missing root name")
	}
	root, _, ok := rt.RootElement(RuntimeName{Name: rootName, Known: true})
	if !ok {
		t.Fatal("missing root element")
	}
	constraints, ok := rt.ElementIdentityConstraints(root)
	if !ok || constraints.Len() != 1 {
		t.Fatal("expected one attached identity")
	}
	id, ok := constraints.At(0)
	if !ok || id != 0 {
		t.Fatalf("identity = %v, %v; want 0, true", id, ok)
	}
	program, ok := rt.IdentityDispatch().Program(id)
	if !ok {
		t.Fatal("missing published identity program")
	}
	selectors, fields := program.Selectors(), program.ElementFields()
	_, selectorOK := selectors.At(0)
	field, fieldOK := fields.At(0)
	path, pathOK := field.Path(0)
	if selectors.Len() != 1 || fields.Len() != 1 || !selectorOK || !fieldOK || !pathOK || field.Field() != 0 || field.PathCount() != 1 {
		t.Fatal("expected one selector and one field with one path")
	}
	rowName, rowOK := rt.LookupQName("", "row")
	idName, idOK := rt.LookupQName("", "id")
	if !rowOK || !idOK {
		t.Fatal("missing path names")
	}
	for _, index := range []int{-1, 1} {
		if _, ok := constraints.At(index); ok {
			t.Fatalf("constraint index %d accepted", index)
		}
		if _, ok := selectors.At(index); ok {
			t.Fatalf("selector index %d accepted", index)
		}
		if _, ok := fields.At(index); ok {
			t.Fatalf("field index %d accepted", index)
		}
		if _, ok := field.Path(index); ok {
			t.Fatalf("path index %d accepted", index)
		}
		if _, ok := path.Step(index); ok {
			t.Fatalf("step index %d accepted", index)
		}
	}
	if got, ok := constraints.At(0); !ok || got != 0 {
		t.Fatal("constraint changed after rejected reads")
	}
	selector, selectorOK := selectors.At(0)
	field, fieldOK = fields.At(0)
	path, pathOK = field.Path(0)
	selectorStep, selectorStepOK := selector.Step(0)
	fieldStep, fieldStepOK := path.Step(0)
	if !selectorOK || !fieldOK || !pathOK || !selectorStepOK || !fieldStepOK || field.Field() != 0 || selectorStep.Name != rowName || fieldStep.Name != idName {
		t.Fatal("published paths changed after rejected reads")
	}
}

func TestIdentityPathProgramMatchesEmptyDirectPathDepthBounds(t *testing.T) {
	t.Parallel()

	program := identityFieldPathProgramRead(IdentityFieldPath{})
	namePath := make([]RuntimeName, 2)
	tests := []struct {
		name                    string
		baseDepth, currentDepth int
		want                    bool
	}{
		{name: "root", baseDepth: 0, currentDepth: 0, want: true},
		{name: "end", baseDepth: 2, currentDepth: 2, want: true},
		{name: "negative", baseDepth: -1, currentDepth: -1, want: false},
		{name: "past_end", baseDepth: 3, currentDepth: 3, want: false},
		{name: "different_depth", baseDepth: 1, currentDepth: 2, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := program.Matches(nil, namePath, test.baseDepth, test.currentDepth); got != test.want {
				t.Fatalf("Matches(%d, %d) = %t, want %t", test.baseDepth, test.currentDepth, got, test.want)
			}
		})
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
	return NewIdentityConstraint(kind, name, refer,
		[]IdentityPath{{Steps: []IdentityStep{{Name: selectorName}}}},
		[]IdentityField{identityAttrField(attr)},
	)
}

func identityAttrField(attr QName) IdentityField {
	return IdentityField{Paths: []IdentityFieldPath{{Attr: true, Attribute: attr}}}
}
