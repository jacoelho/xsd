package schema

import (
	"strings"
	"testing"
)

const detachedComplexTypeSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="b"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="live" type="B"/>
    <xs:element name="empty"><xs:complexType/></xs:element>
    <xs:element name="sequence" minOccurs="0" maxOccurs="0" INLINE_SEQUENCE
    <xs:element name="all" minOccurs="0" maxOccurs="0" INLINE_ALL
    <xs:element name="restricted" minOccurs="0" maxOccurs="0" INLINE_RESTRICTED
    <xs:element name="hollow" minOccurs="0" maxOccurs="0" INLINE_EMPTY
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

func detachedComplexTypeFixture() string {
	return replaceDetachedTypeMarkers(map[string]string{
		"INLINE_SEQUENCE":   `><xs:complexType><xs:sequence><xs:element name="c" maxOccurs="2"/></xs:sequence><xs:attribute name="a"/></xs:complexType></xs:element>`,
		"INLINE_ALL":        `><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>`,
		"INLINE_RESTRICTED": `><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>`,
		"INLINE_EMPTY":      `><xs:complexType/></xs:element>`,
	})
}

func zeroCountSimpleElementFixture() string {
	plain := ` type="xs:string"/>`
	return replaceDetachedTypeMarkers(map[string]string{"INLINE_SEQUENCE": plain, "INLINE_ALL": plain, "INLINE_RESTRICTED": plain, "INLINE_EMPTY": plain})
}

func replaceDetachedTypeMarkers(types map[string]string) string {
	text := detachedComplexTypeSchema
	for marker, inlineType := range types {
		text = strings.Replace(text, marker, inlineType, 1)
	}
	return text
}

func TestDetachedComplexTypesPublishNoContentModelOrAttributeUses(t *testing.T) {
	t.Parallel()

	publish := func(build *schemaBuild) *Schema {
		t.Helper()
		rt, err := publishSchema(build, unlimitedContentModelWork)
		if err != nil {
			t.Fatalf("publishSchema() error = %v", err)
		}
		return rt
	}
	build := compiledTestBuild(t, detachedComplexTypeFixture())
	var detached []ComplexTypeID
	for i := range build.ComplexTypes {
		if build.ComplexTypes[i].Detached {
			detached = append(detached, ComplexTypeID(i))
		}
	}
	if len(detached) != 4 {
		t.Fatalf("detached complex types = %v, want the four inline types of zero-count elements", detached)
	}
	withTypes := publish(build)
	withoutTypes := publish(compiledTestBuild(t, zeroCountSimpleElementFixture()))

	if got, want := len(withTypes.program.CompiledModels), len(withoutTypes.program.CompiledModels); got != want {
		t.Fatalf("published content models = %d, want %d as without the detached types", got, want)
	}
	if got, want := len(withTypes.program.AttributeUseSets), len(withoutTypes.program.AttributeUseSets); got != want {
		t.Fatalf("published attribute-use sets = %d, want %d as without the detached types", got, want)
	}
	for _, id := range detached {
		if info, ok := withTypes.TypeInfo(ComplexRef(id)); ok {
			t.Fatalf("TypeInfo(detached %d) = %+v, want no read", id, info)
		}
		if _, present, valid := withTypes.AttributeUseSetForType(ComplexRef(id)); !present || valid {
			t.Fatalf("AttributeUseSetForType(detached %d) = present %v valid %v, want an invalid complex read", id, present, valid)
		}
		if frame, ok := withTypes.ElementFrame(ComplexRef(id), 0); ok {
			t.Fatalf("ElementFrame(detached %d) = %+v, want no read", id, frame)
		}
	}
}

func TestPublicationRejectsElementDeclarationWithDetachedType(t *testing.T) {
	t.Parallel()

	build := compiledTestBuild(t, detachedComplexTypeFixture())
	for i := range build.ComplexTypes {
		if build.ComplexTypes[i].Detached {
			build.Elements[0].Type = ComplexRef(ComplexTypeID(i))
			break
		}
	}
	_, err := publishSchema(build, unlimitedContentModelWork)
	if err == nil || !strings.Contains(err.Error(), "detached complex type") {
		t.Fatalf("publishSchema() error = %v, want an element declaration with a detached type rejected", err)
	}
}
