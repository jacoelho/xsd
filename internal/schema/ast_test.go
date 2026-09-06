package schema

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

func TestSchemaParseStateDiscardsSchemaTextAfterAdmission(t *testing.T) {
	t.Parallel()

	node := &schemaSyntaxNode{}
	state := schemaParseState{stack: []schemaParseFrame{{node: node}}}
	for _, chunk := range [][]byte{[]byte("first"), []byte(" "), []byte("second")} {
		if err := state.chars(chunk, xmlstream.CharacterDataText, 1, 1); err != nil {
			t.Fatalf("chars() error = %v", err)
		}
	}
	if !node.hasNonWhitespaceText {
		t.Fatal("schema text was not recorded for admission")
	}
}

func TestSchemaNodeResolveQNameReturnsXMLName(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:default" xmlns:p="urn:prefixed"><xs:element name="root" default="item"/></xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatalf("parseSchemaDocument() error = %v", err)
	}
	node := doc.node(doc.root.children[0])
	tests := []struct {
		name    string
		lexical string
		want    xml.Name
		wantErr string
	}{
		{name: "default namespace", lexical: "item", want: xml.Name{Space: "urn:default", Local: "item"}},
		{name: "prefixed namespace", lexical: "p:item", want: xml.Name{Space: "urn:prefixed", Local: "item"}},
		{name: "unbound prefix", lexical: "missing:item", wantErr: "unbound QName prefix missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := node.resolveQName(tt.lexical)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveQName(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveQName(%q) error = %v", tt.lexical, err)
			}
			if got != tt.want {
				t.Fatalf("resolveQName(%q) = %+v, want %+v", tt.lexical, got, tt.want)
			}
		})
	}
}

func TestSchemaParserReuseRetainsEarlierNamespaceContext(t *testing.T) {
	t.Parallel()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	parser := new(xmlstream.Reader)
	first, err := parseSchemaDocument("first.xsd", "first.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:first"><xs:element name="first" default="p:value"/></xs:schema>`), limits, parser)
	if err != nil {
		t.Fatalf("parse first schema: %v", err)
	}
	second, err := parseSchemaDocument("second.xsd", "second.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:second"><xs:element name="second" default="p:value"/></xs:schema>`), limits, parser)
	if err != nil {
		t.Fatalf("parse second schema: %v", err)
	}
	firstElement := first.node(first.root.children[0])
	secondElement := second.node(second.root.children[0])
	for name, tc := range map[string]struct {
		node *schemaNode
		want string
	}{
		"first":  {node: firstElement, want: "urn:first"},
		"second": {node: secondElement, want: "urn:second"},
	} {
		got, err := tc.node.resolveQName(tc.node.attrValue(vocab.XSDAttrDefault))
		if err != nil {
			t.Fatalf("resolve %s default: %v", name, err)
		}
		if got.Space != tc.want || got.Local != "value" {
			t.Fatalf("resolve %s default = %+v, want {%s}value", name, got, tc.want)
		}
	}
}

func TestSchemaSourceDocumentNormalizesAttributesOnceAndAssignsStableIDs(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p" targetNamespace=" urn:test ">
  <xs:simpleType name="value"><xs:restriction base=" xs:string "><xs:enumeration value=" a  b "/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="p:value"/>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.arena) != 5 || len(doc.globals) != 2 {
		t.Fatalf("semantic source counts = arena %d globals %d, want 5 and 2", len(doc.arena), len(doc.globals))
	}
	for i, node := range doc.arena {
		if got := node.ID(); got != schemaNodeID(i) {
			t.Fatalf("node %d ID = %d", i, got)
		}
	}
	if doc.defaults.TargetNamespace != "urn:test" {
		t.Fatalf("target namespace = %q, want normalized value", doc.defaults.TargetNamespace)
	}
	rootChild := doc.node(doc.root.children[0])
	restriction := doc.node(rootChild.children[0])
	if got, ok := restriction.attr(vocab.XSDAttrBase); !ok || got != "xs:string" {
		t.Fatalf("restriction base = %q, %v; QName lexical whitespace must be normalized", got, ok)
	}
	enumeration := doc.node(restriction.children[0])
	if got, ok := enumeration.attr(vocab.XSDAttrValue); !ok || got != " a  b " {
		t.Fatalf("enumeration value = %q, %v; value whitespace must remain lexical", got, ok)
	}
}

func TestSchemaSemanticSourceResolvesQNameAttributes(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="value"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="head" type="t:value"/>
  <xs:element name="member" type="t:value" substitutionGroup="t:head"/>
  <xs:complexType name="container"><xs:sequence><xs:element ref="t:member" minOccurs="0"/><xs:any namespace="##other" processContents="lax"/></xs:sequence></xs:complexType>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	var head, member, container *schemaNode
	for _, childID := range doc.root.children {
		child := doc.node(childID)
		switch child.attrValue(vocab.XSDAttrName) {
		case "head":
			head = child
		case "member":
			member = child
		case "container":
			container = child
		}
	}
	if head == nil || member == nil || container == nil {
		t.Fatal("expected named declarations")
	}
	if got := head.semantic.Element.Type.Name; got.Space != "urn:test" || got.Local != "value" || !head.semantic.Element.Type.Resolved {
		t.Fatalf("head type = %#v, want urn:test:value", got)
	}
	if got := member.semantic.Element.SubstitutionGroup.Name; got.Space != "urn:test" || got.Local != "head" || !member.semantic.Element.SubstitutionGroup.Resolved {
		t.Fatalf("member substitution group = %#v, want urn:test:head", got)
	}
	sequence := container.firstXS(vocab.XSDElemSequence)
	sequenceChildren := schemaModelChildren(sequence)
	if sequence == nil || sequence.semantic.Particle == nil || len(sequenceChildren) != 2 {
		t.Fatal("expected typed sequence particle")
	}
	if got := sequenceChildren[0].semantic.Element.Ref.Name; got.Space != "urn:test" || got.Local != "member" {
		t.Fatalf("particle ref = %#v, want urn:test:member", got)
	}
	if got := sequenceChildren[1].semantic.Wildcard.ProcessContents.Value; got != "lax" {
		t.Fatalf("wildcard processContents = %q, want lax", got)
	}
}

func TestSchemaSemanticSourceTypesAllModelDispatch(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:all minOccurs="0">
        <xs:element name="first"/>
        <xs:element name="second" minOccurs="0"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatalf("parseSchemaDocument() error = %v", err)
	}
	rootElement := doc.node(doc.root.children[0])
	complexType := doc.node(rootElement.children[0])
	all := doc.node(complexType.children[0])
	model, ok := schemaModelKind(all)
	if !ok || model != ModelAll {
		t.Fatalf("all model kind = %v, %v; want ModelAll, true", model, ok)
	}
	if all.semantic.Model == nil || len(all.semantic.Model.ChildIDs) != 2 {
		t.Fatalf("all model children = %#v; want two typed child IDs", all.semantic.Model)
	}
	children := schemaModelChildren(all)
	if len(children) != 2 || children[0].kind != schemaKindElement || children[1].kind != schemaKindElement {
		t.Fatalf("all model child dispatch = %#v; want two element particles", children)
	}
}
