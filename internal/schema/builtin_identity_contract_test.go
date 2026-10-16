package schema_test

import (
	"errors"
	"reflect"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestBuiltinDeclarationIdentityRejectsConsistentRenameAndRetries(t *testing.T) {
	t.Parallel()
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	stringID, stringOK := value.BuiltinTypeID("string")
	booleanID, booleanOK := value.BuiltinTypeID("boolean")
	if !stringOK || !booleanOK {
		t.Fatal("canonical value IDs missing")
	}
	stringName, booleanName := build.SimpleTypes[stringID].Name, build.SimpleTypes[booleanID].Name
	anyTypeName, nameOK := build.Names.LookupQName(vocab.XSDNamespaceURI, "anyType")
	anyType, typeOK := build.GlobalTypes[anyTypeName]
	if !nameOK || !typeOK {
		t.Fatal("xs:anyType binding missing")
	}
	build.SimpleTypes[stringID].Name, build.SimpleTypes[booleanID].Name = booleanName, stringName
	build.GlobalTypes[stringName], build.GlobalTypes[booleanName] = xsdSchema.SimpleRef(booleanID), xsdSchema.SimpleRef(stringID)
	published, err := publishSchema(build)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if published != nil || !ok || diagnostic.Category() != xsderrors.CategoryInternal || diagnostic.Code() != xsderrors.CodeInternalInvariant || diagnostic.Message() != "builtin simple type declaration does not match its value definition" {
		t.Fatalf("renamed builtin publication: schema=%v err=%v", published, err)
	}
	if build.SimpleTypes[stringID].Name != booleanName || build.SimpleTypes[booleanID].Name != stringName || build.GlobalTypes[stringName] != xsdSchema.SimpleRef(booleanID) || build.GlobalTypes[booleanName] != xsdSchema.SimpleRef(stringID) || build.GlobalTypes[anyTypeName] != anyType {
		t.Fatal("failed publication changed declarations or bindings")
	}
	build.SimpleTypes[stringID].Name, build.SimpleTypes[booleanID].Name = stringName, booleanName
	build.GlobalTypes[stringName], build.GlobalTypes[booleanName] = xsdSchema.SimpleRef(stringID), xsdSchema.SimpleRef(booleanID)
	published, err = publishSchema(build)
	if err != nil || published == nil {
		t.Fatalf("same-build publication retry: schema=%v err=%v", published, err)
	}
	if !reflect.DeepEqual(*build, xsdSchema.SchemaBuild{}) {
		t.Fatal("successful retry did not consume the build")
	}
}

func TestAllocatedAnyTypeBindingRejectsConsistentRenameAndRetries(t *testing.T) {
	t.Parallel()
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="Other"/></xs:schema>`)
	anyName, anyNameOK := build.Names.LookupQName(vocab.XSDNamespaceURI, "anyType")
	otherName, otherNameOK := build.Names.LookupQName("", "Other")
	anyRef, anyRefOK := build.GlobalTypes[anyName]
	otherRef, otherRefOK := build.GlobalTypes[otherName]
	anyID, anyComplex := anyRef.Complex()
	otherID, otherComplex := otherRef.Complex()
	if !anyNameOK || !otherNameOK || !anyRefOK || !otherRefOK || !anyComplex || !otherComplex {
		t.Fatal("complex declaration bindings missing")
	}
	build.ComplexTypes[anyID].Name, build.ComplexTypes[otherID].Name = otherName, anyName
	build.GlobalTypes[anyName], build.GlobalTypes[otherName] = otherRef, anyRef
	published, err := publishSchema(build)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if published != nil || !ok || diagnostic.Category() != xsderrors.CategoryInternal || diagnostic.Code() != xsderrors.CodeInternalInvariant || diagnostic.Message() != "builtin anyType handle does not match global type" {
		t.Fatalf("renamed allocated builtin publication: schema=%v err=%v", published, err)
	}
	if build.ComplexTypes[anyID].Name != otherName || build.ComplexTypes[otherID].Name != anyName || build.GlobalTypes[anyName] != otherRef || build.GlobalTypes[otherName] != anyRef {
		t.Fatal("failed publication changed complex declarations or bindings")
	}
	build.ComplexTypes[anyID].Name, build.ComplexTypes[otherID].Name = anyName, otherName
	build.GlobalTypes[anyName], build.GlobalTypes[otherName] = anyRef, otherRef
	published, err = publishSchema(build)
	if err != nil || published == nil {
		t.Fatalf("same-build allocated builtin retry: schema=%v err=%v", published, err)
	}
	if !reflect.DeepEqual(*build, xsdSchema.SchemaBuild{}) {
		t.Fatal("successful retry did not consume the build")
	}
}
