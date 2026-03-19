package xsd_test

import (
	"fmt"
	"strings"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/errors"
)

func ExampleCompile() {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="message" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"simple.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	schema, err := xsd.Compile(fsys, "simple.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if schema == nil {
		fmt.Println("Error: schema is nil")
		return
	}
	fmt.Println("Schema compiled successfully")
	// Output: Schema compiled successfully
}

func ExampleSchema_Validate() {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="person">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="age" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	fsys := fstest.MapFS{
		"simple.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	schema, err := xsd.Compile(fsys, "simple.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		fmt.Printf("Error loading schema: %v\n", err)
		return
	}

	xmlDoc := `<?xml version="1.0"?>
<person xmlns="http://example.com/simple">
  <name>John Doe</name>
  <age>30</age>
</person>`

	if err := schema.Validate(strings.NewReader(xmlDoc)); err != nil {
		if violations, ok := errors.AsValidations(err); ok {
			for _, v := range violations {
				fmt.Printf("Validation: %s\n", v.Error())
			}
			return
		}
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("Document is valid")
	// Output: Document is valid
}

func ExampleSchema_NewValidator() {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="message" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"simple.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	schema, err := xsd.Compile(fsys, "simple.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		fmt.Printf("Error loading schema: %v\n", err)
		return
	}

	v, err := schema.NewValidator(xsd.NewValidateOptions())
	if err != nil {
		fmt.Printf("Error creating validator: %v\n", err)
		return
	}

	xmlDoc := `<?xml version="1.0"?>
<message xmlns="http://example.com/simple">hello</message>`

	if err := v.Validate(strings.NewReader(xmlDoc)); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("Document is valid")
	// Output: Document is valid
}
