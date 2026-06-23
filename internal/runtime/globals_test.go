package runtime

import (
	"strings"
	"testing"
)

func TestValidateRuntimeGlobals(t *testing.T) {
	t.Parallel()

	names, qnames := runtimeGlobalsFixture(t)
	globals := RuntimeGlobals{
		GlobalAttributes: map[QName]AttributeID{qnames["attr"]: 0},
		GlobalElements:   map[QName]ElementID{qnames["elem"]: 0},
		GlobalTypes: map[QName]TypeID{
			qnames["simple"]:  SimpleRef(0),
			qnames["complex"]: ComplexRef(0),
		},
		GlobalIdentities: map[QName]IdentityConstraintID{qnames["identity"]: 0},
		Notations:        map[QName]bool{qnames["notation"]: true},
		AttributeNames:   []QName{qnames["attr"]},
		ElementNames:     []QName{qnames["elem"]},
		SimpleTypeNames:  []QName{qnames["simple"]},
		ComplexTypeNames: []QName{qnames["complex"]},
		IdentityNames:    []QName{qnames["identity"]},
	}
	if err := ValidateRuntimeGlobals(&names, globals); err != nil {
		t.Fatalf("ValidateRuntimeGlobals() error = %v", err)
	}
}

func TestElementNameReadProjectionHelpers(t *testing.T) {
	t.Parallel()

	_, qnames := runtimeGlobalsFixture(t)
	shapes := []ElementNameReadShape{
		{Name: qnames["elem"]},
		{Name: qnames["other"]},
	}

	reads := NewElementNameReads(shapes)
	if !EqualElementNameReadProjection(reads, shapes) {
		t.Fatalf("NewElementNameReads() = %v, want projection for %v", reads, shapes)
	}
	reads[1] = qnames["attr"]
	if EqualElementNameReadProjection(reads, shapes) {
		t.Fatal("EqualElementNameReadProjection() accepted mismatched name")
	}
	if EqualElementNameReadProjection(reads[:1], shapes) {
		t.Fatal("EqualElementNameReadProjection() accepted mismatched table length")
	}

	decls := []ElementDecl{{Name: qnames["elem"]}, {Name: qnames["other"]}}
	declReads := NewElementNameReadsForDecls(decls)
	if !EqualElementNameReadProjectionForDecls(declReads, decls) {
		t.Fatalf("NewElementNameReadsForDecls() = %v, want projection for %v", declReads, decls)
	}
	declReads[1] = qnames["attr"]
	if EqualElementNameReadProjectionForDecls(declReads, decls) {
		t.Fatal("EqualElementNameReadProjectionForDecls() accepted mismatched name")
	}
	if err := ValidateElementNameReadProjectionForDecls(NewElementNameReadsForDecls(decls), decls); err != nil {
		t.Fatalf("ValidateElementNameReadProjectionForDecls() error = %v", err)
	}
	if err := ValidateElementNameReadProjectionForDecls(declReads[:1], decls); err == nil || err.Error() != "element name projection count does not match declarations" {
		t.Fatalf("ValidateElementNameReadProjectionForDecls(short) error = %v, want count invariant", err)
	}
	if err := ValidateElementNameReadProjectionForDecls(declReads, decls); err == nil || err.Error() != "element name projection does not match declaration" {
		t.Fatalf("ValidateElementNameReadProjectionForDecls(changed) error = %v, want mismatch invariant", err)
	}

	if got, ok := ElementNameByID(NewElementNameReadsForDecls(decls), 1); !ok || got != decls[1].Name {
		t.Fatalf("ElementNameByID(valid) = %v, %v; want %v, true", got, ok, decls[1].Name)
	}
	for _, id := range []ElementID{NoElement, 2} {
		got, ok := ElementNameByID(NewElementNameReadsForDecls(decls), id)
		if ok || got != (QName{}) {
			t.Fatalf("ElementNameByID(%d) = %v, %v; want zero, false", id, got, ok)
		}
	}
}

func TestNewRuntimeGlobalsBuildsAndClonesProjection(t *testing.T) {
	t.Parallel()

	_, qnames := runtimeGlobalsFixture(t)
	in := RuntimeGlobalInput{
		GlobalAttributes: map[QName]AttributeID{qnames["attr"]: 0},
		GlobalElements:   map[QName]ElementID{qnames["elem"]: 0},
		GlobalTypes:      map[QName]TypeID{qnames["simple"]: SimpleRef(0)},
		GlobalIdentities: map[QName]IdentityConstraintID{qnames["identity"]: 0},
		Notations:        map[QName]bool{qnames["notation"]: true},
		Attributes:       []AttributeDecl{{Name: qnames["attr"]}},
		Elements:         []ElementDecl{{Name: qnames["elem"]}},
		SimpleTypes:      []SimpleType{{Name: qnames["simple"]}},
		ComplexTypes:     []ComplexType{{Name: qnames["complex"]}},
		Identities:       []IdentityConstraint{{Name: qnames["identity"]}},
	}

	globals := NewRuntimeGlobals(in)
	if globals.AttributeNames[0] != qnames["attr"] ||
		globals.ElementNames[0] != qnames["elem"] ||
		globals.SimpleTypeNames[0] != qnames["simple"] ||
		globals.ComplexTypeNames[0] != qnames["complex"] ||
		globals.IdentityNames[0] != qnames["identity"] {
		t.Fatalf("NewRuntimeGlobals() names = %#v", globals)
	}
	if got, ok := TypeNameByID(in.SimpleTypes, in.ComplexTypes, SimpleRef(0)); !ok || got != qnames["simple"] {
		t.Fatalf("TypeNameByID(simple) = %v, %v; want simple name, true", got, ok)
	}
	if got, ok := TypeNameByID(in.SimpleTypes, in.ComplexTypes, ComplexRef(0)); !ok || got != qnames["complex"] {
		t.Fatalf("TypeNameByID(complex) = %v, %v; want complex name, true", got, ok)
	}
	if got, ok := TypeNameByID(nil, nil, SimpleRef(0)); ok || got != (QName{}) {
		t.Fatalf("TypeNameByID(invalid) = %v, %v; want zero, false", got, ok)
	}

	in.GlobalAttributes[qnames["attr"]] = 9
	in.GlobalElements[qnames["elem"]] = 9
	in.GlobalTypes[qnames["simple"]] = ComplexRef(9)
	in.GlobalIdentities[qnames["identity"]] = 9
	in.Notations[qnames["notation"]] = false
	in.Attributes[0].Name = qnames["other"]
	in.Elements[0].Name = qnames["other"]
	in.SimpleTypes[0].Name = qnames["other"]
	in.ComplexTypes[0].Name = qnames["other"]
	in.Identities[0].Name = qnames["other"]

	if globals.GlobalAttributes[qnames["attr"]] != 0 ||
		globals.GlobalElements[qnames["elem"]] != 0 ||
		globals.GlobalTypes[qnames["simple"]] != SimpleRef(0) ||
		globals.GlobalIdentities[qnames["identity"]] != 0 ||
		!globals.Notations[qnames["notation"]] ||
		globals.AttributeNames[0] != qnames["attr"] ||
		globals.ElementNames[0] != qnames["elem"] ||
		globals.SimpleTypeNames[0] != qnames["simple"] ||
		globals.ComplexTypeNames[0] != qnames["complex"] ||
		globals.IdentityNames[0] != qnames["identity"] {
		t.Fatalf("NewRuntimeGlobals() aliased input state: %#v", globals)
	}
}

func TestValidateRuntimeGlobalsRejectsDrift(t *testing.T) {
	t.Parallel()

	names, qnames := runtimeGlobalsFixture(t)
	base := RuntimeGlobals{
		GlobalAttributes: map[QName]AttributeID{qnames["attr"]: 0},
		GlobalElements:   map[QName]ElementID{qnames["elem"]: 0},
		GlobalTypes:      map[QName]TypeID{qnames["simple"]: SimpleRef(0)},
		GlobalIdentities: map[QName]IdentityConstraintID{qnames["identity"]: 0},
		Notations:        map[QName]bool{qnames["notation"]: true},
		AttributeNames:   []QName{qnames["attr"], qnames["other"]},
		ElementNames:     []QName{qnames["elem"], qnames["other"]},
		SimpleTypeNames:  []QName{qnames["simple"], qnames["other"]},
		ComplexTypeNames: []QName{qnames["complex"]},
		IdentityNames:    []QName{qnames["identity"], qnames["other"]},
	}
	tests := []struct {
		name    string
		wantErr string
		globals RuntimeGlobals
	}{
		{
			name: "invalid attribute id",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalAttributes = map[QName]AttributeID{qnames["attr"]: 99}
				return g
			}(),
			wantErr: "global attribute references invalid declaration",
		},
		{
			name: "attribute name mismatch",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalAttributes = map[QName]AttributeID{qnames["attr"]: 1}
				return g
			}(),
			wantErr: "global attribute name does not match declaration",
		},
		{
			name: "invalid element id",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalElements = map[QName]ElementID{qnames["elem"]: 99}
				return g
			}(),
			wantErr: "global element references invalid declaration",
		},
		{
			name: "element name mismatch",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalElements = map[QName]ElementID{qnames["elem"]: 1}
				return g
			}(),
			wantErr: "global element name does not match declaration",
		},
		{
			name: "invalid type id",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalTypes = map[QName]TypeID{qnames["simple"]: SimpleRef(99)}
				return g
			}(),
			wantErr: "global type references invalid declaration",
		},
		{
			name: "invalid identity id",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalIdentities = map[QName]IdentityConstraintID{qnames["identity"]: 99}
				return g
			}(),
			wantErr: "global identity references invalid declaration",
		},
		{
			name: "type name mismatch",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalTypes = map[QName]TypeID{qnames["simple"]: SimpleRef(1)}
				return g
			}(),
			wantErr: "global type name does not match declaration",
		},
		{
			name: "identity name mismatch",
			globals: func() RuntimeGlobals {
				g := base
				g.GlobalIdentities = map[QName]IdentityConstraintID{qnames["identity"]: 1}
				return g
			}(),
			wantErr: "global identity name does not match declaration",
		},
		{
			name: "invalid notation name",
			globals: func() RuntimeGlobals {
				g := base
				g.Notations = map[QName]bool{NoQName: true}
				return g
			}(),
			wantErr: "notation references invalid name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRuntimeGlobals(&names, tt.globals)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateRuntimeGlobals() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGlobalReadMaps(t *testing.T) {
	t.Parallel()

	_, qnames := runtimeGlobalsFixture(t)
	globals := GlobalDeclarationMaps{
		Attributes: map[QName]AttributeID{qnames["attr"]: 0},
		Elements:   map[QName]ElementID{qnames["elem"]: 0},
		Types:      map[QName]TypeID{qnames["simple"]: SimpleRef(0)},
	}
	reads := NewGlobalReadMaps(globals)
	if !EqualGlobalAttributeReadMap(reads, globals) {
		t.Fatal("EqualGlobalAttributeReadMap() = false, want true")
	}
	if !EqualGlobalElementReadMap(reads, globals) {
		t.Fatal("EqualGlobalElementReadMap() = false, want true")
	}
	if !EqualGlobalTypeReadMap(reads, globals) {
		t.Fatal("EqualGlobalTypeReadMap() = false, want true")
	}
	elementInfo := ElementStartInfo{Type: SimpleRef(0)}
	elementInfos := []ElementStartInfo{elementInfo}
	if id, ok := GlobalElementByName(reads.Elements, elementInfos, qnames["elem"]); !ok || id != 0 {
		t.Fatalf("GlobalElementByName() = %v, %v; want 0, true", id, ok)
	}
	if id, info, ok := RootElementByName(reads.Elements, elementInfos, RuntimeName{Name: qnames["elem"], Known: true}); !ok || id != 0 || info != elementInfo {
		t.Fatalf("RootElementByName() = %v, %+v, %v; want 0, %+v, true", id, info, ok, elementInfo)
	}
	if id, info, ok := RootElementByName(reads.Elements, elementInfos, RuntimeName{Name: qnames["elem"]}); ok || id != NoElement || info != (ElementStartInfo{}) {
		t.Fatalf("RootElementByName(unknown) = %v, %+v, %v; want no element, zero, false", id, info, ok)
	}
	if id, ok := GlobalElementByName(map[QName]ElementID{qnames["elem"]: 99}, elementInfos, qnames["elem"]); ok || id != NoElement {
		t.Fatalf("GlobalElementByName(invalid) = %v, %v; want no element, false", id, ok)
	}
	derivations := NewTypeDerivationRead(0, []SimpleTypeDerivation{{}}, []ComplexTypeDerivation{{}})
	if typ, ok := GlobalTypeByName(reads.Types, derivations, qnames["simple"]); !ok || typ != SimpleRef(0) {
		t.Fatalf("GlobalTypeByName() = %v, %v; want simple 0, true", typ, ok)
	}
	if typ, ok := GlobalTypeByName(map[QName]TypeID{qnames["simple"]: ComplexRef(99)}, derivations, qnames["simple"]); ok || typ != (TypeID{}) {
		t.Fatalf("GlobalTypeByName(invalid) = %v, %v; want zero, false", typ, ok)
	}
	attributeDecls := []AttributeDeclRead{{}}
	if id, ok, valid := GlobalAttributeByName(reads.Attributes, attributeDecls, qnames["attr"]); !ok || !valid || id != 0 {
		t.Fatalf("GlobalAttributeByName() = %v, %v, %v; want 0, true, true", id, ok, valid)
	}
	if id, ok, valid := GlobalAttributeByName(reads.Attributes, attributeDecls, qnames["other"]); ok || !valid || id != 0 {
		t.Fatalf("GlobalAttributeByName(missing) = %v, %v, %v; want 0, false, true", id, ok, valid)
	}
	if id, ok, valid := GlobalAttributeByName(map[QName]AttributeID{qnames["attr"]: 99}, attributeDecls, qnames["attr"]); ok || valid || id != 0 {
		t.Fatalf("GlobalAttributeByName(invalid) = %v, %v, %v; want 0, false, false", id, ok, valid)
	}

	globals.Attributes[qnames["other"]] = 1
	globals.Elements[qnames["other"]] = 1
	globals.Types[qnames["other"]] = ComplexRef(0)
	if EqualGlobalAttributeReadMap(reads, globals) {
		t.Fatal("EqualGlobalAttributeReadMap() accepted attribute drift")
	}
	if EqualGlobalElementReadMap(reads, globals) {
		t.Fatal("EqualGlobalElementReadMap() accepted element drift")
	}
	if EqualGlobalTypeReadMap(reads, globals) {
		t.Fatal("EqualGlobalTypeReadMap() accepted type drift")
	}
	if reads.Attributes[qnames["other"]] != 0 || reads.Elements[qnames["other"]] != 0 || reads.Types[qnames["other"]] != (TypeID{}) {
		t.Fatalf("NewGlobalReadMaps aliased source maps: %#v", reads)
	}
	if err := ValidateGlobalReadMaps(NewGlobalReadMaps(globals), globals); err != nil {
		t.Fatalf("ValidateGlobalReadMaps() error = %v", err)
	}
	attrMismatch := NewGlobalReadMaps(globals)
	attrMismatch.Attributes = nil
	if err := ValidateGlobalReadMaps(attrMismatch, globals); err == nil || err.Error() != "global attribute read map does not match globals" {
		t.Fatalf("ValidateGlobalReadMaps(attribute) error = %v, want attribute invariant", err)
	}
	elemMismatch := NewGlobalReadMaps(globals)
	elemMismatch.Elements = nil
	if err := ValidateGlobalReadMaps(elemMismatch, globals); err == nil || err.Error() != "global element read map does not match globals" {
		t.Fatalf("ValidateGlobalReadMaps(element) error = %v, want element invariant", err)
	}
	typeMismatch := NewGlobalReadMaps(globals)
	typeMismatch.Types = nil
	if err := ValidateGlobalReadMaps(typeMismatch, globals); err == nil || err.Error() != "global type read map does not match globals" {
		t.Fatalf("ValidateGlobalReadMaps(type) error = %v, want type invariant", err)
	}
}

func TestNotationReadMap(t *testing.T) {
	t.Parallel()

	names, qnames := runtimeGlobalsFixture(t)
	notations := map[QName]bool{
		qnames["notation"]: true,
		qnames["other"]:    false,
	}
	read := NewNotationReadMap(&names, notations)
	want := ExpandedName{Namespace: EmptyNamespaceURI, Local: "notation"}
	if len(read) != 1 || !read[want] {
		t.Fatalf("NewNotationReadMap() = %#v, want only %v", read, want)
	}
	if !EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() = false, want true")
	}

	read[want] = false
	if EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted false projected notation")
	}
	read[want] = true
	read[ExpandedName{Namespace: EmptyNamespaceURI, Local: "other"}] = false
	if EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted extra projected notation")
	}
	if EqualNotationReadMap(nil, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted missing projected notation")
	}
	if EqualNotationReadMap(map[ExpandedName]bool{want: true}, nil, notations) {
		t.Fatal("EqualNotationReadMap() accepted nil name table")
	}
	if got := NewNotationReadMap(&names, map[QName]bool{qnames["other"]: false}); got != nil {
		t.Fatalf("NewNotationReadMap(false-only) = %#v, want nil", got)
	}
	if !EqualNotationReadMap(nil, &names, map[QName]bool{qnames["other"]: false}) {
		t.Fatal("EqualNotationReadMap(false-only) = false, want true")
	}
	if err := ValidateNotationReadMap(NewNotationReadMap(&names, notations), &names, notations); err != nil {
		t.Fatalf("ValidateNotationReadMap() error = %v", err)
	}
	if err := ValidateNotationReadMap(nil, &names, notations); err == nil || err.Error() != "notation read map does not match notations" {
		t.Fatalf("ValidateNotationReadMap(missing) error = %v, want notation invariant", err)
	}
}

func runtimeGlobalsFixture(t *testing.T) (NameTable, map[string]QName) {
	t.Helper()

	names, err := NewNameTable(16, []string{EmptyNamespaceURI}, []ExpandedName{
		{Namespace: EmptyNamespaceURI, Local: "attr"},
		{Namespace: EmptyNamespaceURI, Local: "elem"},
		{Namespace: EmptyNamespaceURI, Local: "simple"},
		{Namespace: EmptyNamespaceURI, Local: "complex"},
		{Namespace: EmptyNamespaceURI, Local: "identity"},
		{Namespace: EmptyNamespaceURI, Local: "notation"},
		{Namespace: EmptyNamespaceURI, Local: "other"},
	})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	qnames := make(map[string]QName)
	for _, local := range []string{"attr", "elem", "simple", "complex", "identity", "notation", "other"} {
		q, ok := names.LookupQName(EmptyNamespaceURI, local)
		if !ok {
			t.Fatalf("missing QName for %s", local)
		}
		qnames[local] = q
	}
	return names, qnames
}
