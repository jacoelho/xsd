package runtime_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestRuntimeSimpleContentTypeRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:complexType name="Simple">
    <xs:simpleContent>
      <xs:extension base="Code"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Mixed" mixed="true">
    <xs:sequence><xs:element name="child" minOccurs="0"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.SimpleRef(code)); !ok || !hasSimpleContent || got != code {
		t.Fatalf("simpleContentType(simple Code) = %v, %v, %v; want %v, true, true", got, hasSimpleContent, ok, code)
	}
	simple := mustGlobalComplexType(t, rt, "Simple")
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(simple)); !ok || !hasSimpleContent || got != code {
		t.Fatalf("simpleContentType(complex Simple) = %v, %v, %v; want %v, true, true", got, hasSimpleContent, ok, code)
	}
	elementOnly := mustGlobalComplexType(t, rt, "ElementOnly")
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(elementOnly)); !ok || hasSimpleContent || got != runtime.NoSimpleType {
		t.Fatalf("simpleContentType(ElementOnly) = %v, %v, %v; want noSimpleType, false, true", got, hasSimpleContent, ok)
	}
	mixed := mustGlobalComplexType(t, rt, "Mixed")
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(mixed)); !ok || hasSimpleContent || got != runtime.NoSimpleType {
		t.Fatalf("simpleContentType(Mixed) = %v, %v, %v; want noSimpleType, false, true", got, hasSimpleContent, ok)
	}
}

func TestRuntimeSimpleContentTypeReadRejectsInvalidTypeIDs(t *testing.T) {
	rt := &runtime.Schema{
		SimpleTypePrimitives: []runtime.PrimitiveKind{runtime.PrimitiveString},
		ComplexSimpleContentReads: []runtime.SimpleContentTypeRead{
			runtime.NewSimpleContentTypeRead(runtime.SimpleContentTypeReadShape{Type: 0, Present: true}),
		},
	}
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.SimpleRef(0)); !ok || !hasSimpleContent || got != 0 {
		t.Fatalf("simpleContentType(valid simple) = %v, %v, %v; want 0, true, true", got, hasSimpleContent, ok)
	}
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(0)); !ok || !hasSimpleContent || got != 0 {
		t.Fatalf("simpleContentType(valid complex) = %v, %v, %v; want 0, true, true", got, hasSimpleContent, ok)
	}
	for _, typ := range []runtime.TypeID{runtime.SimpleRef(runtime.NoSimpleType), runtime.SimpleRef(1), runtime.ComplexRef(runtime.NoComplexType), runtime.ComplexRef(1), {}} {
		if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(typ); ok || hasSimpleContent || got != runtime.NoSimpleType {
			t.Fatalf("simpleContentType(%v) = %v, %v, %v; want noSimpleType, false, false", typ, got, hasSimpleContent, ok)
		}
	}
	rt.ComplexSimpleContentReads[0] = runtime.NewSimpleContentTypeRead(runtime.SimpleContentTypeReadShape{
		Type:    runtime.NoSimpleType,
		Present: true,
	})
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(0)); ok || hasSimpleContent || got != runtime.NoSimpleType {
		t.Fatalf("simpleContentType(invalid text type) = %v, %v, %v; want noSimpleType, false, false", got, hasSimpleContent, ok)
	}
}

func TestRuntimeSimpleContentTypeReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Simple">
    <xs:simpleContent><xs:extension base="Code"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)
	id := mustGlobalComplexType(t, rt, "Simple")

	got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(id))
	if !ok || !hasSimpleContent || got != code {
		t.Fatalf("simpleContentType(Simple) before mutation = %v, %v, %v; want %v, true, true", got, hasSimpleContent, ok, code)
	}

	rt.ComplexTypes[id].ContentKind = runtime.ContentElementOnly
	rt.ComplexTypes[id].TextType = runtime.NoSimpleType

	got, hasSimpleContent, ok = rt.SimpleContentTypeForTest(runtime.ComplexRef(id))
	if !ok || !hasSimpleContent || got != code {
		t.Fatalf("simpleContentType(Simple) after raw mutation = %v, %v, %v; want published %v, true, true", got, hasSimpleContent, ok, code)
	}
}

func TestRuntimeSimpleContentTypeRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{}
	if got, hasSimpleContent, ok := rt.SimpleContentTypeForTest(runtime.ComplexRef(0)); ok || hasSimpleContent || got != runtime.NoSimpleType {
		t.Fatalf("simpleContentType accepted missing projection = %v, %v, %v", got, hasSimpleContent, ok)
	}
}

func TestRuntimeValidationRejectsInvalidComplexSimpleContentReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Simple">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ComplexSimpleContentReads = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex simple content read projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex simple content projection invariant", err)
	}

	rt = newRuntime()
	simple := mustGlobalComplexType(t, rt, "Simple")
	rt.ComplexTypes[simple].TextType = runtime.SimpleTypeID(1 << 30)
	rt.ComplexSimpleContentReads[simple] = runtime.NewSimpleContentTypeRead(runtime.SimpleContentTypeReadShape{
		Type:    runtime.SimpleTypeID(1 << 30),
		Present: true,
	})
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex simple content read projection references invalid text type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want invalid complex simple content text type invariant", err)
	}

	rt = newRuntime()
	simple = mustGlobalComplexType(t, rt, "Simple")
	rt.ComplexSimpleContentReads[simple] = runtime.SimpleContentTypeRead{}
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex simple content read projection does not match complex type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex simple content projection mismatch invariant", err)
	}
}

func TestRuntimeElementValueConstraintsRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixedInt" type="xs:int" fixed="01"/>
  <xs:element name="defaultString" type="xs:string" default="abc"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	if _, declared, ok := rt.ElementValueConstraintsForTest(runtime.NoElement); !ok || declared {
		t.Fatalf("elementValueConstraints(noElement) = declared %v, ok %v; want false, true", declared, ok)
	}
	if _, declared, ok := rt.ElementValueConstraintsForTest(runtime.ElementID(1 << 30)); ok || declared {
		t.Fatalf("elementValueConstraints(out-of-range) = declared %v, ok %v; want false, false", declared, ok)
	}

	fixed, declared, ok := rt.ElementValueConstraintsForTest(mustGlobalElement(t, rt, "fixedInt"))
	if !ok || !declared {
		t.Fatal("elementValueConstraints(fixedInt) failed")
	}
	if fixed.OwnerType() != runtime.SimpleRef(rt.Builtin.Int) {
		t.Fatalf("fixed owner = %v, want xs:int", fixed.OwnerType())
	}
	fixedValue, ok := fixed.FixedValue()
	if !ok {
		t.Fatal("fixedInt has no fixed value")
	}
	if fixedValue.LexicalText() != "01" || fixedValue.CanonicalText() != "1" {
		t.Fatalf("fixed value = lexical %q canonical %q, want 01/1", fixedValue.LexicalText(), fixedValue.CanonicalText())
	}
	if value := fixedValue.SimpleValue(); value.Type != rt.Builtin.Int || value.Canonical != "1" {
		t.Fatalf("fixed simple value = %+v, want xs:int canonical 1", value)
	}
	_, hasDefault := fixed.DefaultValueConstraint()
	if hasDefault {
		t.Fatal("fixedInt unexpectedly has default value")
	}

	def, declared, ok := rt.ElementValueConstraintsForTest(mustGlobalElement(t, rt, "defaultString"))
	if !ok || !declared {
		t.Fatal("elementValueConstraints(defaultString) failed")
	}
	defaultValue, ok := def.DefaultValueConstraint()
	if !ok {
		t.Fatal("defaultString has no default value")
	}
	if defaultValue.LexicalText() != "abc" || defaultValue.CanonicalText() != "abc" {
		t.Fatalf("default value = lexical %q canonical %q, want abc/abc", defaultValue.LexicalText(), defaultValue.CanonicalText())
	}
}

func TestRuntimeElementValueConstraintsReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixedInt" type="xs:int" fixed="01"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalElement(t, rt, "fixedInt")

	before, declared, ok := rt.ElementValueConstraintsForTest(id)
	if !ok || !declared {
		t.Fatal("elementValueConstraints(fixedInt) failed before mutation")
	}
	if before.OwnerType() != runtime.SimpleRef(rt.Builtin.Int) {
		t.Fatalf("owner before mutation = %v, want xs:int", before.OwnerType())
	}

	rt.Elements[id].Type = runtime.SimpleRef(rt.Builtin.String)
	rt.Elements[id].Fixed = nil

	after, declared, ok := rt.ElementValueConstraintsForTest(id)
	if !ok || !declared {
		t.Fatal("elementValueConstraints(fixedInt) failed after raw element mutation")
	}
	if after.OwnerType() != runtime.SimpleRef(rt.Builtin.Int) {
		t.Fatalf("owner after raw mutation = %v, want published xs:int", after.OwnerType())
	}
	fixed, ok := after.FixedValue()
	if !ok || fixed.LexicalText() != "01" || fixed.CanonicalText() != "1" {
		t.Fatalf("fixed after raw mutation = %q/%q %v, want published 01/1", fixed.LexicalText(), fixed.CanonicalText(), ok)
	}
}

func TestRuntimeElementValueConstraintsRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{
		Elements: []runtime.ElementDecl{{Type: runtime.SimpleRef(0)}},
	}
	if constraints, declared, ok := rt.ElementValueConstraintsForTest(0); ok || declared {
		t.Fatalf("elementValueConstraints accepted missing read projection: %+v declared %v", constraints, declared)
	}
}

func TestRuntimeValidationRejectsMissingElementValueReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:string"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	rt.ElementValueConstraintReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element value read projection count does not match declarations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing element value read projection invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidElementValueReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:int" fixed="01"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalElement(t, rt, "a")
	rt.ElementValueConstraintReads[id] = runtime.NewElementValueConstraints(runtime.SimpleRef(rt.Builtin.String), runtime.ValueConstraintRead{}, false, runtime.ValueConstraintRead{}, false)

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element value read projection does not match declaration") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want element value read projection mismatch invariant", err)
	}
}

func TestRuntimeElementIdentityConstraintReadRejectsInvalidElementID(t *testing.T) {
	rt := &runtime.Schema{}
	called := false
	rt.ForEachElementIdentityConstraint(runtime.NoElement, func(runtime.IdentityConstraintID) bool {
		called = true
		return true
	})
	if called {
		t.Fatal("ForEachElementIdentityConstraint called callback for noElement")
	}

	rt.ForEachElementIdentityConstraint(0, func(runtime.IdentityConstraintID) bool {
		called = true
		return true
	})
	if called {
		t.Fatal("ForEachElementIdentityConstraint called callback for out-of-range element")
	}
}

func TestRuntimeGlobalTypeReadRejectsInvalidTypeID(t *testing.T) {
	name := runtime.QName{}
	rt := &runtime.Schema{
		GlobalTypeReads: map[runtime.QName]runtime.TypeID{name: runtime.SimpleRef(0)},
		TypeDerivations: runtime.NewTypeDerivationRead(0, []runtime.SimpleTypeDerivation{{}}, []runtime.ComplexTypeDerivation{{}}),
	}
	if got, ok := rt.Type(name); !ok || got != runtime.SimpleRef(0) {
		t.Fatalf("Type(valid simple) = %v, %v; want simpleRef(0), true", got, ok)
	}
	rt.GlobalTypeReads[name] = runtime.ComplexRef(0)
	if got, ok := rt.Type(name); !ok || got != runtime.ComplexRef(0) {
		t.Fatalf("Type(valid complex) = %v, %v; want complexRef(0), true", got, ok)
	}
	for _, typ := range []runtime.TypeID{runtime.SimpleRef(1), runtime.ComplexRef(1), {}} {
		rt.GlobalTypeReads[name] = typ
		if got, ok := rt.Type(name); ok || got != (runtime.TypeID{}) {
			t.Fatalf("Type(%v) = %v, %v; want zero, false", typ, got, ok)
		}
	}
}

func TestRuntimeTypeInfoReadRejectsInvalidTypeID(t *testing.T) {
	rt := &runtime.Schema{
		SimpleTypePrimitives: []runtime.PrimitiveKind{runtime.PrimitiveString},
		ComplexTypeInfos: []runtime.TypeInfo{{
			Block:    runtime.DerivationExtension,
			Abstract: true,
		}},
		ComplexTypes: []runtime.ComplexType{{
			Block:    runtime.DerivationExtension,
			Abstract: true,
		}},
	}
	if info, ok := rt.TypeInfo(runtime.SimpleRef(0)); !ok || info != (runtime.TypeInfo{}) {
		t.Fatalf("TypeInfo(valid simple) = %+v, %v; want zero, true", info, ok)
	}
	if info, ok := rt.TypeInfo(runtime.ComplexRef(0)); !ok ||
		info.Block != runtime.DerivationExtension || !info.Abstract {
		t.Fatalf("TypeInfo(valid complex) = %+v, %v; want block abstract, true", info, ok)
	}
	for _, typ := range []runtime.TypeID{runtime.SimpleRef(runtime.NoSimpleType), runtime.ComplexRef(runtime.NoComplexType), {}} {
		if info, ok := rt.TypeInfo(typ); ok || info != (runtime.TypeInfo{}) {
			t.Fatalf("TypeInfo(%v) = %+v, %v; want zero, false", typ, info, ok)
		}
	}
}

func TestRuntimeTypeInfoReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" abstract="true" block="extension"><xs:sequence/></xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalComplexType(t, rt, "T")

	before, ok := rt.TypeInfo(runtime.ComplexRef(id))
	if !ok || !before.Abstract || before.Block != runtime.DerivationExtension {
		t.Fatalf("TypeInfo(T) before mutation = %+v, %v; want abstract extension-blocked", before, ok)
	}

	rt.ComplexTypes[id].Abstract = false
	rt.ComplexTypes[id].Block = 0

	after, ok := rt.TypeInfo(runtime.ComplexRef(id))
	if !ok || !after.Abstract || after.Block != runtime.DerivationExtension {
		t.Fatalf("TypeInfo(T) after raw mutation = %+v, %v; want published abstract extension-blocked", after, ok)
	}
}

func TestRuntimeTypeInfoRejectsMissingProjection(t *testing.T) {
	rt := &runtime.Schema{
		SimpleTypes:  []runtime.SimpleType{{}},
		ComplexTypes: []runtime.ComplexType{{}},
	}
	if info, ok := rt.TypeInfo(runtime.ComplexRef(0)); ok {
		t.Fatalf("TypeInfo accepted missing projection: %+v", info)
	}
}

func TestRuntimeValidationRejectsInvalidComplexTypeInfoProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" abstract="true" block="extension"><xs:sequence/></xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ComplexTypeInfos = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex type info projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex type info projection invariant", err)
	}

	for _, tt := range []struct {
		name string
		mut  func(*runtime.Schema, runtime.ComplexTypeID)
	}{
		{
			name: "block",
			mut: func(rt *runtime.Schema, id runtime.ComplexTypeID) {
				rt.ComplexTypeInfos[id].Block = runtime.DerivationRestriction
			},
		},
		{
			name: "abstract",
			mut: func(rt *runtime.Schema, id runtime.ComplexTypeID) {
				rt.ComplexTypeInfos[id].Abstract = false
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rt := newRuntime()
			tt.mut(rt, mustGlobalComplexType(t, rt, "T"))
			err := runtime.ValidateSchema(rt)
			if err == nil || !strings.Contains(err.Error(), "complex type info projection does not match complex type") {
				t.Fatalf("runtime.ValidateSchema() error = %v, want complex type info projection mismatch invariant", err)
			}
		})
	}
}

func TestRuntimeTypeDerivationReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Base"><xs:sequence/></xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence/></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)
	base := mustGlobalComplexType(t, rt, "Base")
	derived := mustGlobalComplexType(t, rt, "Derived")

	if mask, ok := rt.TypeDerivation(runtime.SimpleRef(code), runtime.SimpleRef(rt.Builtin.String)); !ok || mask != runtime.DerivationRestriction {
		t.Fatalf("TypeDerivation(Code, xs:string) = %v, %v; want restriction, true", mask, ok)
	}
	if mask, ok := rt.TypeDerivation(runtime.ComplexRef(derived), runtime.ComplexRef(base)); !ok || mask != runtime.DerivationExtension {
		t.Fatalf("TypeDerivation(Derived, Base) = %v, %v; want extension, true", mask, ok)
	}

	rt.SimpleTypes[code].Base = runtime.NoSimpleType
	rt.SimpleTypes[code].Union = []runtime.SimpleTypeID{rt.Builtin.Int}
	rt.ComplexTypes[derived].Base = runtime.ComplexRef(rt.Builtin.AnyType)
	rt.ComplexTypes[derived].Derivation = runtime.DerivationKindNone

	if mask, ok := rt.TypeDerivation(runtime.SimpleRef(code), runtime.SimpleRef(rt.Builtin.String)); !ok || mask != runtime.DerivationRestriction {
		t.Fatalf("TypeDerivation(Code, xs:string) after raw mutation = %v, %v; want restriction, true", mask, ok)
	}
	if mask, ok := rt.TypeDerivation(runtime.ComplexRef(derived), runtime.ComplexRef(base)); !ok || mask != runtime.DerivationExtension {
		t.Fatalf("TypeDerivation(Derived, Base) after raw mutation = %v, %v; want extension, true", mask, ok)
	}
}

func TestRuntimeAnyTypeUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	before := rt.AnyType()

	rt.Builtin.AnyType = runtime.ComplexTypeID(1 << 30)

	if after := rt.AnyType(); after != before {
		t.Fatalf("AnyType() after raw builtin mutation = %v, want published %v", after, before)
	}
}

func TestRuntimeValidationRejectsInvalidTypeDerivationProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Base"><xs:sequence/></xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence/></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.TypeDerivations = runtime.NewTypeDerivationRead(
		rt.Builtin.AnyType,
		nil,
		runtime.NewComplexTypeDerivationsForComplexTypes(rt.ComplexTypes),
	)
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type derivation projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing simple type derivation projection invariant", err)
	}

	rt = newRuntime()
	simpleDerivations := runtime.NewSimpleTypeDerivationsForSimpleTypes(rt.SimpleTypes)
	simpleDerivations[mustGlobalCodeType(t, rt)].Base = runtime.NoSimpleType
	rt.TypeDerivations = runtime.NewTypeDerivationRead(
		rt.Builtin.AnyType,
		simpleDerivations,
		runtime.NewComplexTypeDerivationsForComplexTypes(rt.ComplexTypes),
	)
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type derivation projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple type derivation projection mismatch invariant", err)
	}

	rt = newRuntime()
	complexDerivations := runtime.NewComplexTypeDerivationsForComplexTypes(rt.ComplexTypes)
	complexDerivations[mustGlobalComplexType(t, rt, "Derived")].Kind = runtime.DerivationKindRestriction
	rt.TypeDerivations = runtime.NewTypeDerivationRead(
		rt.Builtin.AnyType,
		runtime.NewSimpleTypeDerivationsForSimpleTypes(rt.SimpleTypes),
		complexDerivations,
	)
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex type derivation projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex type derivation projection mismatch invariant", err)
	}
}

func TestRuntimeSimpleTypePrimitiveReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)

	before, ok := rt.SimpleTypePrimitive(code)
	if !ok || before != runtime.PrimitiveString {
		t.Fatalf("SimpleTypePrimitive(Code) = %v, %v; want string, true", before, ok)
	}

	rt.SimpleTypes[code].Primitive = runtime.PrimitiveDecimal

	after, ok := rt.SimpleTypePrimitive(code)
	if !ok || after != before {
		t.Fatalf("SimpleTypePrimitive(Code) after raw mutation = %v, %v; want %v, true", after, ok, before)
	}
	if primitive, ok := rt.SimpleTypePrimitive(runtime.NoSimpleType); ok || primitive != 0 {
		t.Fatalf("SimpleTypePrimitive(noSimpleType) = %v, %v; want zero, false", primitive, ok)
	}
}

func TestRuntimeValidationRejectsInvalidSimpleTypePrimitiveProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.SimpleTypePrimitives = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type primitive projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing simple type primitive projection invariant", err)
	}

	rt = newRuntime()
	rt.SimpleTypePrimitives[mustGlobalCodeType(t, rt)] = runtime.PrimitiveDecimal
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type primitive projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple type primitive projection mismatch invariant", err)
	}
}

func TestRuntimeSimpleIdentityReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:ID"/></xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)

	before := rt.SimpleIdentity(code)
	if before != runtime.SimpleIdentityID {
		t.Fatalf("SimpleIdentity(Code) = %v; want ID", before)
	}

	rt.SimpleTypes[code].Identity = runtime.SimpleIdentityNone

	if after := rt.SimpleIdentity(code); after != before {
		t.Fatalf("SimpleIdentity(Code) after raw mutation = %v; want %v", after, before)
	}
	if got := rt.SimpleIdentity(runtime.NoSimpleType); got != runtime.SimpleIdentityNone {
		t.Fatalf("SimpleIdentity(noSimpleType) = %v; want none", got)
	}
}

func TestRuntimeValidationRejectsInvalidSimpleTypeIdentityProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:ID"/></xs:simpleType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.SimpleTypeIdentities = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type identity projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing simple type identity projection invariant", err)
	}

	rt = newRuntime()
	rt.SimpleTypeIdentities[mustGlobalCodeType(t, rt)] = runtime.SimpleIdentityNone
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple type identity projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple type identity projection mismatch invariant", err)
	}
}

func TestRuntimeSimpleValueReadUsesPublishedSimpleTypes(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="ok"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)

	value, err := rt.ValidateSimpleValue(code, "ok", nil, runtime.SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(ok) before mutation error = %v", err)
	}
	if value.Canonical != "ok" {
		t.Fatalf("ValidateSimpleValue(ok) canonical = %q, want ok", value.Canonical)
	}
	_, err = rt.ValidateSimpleValue(code, "bad", nil, runtime.SimpleNeedCanonical)
	if err == nil {
		t.Fatal("ValidateSimpleValue(bad) before mutation error is nil")
	}
}

func TestRuntimeSimpleValueQNameNeedsUsePublishedSimpleTypes(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Ref"><xs:restriction base="xs:QName"/></xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	ref := simpleTypeIDByName(t, rt, "Ref")

	if len(rt.SimpleValueTypeReads) != 0 {
		t.Fatalf("SimpleValueTypeReads len = %d, want no compile-time hot type table", len(rt.SimpleValueTypeReads))
	}
	if len(rt.SimpleValueFacetReads.Index) != 0 || len(rt.SimpleValueFacetReads.Values) != 0 {
		t.Fatalf("SimpleValueFacetReads = %#v, want no duplicated cold facet table", rt.SimpleValueFacetReads)
	}
	if len(rt.SimpleValueQNameResolverNeeds) != len(rt.SimpleTypes) {
		t.Fatalf("SimpleValueQNameResolverNeeds len = %d, want %d", len(rt.SimpleValueQNameResolverNeeds), len(rt.SimpleTypes))
	}

	if err := rt.PrepareValidationHotPaths(); err != nil {
		t.Fatalf("PrepareValidationHotPaths() error = %v", err)
	}
	if len(rt.SimpleValueTypeReads) != len(rt.SimpleTypes) {
		t.Fatalf("SimpleValueTypeReads len after prepare = %d, want %d hot type reads", len(rt.SimpleValueTypeReads), len(rt.SimpleTypes))
	}
	if len(rt.SimpleValueFacetReads.Index) != 0 || len(rt.SimpleValueFacetReads.Values) != 0 {
		t.Fatalf("SimpleValueFacetReads after prepare = %#v, want no duplicated cold facet table", rt.SimpleValueFacetReads)
	}
	if !rt.SimpleValueNeedsQNameResolver(ref) {
		t.Fatal("SimpleValueNeedsQNameResolver(Ref) = false, want true")
	}

	rt.SimpleValueQNameResolverNeeds[ref] = false
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple value QName resolver projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want QName resolver projection mismatch invariant", err)
	}
}

func TestFreezeSimpleValuePayloadUsesPublishedSimpleTypes(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code"><xs:restriction base="xs:decimal"/></xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	code := mustGlobalCodeType(t, rt)
	value, err := rt.ValidateSimpleValue(code, "5.0", nil, runtime.SimpleNeedCanonical|runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}

	err = runtime.ValidateRuntimeSimpleValuePayload(rt, value, "test")
	if err != nil {
		t.Fatalf("runtime.ValidateRuntimeSimpleValuePayload() error = %v", err)
	}
}

func TestRuntimeNotationReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif" system="viewer"/>
  <xs:simpleType name="ImageKind">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="gif"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	imageKind := simpleTypeIDByName(t, rt, "ImageKind")

	value, err := rt.ValidateSimpleValue(imageKind, "gif", nil, runtime.SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(gif) before mutation error = %v", err)
	}
	if value.Canonical != "gif" {
		t.Fatalf("ValidateSimpleValue(gif) canonical = %q, want gif", value.Canonical)
	}

	rt.Notations = nil
	rt.Names = runtime.NameTable{}

	value, err = rt.ValidateSimpleValue(imageKind, "gif", nil, runtime.SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(gif) after raw mutation error = %v", err)
	}
	if value.Canonical != "gif" {
		t.Fatalf("ValidateSimpleValue(gif) after raw mutation canonical = %q, want gif", value.Canonical)
	}
}

func TestRuntimeValidationRejectsInvalidSimpleValueReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="ok"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.SimpleValueTypeReads = nil
	err := runtime.ValidateSchema(rt)
	if err != nil {
		t.Fatalf("runtime.ValidateSchema() with absent simple value read projections error = %v", err)
	}

	rt = newRuntime()
	rt.SimpleValueTypeReads = runtime.NewSimpleValueTypeReadsForSimpleTypes(rt.SimpleTypes)
	rt.SimpleValueFacetReads = runtime.NewSimpleValueFacetReadTableForSimpleTypes(rt.SimpleTypes)
	rt.SimpleValueTypeReads[mustGlobalCodeType(t, rt)].Type.Primitive = runtime.PrimitiveBoolean
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple value type read projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple value type read projection mismatch invariant", err)
	}

	rt = newRuntime()
	rt.SimpleValueTypeReads = runtime.NewSimpleValueTypeReadsForSimpleTypes(rt.SimpleTypes)
	rt.SimpleValueFacetReads = runtime.NewSimpleValueFacetReadTableForSimpleTypes(rt.SimpleTypes)
	code := mustGlobalCodeType(t, rt)
	facetIndex := rt.SimpleValueFacetReads.Index[code]
	rt.SimpleValueFacetReads.Values[facetIndex].Enumeration = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple value facet read projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple value facet projection mismatch invariant", err)
	}
}

func TestPrepareValidationHotPathsRejectsInvalidPrefilledHotTables(t *testing.T) {
	rt := engineRuntime(t, mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="ok"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	rt.SimpleValueTypeReads = runtime.NewSimpleValueTypeReadsForSimpleTypes(rt.SimpleTypes)
	rt.SimpleValueTypeReads[mustGlobalCodeType(t, rt)].Type.Primitive = runtime.PrimitiveBoolean

	err := rt.PrepareValidationHotPaths()
	if err == nil || !strings.Contains(err.Error(), "simple value type read projection does not match type") {
		t.Fatalf("PrepareValidationHotPaths() error = %v, want simple value type read projection mismatch invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidNotationReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif" system="viewer"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	rt.NotationReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "notation read map does not match notations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want notation read map invariant", err)
	}
}

func TestRuntimeGlobalElementReadRejectsInvalidElementID(t *testing.T) {
	name := runtime.QName{}
	rt := &runtime.Schema{
		GlobalElementReads: map[runtime.QName]runtime.ElementID{name: 0},
		ElementStartInfos:  []runtime.ElementStartInfo{{}},
	}
	if got, ok := rt.GlobalElement(name); !ok || got != 0 {
		t.Fatalf("GlobalElement(valid) = %v, %v; want 0, true", got, ok)
	}
	rt.GlobalElementReads[name] = 1
	if got, ok := rt.GlobalElement(name); ok || got != runtime.NoElement {
		t.Fatalf("GlobalElement(out-of-range) = %v, %v; want noElement, false", got, ok)
	}
	rt.GlobalElementReads[name] = runtime.NoElement
	if got, ok := rt.GlobalElement(name); ok || got != runtime.NoElement {
		t.Fatalf("GlobalElement(noElement) = %v, %v; want noElement, false", got, ok)
	}
}

func TestRuntimeElementStartReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int" abstract="true" nillable="true" block="extension"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalElement(t, rt, "root")
	name := mustQName(t, rt, "root")

	before, elementOK := rt.Element(id)
	if !elementOK || before.Type != runtime.SimpleRef(rt.Builtin.Int) || before.Block != runtime.DerivationExtension ||
		!before.Abstract || !before.Nillable || before.Fixed {
		t.Fatalf("Element(root) = %+v, %v; want abstract nillable xs:int extension-blocked", before, elementOK)
	}
	if root, info, rootOK := rt.RootElement(runtime.RuntimeName{Known: true, Name: name}); !rootOK || root != id || info != before {
		t.Fatalf("RootElement(root) = %v, %+v, %v; want %v, %+v, true", root, info, rootOK, id, before)
	}
	if typ, typeOK := rt.DeclaredElementTypeForTest(id); !typeOK || typ != before.Type {
		t.Fatalf("declaredElementType(root) = %v, %v; want %v, true", typ, typeOK, before.Type)
	}

	rt.Elements[id].Type = runtime.SimpleRef(rt.Builtin.String)
	rt.Elements[id].Block = 0
	rt.Elements[id].Abstract = false
	rt.Elements[id].Nillable = false
	rt.Elements[id].Fixed = &runtime.ValueConstraint{}

	after, elementOK := rt.Element(id)
	if !elementOK || after != before {
		t.Fatalf("Element(root) after raw mutation = %+v, %v; want %+v, true", after, elementOK, before)
	}
	if root, info, rootOK := rt.RootElement(runtime.RuntimeName{Known: true, Name: name}); !rootOK || root != id || info != before {
		t.Fatalf("RootElement(root) after raw mutation = %v, %+v, %v; want %v, %+v, true", root, info, rootOK, id, before)
	}
	if typ, typeOK := rt.DeclaredElementTypeForTest(id); !typeOK || typ != before.Type {
		t.Fatalf("declaredElementType(root) after raw mutation = %v, %v; want %v, true", typ, typeOK, before.Type)
	}
}

func TestRuntimeElementNameUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalElement(t, rt, "root")
	name := mustQName(t, rt, "root")

	before, ok := rt.ElementName(id)
	if !ok || before != name {
		t.Fatalf("ElementName(root) = %v, %v; want %v, true", before, ok, name)
	}

	rt.Elements[id].Name = runtime.NoQName

	if after, ok := rt.ElementName(id); !ok || after != before {
		t.Fatalf("ElementName(root) after raw mutation = %v, %v; want %v, true", after, ok, before)
	}
	if got, ok := rt.ElementName(runtime.NoElement); ok || got != (runtime.QName{}) {
		t.Fatalf("ElementName(noElement) = %v, %v; want zero, false", got, ok)
	}
}

func TestRuntimeValidationRejectsInvalidElementStartProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int" abstract="true" nillable="true" block="extension"/>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ElementStartInfos = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element start projection count does not match declarations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing element start projection invariant", err)
	}

	for _, tt := range []struct {
		name string
		mut  func(*runtime.Schema, runtime.ElementID)
	}{
		{
			name: "type",
			mut: func(rt *runtime.Schema, id runtime.ElementID) {
				rt.ElementStartInfos[id].Type = runtime.SimpleRef(rt.Builtin.String)
			},
		},
		{
			name: "block",
			mut: func(rt *runtime.Schema, id runtime.ElementID) {
				rt.ElementStartInfos[id].Block = runtime.DerivationRestriction
			},
		},
		{
			name: "abstract",
			mut: func(rt *runtime.Schema, id runtime.ElementID) {
				rt.ElementStartInfos[id].Abstract = false
			},
		},
		{
			name: "nillable",
			mut: func(rt *runtime.Schema, id runtime.ElementID) {
				rt.ElementStartInfos[id].Nillable = false
			},
		},
		{
			name: "fixed",
			mut: func(rt *runtime.Schema, id runtime.ElementID) {
				rt.ElementStartInfos[id].Fixed = true
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rt := newRuntime()
			tt.mut(rt, mustGlobalElement(t, rt, "root"))
			err := runtime.ValidateSchema(rt)
			if err == nil || !strings.Contains(err.Error(), "element start projection does not match declaration") {
				t.Fatalf("runtime.ValidateSchema() error = %v, want element start projection mismatch invariant", err)
			}
		})
	}
}

func TestRuntimeValidationRejectsInvalidElementNameProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ElementNames = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element name projection count does not match declarations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing element name projection invariant", err)
	}

	rt = newRuntime()
	rt.ElementNames[mustGlobalElement(t, rt, "root")] = runtime.NoQName
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element name projection does not match declaration") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want element name projection mismatch invariant", err)
	}
}

func TestRuntimeGlobalAttributeReadRejectsInvalidAttributeID(t *testing.T) {
	name := runtime.QName{}
	rt := &runtime.Schema{
		GlobalAttributeReads: map[runtime.QName]runtime.AttributeID{name: 0},
		AttributeDeclReads:   []runtime.AttributeDeclRead{{}},
	}
	if got, found, ok := rt.GlobalAttribute(name); !ok || !found || got != 0 {
		t.Fatalf("GlobalAttribute(valid) = %v, %v, %v; want 0, true, true", got, found, ok)
	}
	rt.GlobalAttributeReads[name] = 1
	if got, found, ok := rt.GlobalAttribute(name); ok || found || got != 0 {
		t.Fatalf("GlobalAttribute(out-of-range) = %v, %v, %v; want 0, false, false", got, found, ok)
	}
	if got, found, ok := rt.GlobalAttribute(runtime.QName{Local: 1}); !ok || found || got != 0 {
		t.Fatalf("GlobalAttribute(missing) = %v, %v, %v; want 0, false, true", got, found, ok)
	}
}

func TestRuntimeGlobalReadMapsMatchPublishedDeclarations(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T"><xs:sequence/></xs:complexType>
  <xs:element name="root" type="T"/>
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	typeName := mustQName(t, rt, "T")
	elementName := mustQName(t, rt, "root")
	attrName := mustQName(t, rt, "attr")

	beforeType, ok := rt.GlobalType(typeName)
	if !ok {
		t.Fatal("GlobalType(T) failed")
	}
	beforeElement, ok := rt.GlobalElement(elementName)
	if !ok {
		t.Fatal("GlobalElement(root) failed")
	}
	beforeAttr, found, ok := rt.GlobalAttribute(attrName)
	if !ok || !found {
		t.Fatalf("GlobalAttribute(attr) = %v, %v, %v; want found valid", beforeAttr, found, ok)
	}
	if got := rt.GlobalTypeReads[typeName]; got != beforeType {
		t.Fatalf("GlobalTypeReads[T] = %v, want %v", got, beforeType)
	}
	if got := rt.GlobalElementReads[elementName]; got != beforeElement {
		t.Fatalf("GlobalElementReads[root] = %v, want %v", got, beforeElement)
	}
	if got := rt.GlobalAttributeReads[attrName]; got != beforeAttr {
		t.Fatalf("GlobalAttributeReads[attr] = %v, want %v", got, beforeAttr)
	}
}

func TestRuntimeValidationRejectsInvalidGlobalReadMaps(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T"><xs:sequence/></xs:complexType>
  <xs:element name="root" type="T"/>
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.GlobalAttributeReads = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "global attribute read map does not match globals") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want global attribute read map invariant", err)
	}

	rt = newRuntime()
	rt.GlobalElementReads = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "global element read map does not match globals") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want global element read map invariant", err)
	}

	rt = newRuntime()
	rt.GlobalTypeReads = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "global type read map does not match globals") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want global type read map invariant", err)
	}
}

func TestRuntimeSubstitutionReadsMatchPublishedMaps(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="head" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="head"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	head := mustGlobalElement(t, rt, "head")
	member := mustGlobalElement(t, rt, "member")
	memberName := rt.Elements[member].Name

	if got, ok := rt.SubstitutionMemberByName(head, memberName); !ok || got != member {
		t.Fatalf("SubstitutionMemberByName(head, member) = %v, %v; want %v, true", got, ok, member)
	}
	var members []runtime.ElementID
	rt.ForEachSubstitutionMember(head, func(id runtime.ElementID) bool {
		members = append(members, id)
		return true
	})
	if !slices.Equal(members, []runtime.ElementID{member}) {
		t.Fatalf("ForEachSubstitutionMember(head) = %v, want [%v]", members, member)
	}
	membersByName := rt.SubstitutionMembersByName(head)
	if len(membersByName) != 1 || membersByName[memberName] != member {
		t.Fatalf("SubstitutionMembersByName(head) = %v, want {%v: %v}", membersByName, memberName, member)
	}
	if !slices.Equal(rt.SubstitutionReads[head], []runtime.ElementID{member}) {
		t.Fatalf("SubstitutionReads[head] = %v, want [%v]", rt.SubstitutionReads[head], member)
	}
	if got := rt.SubstitutionLookupReads[head][memberName]; got != member {
		t.Fatalf("SubstitutionLookupReads[head][member] = %v, want %v", got, member)
	}
	mustValidate(t, engine, `<root><member>x</member></root>`)
}

func TestRuntimeValidationRejectsInvalidSubstitutionReadMaps(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="head" type="xs:string"/>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.SubstitutionReads = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "substitution read map does not match substitutions") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want substitution read map invariant", err)
	}

	rt = newRuntime()
	rt.SubstitutionLookupReads = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "substitution lookup read map does not match lookup") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want substitution lookup read map invariant", err)
	}
}

func TestRuntimeNameReadsUsePublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:element name="root"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	before, ok := rt.LookupQName("urn:test", "root")
	if !ok {
		t.Fatal("LookupQName(urn:test, root) failed")
	}
	if got := rt.Namespace(before.Namespace); got != "urn:test" {
		t.Fatalf("Namespace(root namespace) = %q, want urn:test", got)
	}
	session, err := validate.NewSession(rt, validate.Options{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rt.Names = runtime.NameTable{}

	after, ok := rt.LookupQName("urn:test", "root")
	if !ok || after != before {
		t.Fatalf("LookupQName(urn:test, root) after raw mutation = %v, %v; want %v, true", after, ok, before)
	}
	if got := rt.Namespace(before.Namespace); got != "urn:test" {
		t.Fatalf("Namespace(root namespace) after raw mutation = %q, want urn:test", got)
	}
	if err := session.Validate(strings.NewReader(`<t:root xmlns:t="urn:test"/>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRuntimeValidationRejectsInvalidNameReadProjection(t *testing.T) {
	rt := engineRuntime(t, mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"/>
</xs:schema>`))
	rt.NameReads = runtime.NameReadView{}
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "name read projection does not match name table") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want name read projection invariant", err)
	}
}

func TestRuntimeAttributeWildcardRejectsInvalidGlobalAttributeID(t *testing.T) {
	name := runtime.QName{}
	wild := runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}
	rt := &runtime.Schema{
		GlobalAttributeReads: map[runtime.QName]runtime.AttributeID{name: 1},
		AttributeDeclReads:   []runtime.AttributeDeclRead{{}},
		Wildcards:            []runtime.Wildcard{wild},
		WildcardReads:        []runtime.WildcardView{runtime.NewWildcardView(nil, &wild)},
	}
	_, valid := validate.MatchAttributeWildcard(rt, 0, runtime.RuntimeName{
		Known: true,
		Name:  name,
		NS:    "urn:any",
		Local: "a",
	})
	if valid {
		t.Fatal("MatchAttributeWildcard accepted invalid global attribute ID")
	}
}

func TestRuntimeWildcardAttributeUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="OnlyA">
    <xs:anyAttribute namespace="urn:a" processContents="lax"/>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	set := mustAttributeUseSet(t, rt, "OnlyA")
	wildID := set.Wildcard()

	match := mustWildcardAttributeMatch(t, rt, "OnlyA", runtime.RuntimeName{NS: "urn:a", Local: "x"})
	if !match.Matched || !match.LaxMissing || match.Skip {
		t.Fatalf("wildcard before mutation = %+v, want lax missing match", match)
	}
	noMatch, valid := validate.MatchAttributeWildcard(rt, wildID, runtime.RuntimeName{NS: "urn:b", Local: "x"})
	if !valid || noMatch.Matched {
		t.Fatalf("wildcard urn:b before mutation = %+v valid %v, want no match", noMatch, valid)
	}

	rt.Wildcards[wildID] = runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessSkip}
	rt.Names = runtime.NameTable{}

	match, valid = validate.MatchAttributeWildcard(rt, wildID, runtime.RuntimeName{NS: "urn:a", Local: "x"})
	if !valid || !match.Matched || !match.LaxMissing || match.Skip {
		t.Fatalf("wildcard after raw mutation = %+v, want published lax missing match", match)
	}
	noMatch, valid = validate.MatchAttributeWildcard(rt, wildID, runtime.RuntimeName{NS: "urn:b", Local: "x"})
	if !valid || noMatch.Matched {
		t.Fatalf("wildcard urn:b after raw mutation = %+v valid %v, want published no match", noMatch, valid)
	}
}

func TestRuntimeValidationRejectsInvalidWildcardReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="OnlyA">
    <xs:anyAttribute namespace="urn:a" processContents="lax"/>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.WildcardReads = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "wildcard read projection count does not match wildcards") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing wildcard read projection invariant", err)
	}

	rt = newRuntime()
	set := mustAttributeUseSet(t, rt, "OnlyA")
	anyWildcard := runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessSkip}
	rt.WildcardReads[set.Wildcard()] = runtime.NewWildcardView(&rt.Names, &anyWildcard)
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "wildcard read projection does not match wildcard") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want wildcard read projection mismatch invariant", err)
	}
}

func TestRuntimeCompiledContentModelViewMatchesPublishedModel(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="ElementOnly"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	typ := runtime.ComplexRef(mustGlobalComplexType(t, rt, "ElementOnly"))
	modelID := rt.ContentModelForType(typ)
	before, ok := rt.CompiledContentModelView(modelID)
	if !ok {
		t.Fatalf("CompiledContentModelView(%v) failed", modelID)
	}
	if !runtime.EqualCompiledModelViewProjection(before, &rt.CompiledModels[modelID]) {
		t.Fatal("CompiledContentModelView does not match published model")
	}
	mustValidate(t, engine, `<root><child/></root>`)
	mustNotValidate(t, engine, `<root><other/></root>`, xsderrors.CodeValidationElement)
}

func TestRuntimeValidationRejectsInvalidCompiledModelViewProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.CompiledModelViews = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "compiled model view projection count does not match compiled models") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing compiled model view projection invariant", err)
	}

	rt = newRuntime()
	modelID := rt.ContentModelForType(runtime.ComplexRef(mustGlobalComplexType(t, rt, "ElementOnly")))
	rt.CompiledModelViews[modelID] = runtime.NewCompiledModelView(&runtime.CompiledModel{
		Source: modelID,
		Kind:   runtime.CompiledModelEmpty,
		Empty:  true,
	})
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "compiled model view projection does not match compiled model") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want compiled model view projection mismatch invariant", err)
	}
}

func TestRuntimeIdentityConstraintReadRejectsInvalidConstraintID(t *testing.T) {
	rt := &runtime.Schema{}
	invalid := runtime.IdentityConstraintID(0)

	called := false
	if rt.ForEachIdentitySelector(invalid, func(runtime.IdentityPath) bool {
		called = true
		return true
	}) || called {
		t.Fatal("ForEachIdentitySelector accepted invalid identity constraint")
	}
	if count, ok := rt.IdentityFieldCount(invalid); ok || count != 0 {
		t.Fatalf("IdentityFieldCount(invalid) = %d, %v; want 0, false", count, ok)
	}
	if rt.ForEachIdentityElementField(invalid, func(runtime.CompiledIdentityField) bool {
		called = true
		return true
	}) || called {
		t.Fatal("ForEachIdentityElementField accepted invalid identity constraint")
	}
	if rt.ForEachIdentityAttributeField(invalid, runtime.QName{}, func(runtime.CompiledIdentityField) bool {
		called = true
		return true
	}) || called {
		t.Fatal("ForEachIdentityAttributeField accepted invalid identity constraint")
	}
	if rt.ForEachIdentityAttributeWildcardField(invalid, func(runtime.CompiledIdentityField) bool {
		called = true
		return true
	}) || called {
		t.Fatal("ForEachIdentityAttributeWildcardField accepted invalid identity constraint")
	}
	if info, ok := rt.IdentityConstraintInfo(invalid); ok || info != (runtime.IdentityConstraintInfo{}) {
		t.Fatalf("IdentityConstraintInfo(invalid) = %+v, %v; want zero, false", info, ok)
	}
}

func TestRuntimeIdentityReadsUsePublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	root := mustGlobalElement(t, rt, "root")
	key := rt.GlobalIdentities[mustQName(t, rt, "k")]

	var constraints []runtime.IdentityConstraintID
	rt.ForEachElementIdentityConstraint(root, func(id runtime.IdentityConstraintID) bool {
		constraints = append(constraints, id)
		return true
	})
	if len(constraints) != 1 || constraints[0] != key {
		t.Fatalf("ForEachElementIdentityConstraint(root) = %v, want [%v]", constraints, key)
	}
	if count, ok := rt.IdentityFieldCount(key); !ok || count != 1 {
		t.Fatalf("IdentityFieldCount(k) = %d, %v; want 1, true", count, ok)
	}
	before, infoOK := rt.IdentityConstraintInfo(key)
	if !infoOK || before.Kind != runtime.IdentityKey || before.Refer != runtime.NoIdentityConstraint {
		t.Fatalf("IdentityConstraintInfo(k) = %+v, %v; want key with no refer", before, infoOK)
	}

	rt.Elements[root].Identity = nil
	rt.Identities[key].Fields = nil
	rt.Identities[key].Kind = runtime.IdentityKeyRef
	rt.Identities[key].Refer = key

	constraints = constraints[:0]
	rt.ForEachElementIdentityConstraint(root, func(id runtime.IdentityConstraintID) bool {
		constraints = append(constraints, id)
		return true
	})
	if len(constraints) != 1 || constraints[0] != key {
		t.Fatalf("ForEachElementIdentityConstraint(root) after raw mutation = %v, want [%v]", constraints, key)
	}
	if count, ok := rt.IdentityFieldCount(key); !ok || count != 1 {
		t.Fatalf("IdentityFieldCount(k) after raw mutation = %d, %v; want 1, true", count, ok)
	}
	after, infoOK := rt.IdentityConstraintInfo(key)
	if !infoOK || after != before {
		t.Fatalf("IdentityConstraintInfo(k) after raw mutation = %+v, %v; want %+v, true", after, infoOK, before)
	}
}

func TestRuntimeValidationRejectsInvalidIdentityReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ElementIdentityConstraintReads = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element identity constraint projection count does not match declarations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing element identity projection invariant", err)
	}

	rt = newRuntime()
	rt.ElementIdentityConstraintReads[mustGlobalElement(t, rt, "root")] = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "element identity constraint projection does not match declaration") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want element identity projection mismatch invariant", err)
	}

	rt = newRuntime()
	rt.IdentityConstraintReads = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "identity constraint read projection count does not match constraints") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing identity constraint read projection invariant", err)
	}

	rt = newRuntime()
	key := rt.GlobalIdentities[mustQName(t, rt, "k")]
	changed := rt.Identities[key]
	changed.Kind = runtime.IdentityKeyRef
	rt.IdentityConstraintReads[key] = runtime.NewIdentityConstraintRead(changed)
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "identity constraint read projection does not match constraints") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want identity constraint read projection mismatch invariant", err)
	}
}

func TestRuntimeContentFrameRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="ElementOnly"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	frame := runtime.ContentFrameForType(rt, runtime.ComplexRef(mustGlobalComplexType(t, rt, "ElementOnly")))
	if !frame.ContentState().HasModel() {
		t.Fatal("ContentFrameForType(ElementOnly) has no content model")
	}
	if frame.AllBitLen() < 0 {
		t.Fatalf("ContentFrameForType(ElementOnly) bit length = %d, want non-negative", frame.AllBitLen())
	}
	if got := runtime.ContentFrameForType(rt, runtime.SimpleRef(rt.Builtin.String)); got.ContentState().HasModel() {
		t.Fatalf("ContentFrameForType(xs:string) state = %+v, want no content model", got.ContentState())
	}
}

func TestRuntimeContentModelForTypeUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalComplexType(t, rt, "ElementOnly")

	before := rt.ContentModelForType(runtime.ComplexRef(id))
	if !runtime.ValidContentModelID(before, len(rt.CompiledModels)) {
		t.Fatalf("ContentModelForType(ElementOnly) = %v, want valid compiled model", before)
	}

	rt.ComplexTypes[id].Content = runtime.NoContentModel

	if after := rt.ContentModelForType(runtime.ComplexRef(id)); after != before {
		t.Fatalf("ContentModelForType(ElementOnly) after raw mutation = %v, want %v", after, before)
	}
	if got := rt.ContentModelForType(runtime.SimpleRef(rt.Builtin.String)); got != runtime.NoContentModel {
		t.Fatalf("ContentModelForType(xs:string) = %v, want noContentModel", got)
	}
	if got := rt.ContentModelForType(runtime.ComplexRef(runtime.NoComplexType)); got != runtime.NoContentModel {
		t.Fatalf("ContentModelForType(noComplexType) = %v, want noContentModel", got)
	}
}

func TestRuntimeValidationRejectsInvalidComplexContentModelProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ComplexContentModelIDs = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex content-model projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex content-model projection invariant", err)
	}

	rt = newRuntime()
	rt.ComplexContentModelIDs[mustGlobalComplexType(t, rt, "ElementOnly")] = runtime.NoContentModel
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex content-model projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex content-model projection mismatch invariant", err)
	}
}

func TestRuntimeSliceBackedAccessorsCloneAtPhaseBoundaries(t *testing.T) {
	t.Parallel()

	rt := &runtime.Schema{
		Models: []runtime.ContentModel{{
			Kind:         runtime.ModelSequence,
			Occurs:       runtime.Occurrence{Min: 1, Max: 1},
			Particles:    []runtime.Particle{runtime.ElementParticle(1, runtime.Occurrence{Min: 1, Max: 1})},
			ChoiceLimits: []uint32{0},
		}},
		Wildcards: []runtime.Wildcard{{
			Mode:       runtime.WildcardList,
			Namespaces: []runtime.NamespaceID{1},
			Process:    runtime.ProcessStrict,
		}},
		SimpleTypes: []runtime.SimpleType{{
			Union:   []runtime.SimpleTypeID{1},
			Variety: runtime.SimpleVarietyUnion,
		}},
	}

	model, ok := rt.ContentModel(0)
	if !ok {
		t.Fatal("ContentModel(0) failed")
	}
	model.Particles[0].Element = 9
	model.ChoiceLimits[0] = 9
	if rt.Models[0].Particles[0].Element != 1 || rt.Models[0].ChoiceLimits[0] != 0 {
		t.Fatalf("ContentModel returned table-backed slices: %#v", rt.Models[0])
	}

	wild, ok := rt.Wildcard(0)
	if !ok {
		t.Fatal("Wildcard(0) failed")
	}
	wild.Namespaces[0] = 9
	if rt.Wildcards[0].Namespaces[0] != 1 {
		t.Fatalf("Wildcard returned table-backed namespace slice: %#v", rt.Wildcards[0])
	}

	derivation, ok := rt.SimpleTypeDerivation(0)
	if !ok {
		t.Fatal("SimpleTypeDerivation(0) failed")
	}
	derivation.Union[0] = 9
	if rt.SimpleTypes[0].Union[0] != 1 {
		t.Fatalf("SimpleTypeDerivation returned table-backed union slice: %#v", rt.SimpleTypes[0].Union)
	}

	vcType, ok := rt.ValueConstraintSimpleType(0)
	if !ok {
		t.Fatal("ValueConstraintSimpleType(0) failed")
	}
	vcType.Union[0] = 9
	if rt.SimpleTypes[0].Union[0] != 1 {
		t.Fatalf("ValueConstraintSimpleType returned table-backed union slice: %#v", rt.SimpleTypes[0].Union)
	}
}

func TestRuntimeDeclaredElementTypeRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="n" type="xs:int"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	if got, ok := rt.DeclaredElementTypeForTest(mustGlobalElement(t, rt, "n")); !ok || got != runtime.SimpleRef(rt.Builtin.Int) {
		t.Fatalf("declaredElementType(n) = %v, %v; want xs:int, true", got, ok)
	}
	if got, ok := rt.DeclaredElementTypeForTest(runtime.NoElement); ok || got != (runtime.TypeID{}) {
		t.Fatalf("declaredElementType(noElement) = %v, %v; want zero, false", got, ok)
	}
}

func TestRuntimeElementChildContentRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Simple">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="ElementOnly">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	simpleTypeContent, ok := rt.ElementChildContentForTest(runtime.SimpleRef(rt.Builtin.String))
	if !ok || simpleTypeContent.IsComplexType() || simpleTypeContent.HasSimpleContent() {
		t.Fatalf("elementChildContent(xs:string) = %+v, %v; want non-complex, true", simpleTypeContent, ok)
	}
	simpleContent, ok := rt.ElementChildContentForTest(runtime.ComplexRef(mustGlobalComplexType(t, rt, "Simple")))
	if !ok || !simpleContent.IsComplexType() || !simpleContent.HasSimpleContent() {
		t.Fatalf("elementChildContent(Simple) = %+v, %v; want complex simple-content, true", simpleContent, ok)
	}
	elementOnly, ok := rt.ElementChildContentForTest(runtime.ComplexRef(mustGlobalComplexType(t, rt, "ElementOnly")))
	if !ok || !elementOnly.IsComplexType() || elementOnly.HasSimpleContent() {
		t.Fatalf("elementChildContent(ElementOnly) = %+v, %v; want complex element-content, true", elementOnly, ok)
	}
	for _, typ := range []runtime.TypeID{runtime.SimpleRef(runtime.NoSimpleType), runtime.ComplexRef(runtime.NoComplexType), {}} {
		if got, ok := rt.ElementChildContentForTest(typ); ok || got != (runtime.ElementChildContent{}) {
			t.Fatalf("elementChildContent(%v) = %+v, %v; want zero, false", typ, got, ok)
		}
	}
}

func TestRuntimeElementChildContentReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Simple">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalComplexType(t, rt, "Simple")

	before, ok := rt.ElementChildContentForTest(runtime.ComplexRef(id))
	if !ok || !before.IsComplexType() || !before.HasSimpleContent() {
		t.Fatalf("elementChildContent(Simple) before mutation = %+v, %v; want complex simple-content", before, ok)
	}

	rt.ComplexTypes[id].ContentKind = runtime.ContentElementOnly

	after, ok := rt.ElementChildContentForTest(runtime.ComplexRef(id))
	if !ok || !after.IsComplexType() || !after.HasSimpleContent() {
		t.Fatalf("elementChildContent(Simple) after raw mutation = %+v, %v; want published simple-content", after, ok)
	}
}

func TestRuntimeElementChildContentRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{}
	if content, ok := rt.ElementChildContentForTest(runtime.ComplexRef(0)); ok {
		t.Fatalf("elementChildContent accepted missing read projection: %+v", content)
	}
}

func TestRuntimeValidationRejectsMissingComplexChildContentReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	rt.ComplexChildContentReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex child content read projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex child content read projection invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidComplexChildContentReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalComplexType(t, rt, "T")
	rt.ComplexChildContentReads[id] = runtime.NewElementChildContent(runtime.ElementChildContentShape{
		Complex: true,
	})

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex child content read projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex child content read projection mismatch invariant", err)
	}
}

func TestRuntimeComplexAttributeUsesRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="req" type="xs:string" use="required"/>
    <xs:attribute name="fixed" type="xs:int" fixed="01"/>
    <xs:attribute name="defaulted" type="xs:string" default="abc"/>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	set, ok := rt.ComplexAttributeUsesForTest(mustGlobalComplexType(t, rt, "T"))
	if !ok {
		t.Fatal("complexAttributeUses(T) failed")
	}
	if set.UseCount() != 3 {
		t.Fatalf("UseCount() = %d, want 3", set.UseCount())
	}
	reqName := mustRuntimeLocalQName(t, rt, "req")
	req, reqSlot, ok := set.DeclaredUse(reqName)
	if !ok {
		t.Fatal("DeclaredUse(req) failed")
	}
	if !req.Required() || req.Name() != reqName || req.TypeID() != rt.Builtin.String || req.Label() != "req" {
		t.Fatalf("DeclaredUse(req) = %+v slot %d, want required xs:string", req, reqSlot)
	}
	var requiredSlots []int
	err := set.ForEachRequiredUse(func(slot int, use runtime.AttributeUseRead) error {
		requiredSlots = append(requiredSlots, slot)
		if !use.Required() {
			t.Fatalf("ForEachRequiredUse(%d) returned non-required use", slot)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachRequiredUse() error = %v", err)
	}
	if len(requiredSlots) != 1 || requiredSlots[0] != reqSlot {
		t.Fatalf("ForEachRequiredUse slots = %v, want [%d]", requiredSlots, reqSlot)
	}

	fixed, _, ok := set.DeclaredUse(mustRuntimeLocalQName(t, rt, "fixed"))
	if !ok {
		t.Fatal("DeclaredUse(fixed) failed")
	}
	fixedValue, ok := fixed.FixedValue()
	if !ok || fixedValue.LexicalText() != "01" || fixedValue.CanonicalText() != "1" {
		t.Fatalf("fixedValue() = %q/%q %v, want 01/1 true", fixedValue.LexicalText(), fixedValue.CanonicalText(), ok)
	}
	absentFixed, ok := fixed.AbsentValueConstraint()
	if !ok || absentFixed.CanonicalText() != "1" {
		t.Fatalf("absentValueConstraint(fixed) = %q %v, want 1 true", absentFixed.CanonicalText(), ok)
	}

	defaulted, _, ok := set.DeclaredUse(mustRuntimeLocalQName(t, rt, "defaulted"))
	if !ok {
		t.Fatal("DeclaredUse(defaulted) failed")
	}
	absentDefault, ok := defaulted.AbsentValueConstraint()
	if !ok || absentDefault.CanonicalText() != "abc" {
		t.Fatalf("absentValueConstraint(defaulted) = %q %v, want abc true", absentDefault.CanonicalText(), ok)
	}
	valueConstraintCount := 0
	err = set.ForEachValueConstraintUse(func(slot int, use runtime.AttributeUseRead) error {
		valueConstraintCount++
		if _, ok := use.AbsentValueConstraint(); !ok {
			t.Fatalf("ForEachValueConstraintUse(%d) returned unconstrained use", slot)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachValueConstraintUse() error = %v", err)
	}
	if valueConstraintCount != 2 {
		t.Fatalf("ForEachValueConstraintUse count = %d, want 2", valueConstraintCount)
	}
}

func TestRuntimeComplexAttributeUsesReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:string" use="required"/>
    <xs:attribute name="b" type="xs:string" default="abc"/>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	complexID := mustGlobalComplexType(t, rt, "T")
	setID := rt.ComplexTypes[complexID].Attrs
	name := mustRuntimeLocalQName(t, rt, "a")

	set, ok := rt.ComplexAttributeUsesForTest(complexID)
	if !ok {
		t.Fatal("complexAttributeUses(T) failed before mutation")
	}
	if _, _, declaredOK := set.DeclaredUse(name); !declaredOK {
		t.Fatal("DeclaredUse(a) failed before mutation")
	}

	rt.ComplexTypes[complexID].Attrs = runtime.NoAttributeUseSet
	delete(rt.AttributeUseSets[setID].Index, name)
	rt.AttributeUseSets[setID].Uses[0].Name = runtime.NoQName
	rt.AttributeUseSets[setID].Uses[1].Name = runtime.NoQName
	rt.AttributeUseSets[setID].Required = nil
	rt.AttributeUseSets[setID].ValueConstraints = nil

	set, ok = rt.ComplexAttributeUsesForTest(complexID)
	if !ok {
		t.Fatal("complexAttributeUses(T) failed after raw use-set mutation")
	}
	use, slot, ok := set.DeclaredUse(name)
	if !ok || slot != 0 || use.Name() != name || !use.Required() {
		t.Fatalf("DeclaredUse(a) after raw mutation = %+v slot %d %v, want published required use", use, slot, ok)
	}
	requiredCount := 0
	if err := set.ForEachRequiredUse(func(slot int, use runtime.AttributeUseRead) error {
		requiredCount++
		if slot != 0 || use.Name() != name {
			t.Fatalf("ForEachRequiredUse after raw mutation = slot %d use %v, want published use", slot, use.Name())
		}
		return nil
	}); err != nil {
		t.Fatalf("ForEachRequiredUse() after raw mutation error = %v", err)
	}
	if requiredCount != 1 {
		t.Fatalf("ForEachRequiredUse count after raw mutation = %d, want 1", requiredCount)
	}
	valueConstraintCount := 0
	if err := set.ForEachValueConstraintUse(func(slot int, use runtime.AttributeUseRead) error {
		valueConstraintCount++
		if _, ok := use.AbsentValueConstraint(); !ok {
			t.Fatalf("published use at slot %d lost absent value constraint", slot)
		}
		return nil
	}); err != nil {
		t.Fatalf("ForEachValueConstraintUse() after raw mutation error = %v", err)
	}
	if valueConstraintCount != 1 {
		t.Fatalf("ForEachValueConstraintUse count after raw mutation = %d, want 1", valueConstraintCount)
	}
}

func TestAttributeUseFixedStringFastPathUsesPublishedSimpleValueProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:string" fixed="abc"/>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	use, setID, slot := mustRawAttributeUse(t, rt, "a")
	readUse, readSlot, ok := rt.AttributeUseSetReads[setID].DeclaredUse(use.Name)
	if !ok || readSlot != slot || !readUse.CanValidateFixedStringFast() {
		t.Fatalf("published fixed-string fast path = %v slot %d %v, want true slot %d true", readUse.CanValidateFixedStringFast(), readSlot, ok, slot)
	}

	rt.SimpleTypes[use.Type].Whitespace = runtime.WhitespaceCollapse
	readUse, readSlot, ok = rt.AttributeUseSetReads[setID].DeclaredUse(use.Name)
	if !ok || readSlot != slot || !readUse.CanValidateFixedStringFast() {
		t.Fatalf("published fixed-string fast path after raw mutation = %v slot %d %v, want true slot %d true", readUse.CanValidateFixedStringFast(), readSlot, ok, slot)
	}
}

func TestRuntimeComplexAttributeUsesRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{
		ComplexAttributeUseSetIDs: []runtime.AttributeUseSetID{0},
		AttributeUseSets:          []runtime.AttributeUseSet{{}},
	}
	if set, ok := rt.ComplexAttributeUsesForTest(0); ok {
		t.Fatalf("complexAttributeUses accepted missing read projection: %+v", set)
	}
}

func TestRuntimeValidationRejectsInvalidComplexAttributeUseSetProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ComplexAttributeUseSetIDs = nil
	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex attribute use-set projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex attribute use-set projection invariant", err)
	}

	rt = newRuntime()
	rt.ComplexAttributeUseSetIDs[mustGlobalComplexType(t, rt, "T")] = runtime.NoAttributeUseSet
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex attribute use-set projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex attribute use-set projection mismatch invariant", err)
	}
}

func TestRuntimeValidationRejectsMissingAttributeUseSetReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	rt.AttributeUseSetReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "attribute use set read projection count does not match use sets") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing attribute use-set read projection invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidAttributeUseSetReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="req" type="xs:string" use="required"/>
    <xs:attribute name="fixed" type="xs:string" fixed="abc"/>
    <xs:attribute name="defaulted" type="xs:string" default="def"/>
    <xs:anyAttribute processContents="lax"/>
  </xs:complexType>
</xs:schema>`
	tests := []struct {
		name   string
		mutate func(rt *runtime.Schema, setID runtime.AttributeUseSetID, reqSlot, fixedSlot, defaultedSlot int)
	}{
		{
			name: "type",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, fixedSlot, _ int) {
				replaceAttributeUseSetReadUse(rt, setID, fixedSlot, func(shape *runtime.AttributeUseReadShape) {
					shape.Type = rt.Builtin.Int
				})
			},
		},
		{
			name: "label",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, fixedSlot, _ int) {
				replaceAttributeUseSetReadUse(rt, setID, fixedSlot, func(shape *runtime.AttributeUseReadShape) {
					shape.Label = "wrong"
				})
			},
		},
		{
			name: "fixed value",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, fixedSlot, _ int) {
				replaceAttributeUseSetReadUse(rt, setID, fixedSlot, func(shape *runtime.AttributeUseReadShape) {
					shape.Fixed = runtime.NewValueConstraintRead("wrong", "wrong", runtime.SimpleValue{Canonical: "wrong", Type: shape.Type})
				})
			},
		},
		{
			name: "default value",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, _, defaultedSlot int) {
				replaceAttributeUseSetReadUse(rt, setID, defaultedSlot, func(shape *runtime.AttributeUseReadShape) {
					shape.Default = runtime.NewValueConstraintRead("wrong", "wrong", runtime.SimpleValue{Canonical: "wrong", Type: shape.Type})
				})
			},
		},
		{
			name: "required slots",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, _, _ int) {
				replaceAttributeUseSetRead(rt, setID, func(shape *runtime.AttributeUseSetReadShape) {
					shape.Required = nil
				})
			},
		},
		{
			name: "wildcard",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, _, _ int) {
				replaceAttributeUseSetRead(rt, setID, func(shape *runtime.AttributeUseSetReadShape) {
					shape.Wildcard = runtime.NoWildcard
				})
			},
		},
		{
			name: "fixed string fast path",
			mutate: func(rt *runtime.Schema, setID runtime.AttributeUseSetID, _, fixedSlot, _ int) {
				typ := rt.AttributeUseSets[setID].Uses[fixedSlot].Type
				simpleTypes := slices.Clone(rt.SimpleTypes)
				simpleTypes[typ].Whitespace = runtime.WhitespaceCollapse
				rt.AttributeUseSetReads[setID] = runtime.NewAttributeUseSetReadForSimpleTypes(testAttributeUseSetReadShape(rt, setID), simpleTypes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := engineRuntime(t, mustCompile(t, source))
			_, setID, reqSlot := mustRawAttributeUse(t, rt, "req")
			_, fixedSetID, fixedSlot := mustRawAttributeUse(t, rt, "fixed")
			_, defaultedSetID, defaultedSlot := mustRawAttributeUse(t, rt, "defaulted")
			if fixedSetID != setID || defaultedSetID != setID {
				t.Fatalf("test setup uses multiple attribute-use sets: %v %v %v", setID, fixedSetID, defaultedSetID)
			}
			fixedRead, _, ok := rt.AttributeUseSetReads[setID].DeclaredUse(rt.AttributeUseSets[setID].Uses[fixedSlot].Name)
			if !ok || !fixedRead.CanValidateFixedStringFast() {
				t.Fatal("test setup expected published fixed xs:string attribute to use fast path")
			}
			tt.mutate(rt, setID, reqSlot, fixedSlot, defaultedSlot)

			err := runtime.ValidateSchema(rt)
			if err == nil || !strings.Contains(err.Error(), "attribute use read projection does not match use set") {
				t.Fatalf("runtime.ValidateSchema() error = %v, want attribute-use read projection invariant", err)
			}
		})
	}
}

func TestRuntimeWildcardAttributeRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="known" type="xs:int" fixed="01"/>
  <xs:complexType name="Skip"><xs:anyAttribute processContents="skip"/></xs:complexType>
  <xs:complexType name="Lax"><xs:anyAttribute processContents="lax"/></xs:complexType>
  <xs:complexType name="Strict"><xs:anyAttribute processContents="strict"/></xs:complexType>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	unknown := runtime.RuntimeName{NS: "urn:missing", Local: "a"}

	skip := mustWildcardAttributeMatch(t, rt, "Skip", unknown)
	if !skip.Matched || !skip.Skip {
		t.Fatalf("skip wildcard = %+v, want matched skip", skip)
	}
	lax := mustWildcardAttributeMatch(t, rt, "Lax", unknown)
	if !lax.Matched || !lax.LaxMissing {
		t.Fatalf("lax wildcard = %+v, want matched lax missing", lax)
	}
	strict := mustWildcardAttributeMatch(t, rt, "Strict", unknown)
	if !strict.Matched || strict.Skip || strict.LaxMissing {
		t.Fatalf("strict missing wildcard = %+v, want matched strict missing", strict)
	}

	known := runtime.RuntimeName{Known: true, Name: mustRuntimeLocalQName(t, rt, "known"), Local: "known"}
	match := mustWildcardAttributeMatch(t, rt, "Strict", known)
	if !match.Matched || !match.HasAttribute {
		t.Fatalf("strict known wildcard = %+v, want declaration", match)
	}
	decl, ok := rt.AttributeDecl(match.Attribute)
	if !ok {
		t.Fatalf("wildcard attribute %d has no declaration", match.Attribute)
	}
	if decl.Name() != known.Name || decl.TypeID() != rt.Builtin.Int {
		t.Fatalf("wildcard declaration = name %v type %v, want known xs:int", decl.Name(), decl.TypeID())
	}
	fixed, ok := decl.FixedValue()
	if !ok || fixed.CanonicalText() != "1" {
		t.Fatalf("wildcard fixedValue() = %q %v, want 1 true", fixed.CanonicalText(), ok)
	}
}

func TestRuntimeAttributeDeclReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="known" type="xs:int" fixed="01"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalAttribute(t, rt, "known")
	name := mustRuntimeLocalQName(t, rt, "known")

	before, ok := rt.AttributeDecl(id)
	if !ok {
		t.Fatal("attributeDecl(known) failed before mutation")
	}
	if before.Name() != name || before.TypeID() != rt.Builtin.Int {
		t.Fatalf("attributeDecl before mutation = name %v type %v, want known xs:int", before.Name(), before.TypeID())
	}

	rt.Attributes[id].Name = runtime.NoQName
	rt.Attributes[id].Type = rt.Builtin.String
	rt.Attributes[id].Fixed = nil

	after, ok := rt.AttributeDecl(id)
	if !ok {
		t.Fatal("attributeDecl(known) failed after raw declaration mutation")
	}
	if after.Name() != name || after.TypeID() != rt.Builtin.Int {
		t.Fatalf("attributeDecl after raw mutation = name %v type %v, want published known xs:int", after.Name(), after.TypeID())
	}
	fixed, ok := after.FixedValue()
	if !ok || fixed.LexicalText() != "01" || fixed.CanonicalText() != "1" {
		t.Fatalf("attribute fixed after raw mutation = %q/%q %v, want published 01/1", fixed.LexicalText(), fixed.CanonicalText(), ok)
	}
}

func TestRuntimeAttributeDeclRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{
		Attributes: []runtime.AttributeDecl{{Type: 0}},
	}
	if decl, ok := rt.AttributeDecl(0); ok {
		t.Fatalf("attributeDecl accepted missing read projection: %+v", decl)
	}
}

func TestRuntimeValidationRejectsMissingAttributeDeclReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	rt.AttributeDeclReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "attribute declaration read projection count does not match declarations") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing attribute declaration read projection invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidAttributeDeclReadProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:int" fixed="01"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	id := mustGlobalAttribute(t, rt, "a")
	rt.AttributeDeclReads[id] = runtime.NewAttributeDeclRead(runtime.AttributeDeclReadShape{
		Name: mustRuntimeLocalQName(t, rt, "a"),
		Type: rt.Builtin.String,
	})

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "attribute declaration read projection does not match declaration") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want attribute declaration read projection mismatch invariant", err)
	}
}

func mustWildcardAttributeMatch(t *testing.T, rt *runtime.Schema, local string, rn runtime.RuntimeName) validate.AttributeWildcardMatch {
	t.Helper()
	set := mustAttributeUseSet(t, rt, local)
	match, valid := validate.MatchAttributeWildcard(rt, set.Wildcard(), rn)
	if !valid {
		t.Fatalf("MatchAttributeWildcard(%q) returned invalid runtime state", local)
	}
	return match
}

func TestRuntimeElementTextContentRead(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="simple" type="xs:string"/>
  <xs:element name="elementOnly">
    <xs:complexType><xs:sequence/></xs:complexType>
  </xs:element>
  <xs:element name="mixed">
    <xs:complexType mixed="true"><xs:sequence minOccurs="0"><xs:element name="child"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="fixedMixed" fixed="abc">
    <xs:complexType mixed="true"><xs:sequence minOccurs="0"><xs:element name="child"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	simple := mustElementValueConstraints(t, rt, "simple")
	simpleText, ok := rt.ElementTextContentForTest(simple.OwnerType(), mustGlobalElement(t, rt, "simple"))
	if !ok {
		t.Fatal("elementTextContent(simple) failed")
	}
	if !simpleText.HasSimpleContent() || simpleText.IsComplexType() || simpleText.AllowsMixedContent() || simpleText.HasFixedElementValue() {
		t.Fatalf("elementTextContent(simple) = %+v, want simple-only", simpleText)
	}
	elementOnly := mustElementValueConstraints(t, rt, "elementOnly")
	elementOnlyText, ok := rt.ElementTextContentForTest(elementOnly.OwnerType(), mustGlobalElement(t, rt, "elementOnly"))
	if !ok {
		t.Fatal("elementTextContent(elementOnly) failed")
	}
	if elementOnlyText.HasSimpleContent() || !elementOnlyText.IsComplexType() || elementOnlyText.AllowsMixedContent() || elementOnlyText.HasFixedElementValue() {
		t.Fatalf("elementTextContent(elementOnly) = %+v, want complex element-only", elementOnlyText)
	}
	mixed := mustElementValueConstraints(t, rt, "mixed")
	mixedText, ok := rt.ElementTextContentForTest(mixed.OwnerType(), mustGlobalElement(t, rt, "mixed"))
	if !ok {
		t.Fatal("elementTextContent(mixed) failed")
	}
	if mixedText.HasSimpleContent() || !mixedText.IsComplexType() || !mixedText.AllowsMixedContent() || mixedText.HasFixedElementValue() {
		t.Fatalf("elementTextContent(mixed) = %+v, want non-fixed mixed", mixedText)
	}
	fixedMixed := mustElementValueConstraints(t, rt, "fixedMixed")
	fixedMixedText, ok := rt.ElementTextContentForTest(fixedMixed.OwnerType(), mustGlobalElement(t, rt, "fixedMixed"))
	if !ok {
		t.Fatal("elementTextContent(fixedMixed) failed")
	}
	if fixedMixedText.HasSimpleContent() || !fixedMixedText.IsComplexType() || !fixedMixedText.AllowsMixedContent() || !fixedMixedText.HasFixedElementValue() {
		t.Fatalf("elementTextContent(fixedMixed) = %+v, want fixed mixed", fixedMixedText)
	}
}

func TestRuntimeElementTextContentReadUsesPublishedProjection(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="fixedMixed" fixed="abc">
    <xs:complexType mixed="true"><xs:sequence minOccurs="0"><xs:element name="child"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	elem := mustGlobalElement(t, rt, "fixedMixed")
	typ := mustElementValueConstraints(t, rt, "fixedMixed").OwnerType()
	complexID, ok := typ.Complex()
	if !ok {
		t.Fatalf("fixedMixed owner type = %v, want complex", typ)
	}

	before, ok := rt.ElementTextContentForTest(typ, elem)
	if !ok || !before.IsComplexType() || !before.AllowsMixedContent() || !before.HasFixedElementValue() {
		t.Fatalf("elementTextContent before mutation = %+v, %v; want fixed mixed complex", before, ok)
	}

	rt.ComplexTypes[complexID].ContentKind = runtime.ContentElementOnly
	rt.Elements[elem].Fixed = nil

	after, ok := rt.ElementTextContentForTest(typ, elem)
	if !ok || !after.IsComplexType() || !after.AllowsMixedContent() || !after.HasFixedElementValue() {
		t.Fatalf("elementTextContent after raw mutation = %+v, %v; want published fixed mixed complex", after, ok)
	}
}

func TestRuntimeElementTextContentRejectsMissingReadProjection(t *testing.T) {
	rt := &runtime.Schema{
		SimpleTypePrimitives: []runtime.PrimitiveKind{runtime.PrimitiveString},
	}
	if content, ok := rt.ElementTextContentForTest(runtime.SimpleRef(0), runtime.NoElement); ok {
		t.Fatalf("elementTextContent accepted missing simple projection: %+v", content)
	}
	rt.SimpleTextContentRead = runtime.NewElementTextContentForSimpleType()
	if content, ok := rt.ElementTextContentForTest(runtime.ComplexRef(0), runtime.NoElement); ok {
		t.Fatalf("elementTextContent accepted missing complex projection: %+v", content)
	}
}

func TestRuntimeValidationRejectsMissingTextContentReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" mixed="true"><xs:sequence/></xs:complexType>
</xs:schema>`
	newRuntime := func() *runtime.Schema {
		t.Helper()
		return engineRuntime(t, mustCompile(t, source))
	}

	rt := newRuntime()
	rt.ComplexTextContentReads = nil

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex text content read projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing complex text content read projection invariant", err)
	}

	rt = newRuntime()
	rt.FixedComplexTextContentReads = nil
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "fixed complex text content read projection count does not match types") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing fixed complex text content read projection invariant", err)
	}

	rt = newRuntime()
	rt.SimpleTextContentRead = runtime.ElementTextContent{}
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple text content read projection does not match simple type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want missing simple text content read projection invariant", err)
	}
}

func TestRuntimeValidationRejectsInvalidTextContentReadProjection(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" mixed="true"><xs:sequence/></xs:complexType>
</xs:schema>`
	newRuntime := func() (*runtime.Schema, runtime.ComplexTypeID) {
		t.Helper()
		rt := engineRuntime(t, mustCompile(t, source))
		return rt, mustGlobalComplexType(t, rt, "T")
	}

	rt, id := newRuntime()
	rt.ComplexTextContentReads[id] = runtime.NewElementTextContent(runtime.ElementTextContentShape{
		Complex: true,
	})

	err := runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "complex text content read projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want complex text content read projection mismatch invariant", err)
	}

	rt, id = newRuntime()
	rt.FixedComplexTextContentReads[id] = runtime.NewElementTextContentForComplexType(rt.ComplexTypes[id], false)
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "fixed complex text content read projection does not match type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want fixed complex text content read projection mismatch invariant", err)
	}

	rt, _ = newRuntime()
	rt.SimpleTextContentRead = runtime.NewElementTextContent(runtime.ElementTextContentShape{
		Simple:  true,
		Complex: true,
	})
	err = runtime.ValidateSchema(rt)
	if err == nil || !strings.Contains(err.Error(), "simple text content read projection does not match simple type") {
		t.Fatalf("runtime.ValidateSchema() error = %v, want simple text content read projection mismatch invariant", err)
	}
}

func TestRuntimeElementTextContentRejectsInvalidMetadata(t *testing.T) {
	rt := &runtime.Schema{
		SimpleTypePrimitives:  []runtime.PrimitiveKind{runtime.PrimitiveString},
		SimpleTextContentRead: runtime.NewElementTextContentForSimpleType(),
	}
	if content, ok := rt.ElementTextContentForTest(runtime.SimpleRef(0), runtime.NoElement); !ok || !content.HasSimpleContent() {
		t.Fatalf("elementTextContent(simple/noElement) = %+v, %v; want simple content, true", content, ok)
	}
	if _, ok := rt.ElementTextContentForTest(runtime.SimpleRef(1), runtime.NoElement); ok {
		t.Fatal("elementTextContent accepted invalid simple type")
	}
	if _, ok := rt.ElementTextContentForTest(runtime.ComplexRef(1), runtime.NoElement); ok {
		t.Fatal("elementTextContent accepted invalid complex type")
	}
	if _, ok := rt.ElementTextContentForTest(runtime.SimpleRef(0), runtime.ElementID(1<<30)); ok {
		t.Fatal("elementTextContent accepted invalid element")
	}
}

func mustElementValueConstraints(t *testing.T, rt *runtime.Schema, local string) runtime.ElementValueConstraints {
	t.Helper()
	constraints, declared, ok := rt.ElementValueConstraintsForTest(mustGlobalElement(t, rt, local))
	if !ok || !declared {
		t.Fatalf("elementValueConstraints(%q) failed", local)
	}
	return constraints
}

func mustAttributeUseSet(t *testing.T, rt *runtime.Schema, local string) runtime.AttributeUseSetRead {
	t.Helper()
	set, ok := rt.ComplexAttributeUsesForTest(mustGlobalComplexType(t, rt, local))
	if !ok {
		t.Fatalf("complexAttributeUses(%q) failed", local)
	}
	return set
}

func mustRawAttributeUse(t *testing.T, rt *runtime.Schema, attrLocal string) (*runtime.AttributeUse, runtime.AttributeUseSetID, int) {
	t.Helper()
	const complexLocal = "T"
	complexID := mustGlobalComplexType(t, rt, complexLocal)
	setID := rt.ComplexTypes[complexID].Attrs
	if !runtime.ValidAttributeUseSetID(setID, len(rt.AttributeUseSets)) {
		t.Fatalf("complex type %q attribute use set = %v, want valid", complexLocal, setID)
	}
	name := mustRuntimeLocalQName(t, rt, attrLocal)
	set := &rt.AttributeUseSets[setID]
	slot, ok := set.Index[name]
	if !ok || int(slot) >= len(set.Uses) {
		t.Fatalf("attribute use %q.%q not found", complexLocal, attrLocal)
	}
	return &set.Uses[slot], setID, int(slot)
}

func replaceAttributeUseSetReadUse(rt *runtime.Schema, setID runtime.AttributeUseSetID, targetSlot int, mutate func(*runtime.AttributeUseReadShape)) {
	replaceAttributeUseSetRead(rt, setID, func(shape *runtime.AttributeUseSetReadShape) {
		useShape := testAttributeUseReadShape(rt, &rt.AttributeUseSets[setID].Uses[targetSlot])
		mutate(&useShape)
		shape.Uses[targetSlot] = useShape
	})
}

func replaceAttributeUseSetRead(rt *runtime.Schema, setID runtime.AttributeUseSetID, mutate func(*runtime.AttributeUseSetReadShape)) {
	shape := testAttributeUseSetReadShape(rt, setID)
	mutate(&shape)
	rt.AttributeUseSetReads[setID] = runtime.NewAttributeUseSetReadForSimpleTypes(shape, rt.SimpleTypes)
}

func testAttributeUseSetReadShape(rt *runtime.Schema, setID runtime.AttributeUseSetID) runtime.AttributeUseSetReadShape {
	set := &rt.AttributeUseSets[setID]
	uses := make([]runtime.AttributeUseReadShape, len(set.Uses))
	for i := range set.Uses {
		uses[i] = testAttributeUseReadShape(rt, &set.Uses[i])
	}
	return runtime.AttributeUseSetReadShape{
		Index:            set.Index,
		Uses:             uses,
		Required:         set.Required,
		ValueConstraints: set.ValueConstraints,
		Wildcard:         set.Wildcard,
	}
}

func testAttributeUseReadShape(rt *runtime.Schema, use *runtime.AttributeUse) runtime.AttributeUseReadShape {
	fixed, hasFixed := runtime.NewValueConstraintReadFromConstraint(use.Fixed)
	def, hasDefault := runtime.NewValueConstraintReadFromConstraint(use.Default)
	return runtime.AttributeUseReadShape{
		Name:       use.Name,
		Type:       use.Type,
		Label:      rt.Names.Format(use.Name),
		Fixed:      fixed,
		Default:    def,
		Required:   use.Required,
		HasFixed:   hasFixed,
		HasDefault: hasDefault,
	}
}

func mustRuntimeLocalQName(t *testing.T, rt *runtime.Schema, local string) runtime.QName {
	t.Helper()
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	return q
}

func mustGlobalCodeType(t *testing.T, rt *runtime.Schema) runtime.SimpleTypeID {
	t.Helper()
	const local = "Code"
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	typ, ok := rt.GlobalTypes[q]
	if !ok {
		t.Fatalf("GlobalTypes[%q] missing", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("GlobalTypes[%q] = %v, want simple", local, typ)
	}
	return id
}

func mustGlobalComplexType(t *testing.T, rt *runtime.Schema, local string) runtime.ComplexTypeID {
	t.Helper()
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	typ, ok := rt.GlobalTypes[q]
	if !ok {
		t.Fatalf("GlobalTypes[%q] missing", local)
	}
	id, ok := typ.Complex()
	if !ok {
		t.Fatalf("GlobalTypes[%q] = %v, want complex", local, typ)
	}
	return id
}

func mustGlobalElement(t *testing.T, rt *runtime.Schema, local string) runtime.ElementID {
	t.Helper()
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	id, ok := rt.GlobalElements[q]
	if !ok {
		t.Fatalf("GlobalElements[%q] missing", local)
	}
	return id
}

func mustGlobalAttribute(t *testing.T, rt *runtime.Schema, local string) runtime.AttributeID {
	t.Helper()
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	id, ok := rt.GlobalAttributes[q]
	if !ok {
		t.Fatalf("GlobalAttributes[%q] missing", local)
	}
	return id
}
