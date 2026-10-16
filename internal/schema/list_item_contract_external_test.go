package schema_test

import (
	"errors"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestListItemCompilationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		code    xsderrors.Code
		message string
		line    int
		column  int
	}{
		{
			name: "direct builtin list",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="values"><xs:list itemType="xs:NMTOKENS"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel, message: "list item type cannot be a list type", line: 2, column: 32,
		},
		{
			name: "nested named union",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="leaf"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:simpleType name="inner"><xs:union memberTypes="xs:int leaf"/></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="inner xs:boolean"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="outer"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel, message: "list item type cannot be a list type", line: 5, column: 32,
		},
		{
			name: "anonymous nested union",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="values"><xs:list>
    <xs:simpleType><xs:union memberTypes="xs:int">
      <xs:simpleType><xs:union memberTypes="xs:boolean xs:NMTOKENS"/></xs:simpleType>
    </xs:union></xs:simpleType>
  </xs:list></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel, message: "list item type cannot be a list type", line: 2, column: 32,
		},
		{
			name: "restricted union",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:int xs:NMTOKENS"/></xs:simpleType>
  <xs:simpleType name="restricted"><xs:restriction base="inner"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="restricted"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel, message: "list item type cannot be a list type", line: 4, column: 32,
		},
		{
			name: "final list precedes list membership",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="leaf" final="list"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="leaf"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference, message: "item simple type final blocks list", line: 3, column: 32,
		},
		{
			name: "later unknown member precedes outer list membership",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="item"><xs:union memberTypes="xs:NMTOKENS xs:absent"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="item"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference, message: "unknown simple type {http://www.w3.org/2001/XMLSchema}absent", line: 2, column: 30,
		},
		{
			name: "unavailable source list inside union",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:m="urn:missing">
  <xs:import namespace="urn:missing"/>
  <xs:simpleType name="leaf"><xs:list itemType="m:absent"/></xs:simpleType>
  <xs:simpleType name="item"><xs:union memberTypes="leaf xs:int"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="item"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel, message: "list item type cannot be a list type", line: 5, column: 32,
		},
		{
			name: "self union cycle",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:union memberTypes="T"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference, message: "cyclic simple type T", line: 2, column: 3,
		},
		{
			name: "mutual union cycle",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T0"><xs:union memberTypes="T1"/></xs:simpleType>
  <xs:simpleType name="T1"><xs:union memberTypes="T0"/></xs:simpleType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference, message: "cyclic simple type T0", line: 2, column: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compiled, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("list-item.xsd", []byte(tt.schema))})
			if compiled != nil {
				t.Fatal("Compile() returned a schema for an invalid list item or dependency")
			}
			x, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || x.Category() != xsderrors.CategorySchemaCompile || x.Code() != tt.code {
				t.Fatalf("Compile() error = %v, want schema compile/%s", err, tt.code)
			}
			if x.Message() != tt.message || x.Path() != "list-item.xsd" || x.Line() != tt.line || x.Column() != tt.column || x.Unwrap() != nil {
				t.Fatalf("diagnostic = %q at %s:%d:%d, cause %v; want %q at list-item.xsd:%d:%d without cause", x.Message(), x.Path(), x.Line(), x.Column(), x.Unwrap(), tt.message, tt.line, tt.column)
			}
		})
	}
}

func TestScalarUnionIsAListItem(t *testing.T) {
	t.Parallel()
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:int xs:boolean"/></xs:simpleType>
  <xs:simpleType name="item"><xs:restriction base="inner"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="item"/></xs:simpleType>
  <xs:element name="values" type="values"/>
</xs:schema>`)
	mustValidate(t, engine, `<values>1 true 2 false</values>`)
}

func TestUnavailableScalarUnionListIsIsolated(t *testing.T) {
	t.Parallel()
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:m="urn:missing">
  <xs:import namespace="urn:missing"/>
  <xs:simpleType name="item"><xs:union memberTypes="m:absent xs:int"/></xs:simpleType>
  <xs:simpleType name="values"><xs:list itemType="item"/></xs:simpleType>
  <xs:element name="good" type="xs:int"/>
  <xs:element name="bad" type="values"/>
</xs:schema>`)
	mustValidate(t, engine, `<good>1</good>`)
	mustNotValidate(t, engine, `<bad>1</bad>`, xsderrors.CodeValidationElement)
}
