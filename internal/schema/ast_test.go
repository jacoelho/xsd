package schema

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaParseStateDiscardsSchemaTextAfterAdmission(t *testing.T) {
	t.Parallel()

	node := &schemaNode{}
	state := typedSchemaParseState{stack: []typedSchemaParseFrame{{node: node}}}
	for _, chunk := range [][]byte{[]byte("first"), []byte(" "), []byte("second")} {
		if err := state.chars(chunk, 1, 1); err != nil {
			t.Fatalf("chars() error = %v", err)
		}
	}
	if !node.hasNonWhitespaceText {
		t.Fatal("schema text was not recorded for admission")
	}
}

func TestNormalizeSchemaAttributeUsesExpandedNamespaceDeclarationIdentity(t *testing.T) {
	t.Parallel()

	declaration, ok := normalizeSchemaAttribute(
		xml.Name{Space: vocab.XSDNamespaceURI, Local: vocab.XSDElemSchema},
		xml.Name{Space: vocab.XMLNSNamespaceURI, Local: "p"},
		"xmlns",
	)
	if ok || declaration != (schemaAttribute{}) {
		t.Fatalf("expanded namespace declaration = (%+v, %v), want omitted", declaration, ok)
	}

	ordinary, ok := normalizeSchemaAttribute(
		xml.Name{Space: vocab.XSDNamespaceURI, Local: vocab.XSDElemSchema},
		xml.Name{Space: vocab.XMLNSPrefix, Local: "p"},
		"relative-value",
	)
	if !ok {
		t.Fatal("ordinary attribute in relative namespace was treated as a declaration")
	}
	if ordinary.Name != (xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}) || ordinary.Value != "relative-value" {
		t.Fatalf("ordinary relative-namespace attribute = %+v, want preserved expanded name/value", ordinary)
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
		got, err := tc.node.resolveQName(semanticSource[*schemaElementSource](&tc.node.semantic).Default.Value)
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
	globals := schemaGlobalNodesForTest(doc)
	if len(doc.arena) != 5 || len(globals) != 2 {
		t.Fatalf("semantic source counts = arena %d globals %d, want 5 and 2", len(doc.arena), len(globals))
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
	base, ok := restriction.semantic.derivationBase()
	if !ok || !base.Lexical.Present || base.Lexical.Value != "xs:string" {
		t.Fatalf("restriction base = %+v, %v; QName lexical whitespace must be normalized", base.Lexical, ok)
	}
	enumeration := doc.node(restriction.children[0])
	if got := semanticSource[*schemaFacetSource](&enumeration.semantic).Value; !got.Present || got.Value != " a  b " {
		t.Fatalf("enumeration value = %+v; value whitespace must remain lexical", got)
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
	for _, child := range schemaGlobalNodesForTest(doc) {
		name, _ := schemaGlobalName(&child.semantic)
		switch name.Value {
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
	if got := semanticSource[*schemaElementSource](&head.semantic).Type.Name; got.Space != "urn:test" || got.Local != "value" {
		t.Fatalf("head type = %#v, want urn:test:value", got)
	}
	if got := semanticSource[*schemaElementSource](&member.semantic).SubstitutionGroup.Name; got.Space != "urn:test" || got.Local != "head" {
		t.Fatalf("member substitution group = %#v, want urn:test:head", got)
	}
	sequence := container.firstXS(vocab.XSDElemSequence)
	sequenceChildren := slices.Collect(sequence.xsdChildren())
	if sequence == nil || sequence.semantic.particle() == nil || len(sequenceChildren) != 2 {
		t.Fatal("expected typed sequence particle")
	}
	if got := semanticSource[*schemaElementSource](&sequenceChildren[0].semantic).Ref.Name; got.Space != "urn:test" || got.Local != "member" {
		t.Fatalf("particle ref = %#v, want urn:test:member", got)
	}
	if got := semanticSource[*schemaWildcardSource](&sequenceChildren[1].semantic).ProcessContents.Value; got != "lax" {
		t.Fatalf("wildcard processContents = %q, want lax", got)
	}
	qnameAttributes := []struct {
		name    string
		body    string
		path    []string
		read    func(*schemaNode) schemaQNameAttribute
		lexical string
		want    xml.Name
	}{
		{name: "simple restriction base", body: `<xs:simpleType name="value"><xs:restriction base=" xs:string "/></xs:simpleType>`, path: []string{"simpleType", "restriction"}, read: func(n *schemaNode) schemaQNameAttribute { value, _ := n.semantic.derivationBase(); return value }, lexical: "xs:string", want: xml.Name{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}},
		{name: "complex content base", body: `<xs:complexType name="derived"><xs:complexContent><xs:extension base=" t:base "/></xs:complexContent></xs:complexType>`, path: []string{"complexType", "complexContent", "extension"}, read: func(n *schemaNode) schemaQNameAttribute { value, _ := n.semantic.derivationBase(); return value }, lexical: "t:base", want: xml.Name{Space: "urn:test", Local: "base"}},
		{name: "simple content base", body: `<xs:complexType name="derived"><xs:simpleContent><xs:restriction base=" t:base "/></xs:simpleContent></xs:complexType>`, path: []string{"complexType", "simpleContent", "restriction"}, read: func(n *schemaNode) schemaQNameAttribute { value, _ := n.semantic.derivationBase(); return value }, lexical: "t:base", want: xml.Name{Space: "urn:test", Local: "base"}},
		{name: "list itemType", body: `<xs:simpleType name="list"><xs:list itemType=" t:value "/></xs:simpleType>`, path: []string{"simpleType", "list"}, read: func(n *schemaNode) schemaQNameAttribute { return n.semantic.derivationItemType() }, lexical: "t:value", want: xml.Name{Space: "urn:test", Local: "value"}},
		{name: "element type", body: `<xs:element name="head" type=" t:value "/>`, path: []string{"element"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaElementSource](&n.semantic).Type
		}, lexical: "t:value", want: xml.Name{Space: "urn:test", Local: "value"}},
		{name: "attribute type", body: `<xs:attribute name="code" type=" t:value "/>`, path: []string{"attribute"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaAttributeSource](&n.semantic).Type
		}, lexical: "t:value", want: xml.Name{Space: "urn:test", Local: "value"}},
		{name: "element ref", body: `<xs:complexType name="container"><xs:sequence><xs:element ref=" t:member "/></xs:sequence></xs:complexType>`, path: []string{"complexType", "sequence", "element"}, read: func(n *schemaNode) schemaQNameAttribute { return semanticSource[*schemaElementSource](&n.semantic).Ref }, lexical: "t:member", want: xml.Name{Space: "urn:test", Local: "member"}},
		{name: "attribute ref", body: `<xs:complexType name="container"><xs:attribute ref=" t:code "/></xs:complexType>`, path: []string{"complexType", "attribute"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaAttributeSource](&n.semantic).Ref
		}, lexical: "t:code", want: xml.Name{Space: "urn:test", Local: "code"}},
		{name: "group ref", body: `<xs:complexType name="container"><xs:sequence><xs:group ref=" t:members "/></xs:sequence></xs:complexType>`, path: []string{"complexType", "sequence", "group"}, read: func(n *schemaNode) schemaQNameAttribute { return semanticSource[*schemaGroupSource](&n.semantic).Ref }, lexical: "t:members", want: xml.Name{Space: "urn:test", Local: "members"}},
		{name: "attributeGroup ref", body: `<xs:complexType name="container"><xs:attributeGroup ref=" t:codes "/></xs:complexType>`, path: []string{"complexType", "attributeGroup"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaAttributeGroupSource](&n.semantic).Ref
		}, lexical: "t:codes", want: xml.Name{Space: "urn:test", Local: "codes"}},
		{name: "identity refer", body: `<xs:element name="root"><xs:keyref name="reference" refer=" t:key "><xs:selector xpath="."/><xs:field xpath="@id"/></xs:keyref></xs:element>`, path: []string{"element", "keyref"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaIdentitySource](&n.semantic).Refer
		}, lexical: "t:key", want: xml.Name{Space: "urn:test", Local: "key"}},
		{name: "substitutionGroup", body: `<xs:element name="member" substitutionGroup=" t:head "/>`, path: []string{"element"}, read: func(n *schemaNode) schemaQNameAttribute {
			return semanticSource[*schemaElementSource](&n.semantic).SubstitutionGroup
		}, lexical: "t:head", want: xml.Name{Space: "urn:test", Local: "head"}},
	}
	for _, test := range qnameAttributes {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test">` + test.body + `</xs:schema>`
			parsed, parseErr := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			node := parsed.root
			for _, local := range test.path {
				node = node.firstXS(local)
				if node == nil {
					t.Fatalf("missing %s in admitted source", local)
				}
			}
			got := test.read(node)
			if !got.Lexical.Present || got.Lexical.Value != test.lexical || got.Name != test.want {
				t.Fatalf("admitted QName = %+v, want present %q and %+v", got, test.lexical, test.want)
			}
		})
	}
	t.Run("union memberTypes", func(t *testing.T) {
		t.Parallel()
		const input = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test"><xs:simpleType name="union"><xs:union memberTypes=" xs:string   t:value "/></xs:simpleType></xs:schema>`
		parsed, parseErr := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		union := parsed.root.firstXS("simpleType").firstXS("union")
		got, present := union.semantic.derivationMemberTypes()
		want := []xml.Name{{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}, {Space: "urn:test", Local: "value"}}
		if !present || !got.Lexical.Present || got.Lexical.Value != "xs:string t:value" || !slices.Equal(got.Names, want) {
			t.Fatalf("admitted members = %+v, %v, want present xs:string t:value and %+v", got, present, want)
		}
	})
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
	if semanticSource[*schemaModelSource](&all.semantic) == nil {
		t.Fatal("all has no typed model source")
	}
	children := slices.Collect(all.xsdChildren())
	if len(children) != 2 || children[0].local != elementChild || children[1].local != elementChild ||
		schemaElementName(children[0]) != "first" || schemaElementName(children[1]) != "second" {
		t.Fatalf("all model child dispatch = %#v; want two element particles", children)
	}
}

func TestSchemaSemanticQNameNamesRetainNodeBindings(t *testing.T) {
	t.Parallel()
	const input = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:default" xmlns:p="urn:root">
  <xs:element name="first" xmlns:p="urn:first" type=" p:T "/>
  <xs:element name="second" xmlns:p="urn:second" type=" p:T "/>
  <xs:element name="default" type=" T "/>
  <xs:element name="cleared" xmlns="" type=" T "/>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		lexical string
		name    xml.Name
	}{
		{lexical: "p:T", name: xml.Name{Space: "urn:first", Local: "T"}},
		{lexical: "p:T", name: xml.Name{Space: "urn:second", Local: "T"}},
		{lexical: "T", name: xml.Name{Space: "urn:default", Local: "T"}},
		{lexical: "T", name: xml.Name{Space: "", Local: "T"}},
	}
	globals := schemaGlobalNodesForTest(parsed)
	if len(globals) != len(want) {
		t.Fatalf("global declarations = %d, want %d", len(globals), len(want))
	}
	for i, node := range globals {
		got := semanticSource[*schemaElementSource](&node.semantic).Type
		if !got.Lexical.Present || got.Lexical.Value != want[i].lexical || got.Name != want[i].name {
			t.Fatalf("declaration %d QName = %+v, want present %q and %+v", i, got, want[i].lexical, want[i].name)
		}
	}
	const noDefault = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="absent" type=" T "/></xs:schema>`
	absent, err := parseSchemaDocument("absent.xsd", "absent.xsd", strings.NewReader(noDefault), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	if got := semanticSource[*schemaElementSource](&schemaGlobalNodesForTest(absent)[0].semantic).Type; !got.Lexical.Present || got.Lexical.Value != "T" || got.Name != (xml.Name{Space: "", Local: "T"}) {
		t.Fatalf("QName without default namespace = %+v, want present T in empty namespace", got)
	}
}

func schemaGlobalNodesForTest(doc *schemaDocument) []*schemaNode {
	var globals []*schemaNode
	for child := range doc.root.xsdChildren() {
		if _, global := schemaGlobalName(&child.semantic); global {
			globals = append(globals, child)
		}
	}
	return globals
}

func TestSchemaQNameAdmissionRejectsInvalidReferences(t *testing.T) {
	t.Parallel()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	categories := []struct {
		name   string
		before string
		node   string
		after  string
	}{
		{name: "base", before: `<xs:simpleType name="value">`, node: `<xs:restriction base="VALUE"/>`, after: `</xs:simpleType>`},
		{name: "itemType", before: `<xs:simpleType name="value">`, node: `<xs:list itemType="VALUE"/>`, after: `</xs:simpleType>`},
		{name: "ref", before: `<xs:complexType name="value"><xs:sequence>`, node: `<xs:element ref="VALUE"/>`, after: `</xs:sequence></xs:complexType>`},
		{name: "refer", before: `<xs:element name="value">`, node: `<xs:keyref name="reference" refer="VALUE"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:keyref>`, after: `</xs:element>`},
		{name: "substitutionGroup", node: `<xs:element name="value" substitutionGroup="VALUE"/>`},
		{name: "type", node: `<xs:element name="value" type="VALUE"/>`},
		{name: "memberTypes", before: `<xs:simpleType name="value">`, node: `<xs:union memberTypes="xs:string VALUE"/>`, after: `</xs:simpleType>`},
	}
	for _, category := range categories {
		for _, failure := range []struct {
			name    string
			lexical string
			message string
		}{
			{name: "malformed", lexical: "p:bad:name", message: "invalid QName p:bad:name"},
			{name: "unbound", lexical: "missing:T", message: "unbound QName prefix missing"},
		} {
			t.Run(category.name+"/"+failure.name, func(t *testing.T) {
				t.Parallel()
				input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + category.before + "\n  " + strings.ReplaceAll(category.node, "VALUE", failure.lexical) + category.after + `</xs:schema>`
				parsed, parseErr := parseSchemaDocument("admission.xsd", "admission.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
				if parsed != nil {
					t.Fatal("failed QName admission published a document")
				}
				diagnostic, ok := errors.AsType[*xsderrors.Error](parseErr)
				if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != failure.message || diagnostic.Path() != "admission.xsd" || diagnostic.Line() != 2 || diagnostic.Column() != 3 {
					t.Fatalf("QName admission error = %v, want schema_compile/schema_reference %q at admission.xsd:2:3", parseErr, failure.message)
				}
			})
		}
	}
}
