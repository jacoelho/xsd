package xsd_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// Line and column values below assume one element per document line; they
// pin where the validator reports each identity diagnostic.

type selfSelectorDiagnostic struct {
	Code    xsderrors.Code
	Path    string
	Line    int
	Column  int
	Message string
}

type selfSelectorCase struct {
	name string
	xml  string
	want []selfSelectorDiagnostic
}

// requireSelfSelectorCases validates every case on one session and, after each
// failing case, revalidates valid on the same session so leaked identity state
// would surface.
func requireSelfSelectorCases(t *testing.T, schema, valid string, cases []selfSelectorCase) {
	t.Helper()
	session, err := compileIdentityDispatchTestEngine(t, schema).NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		got, err := selfSelectorDiagnostics(session.Validate(strings.NewReader(test.xml)))
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if !slices.Equal(got, test.want) {
			t.Fatalf("%s: diagnostics =\n%+v\nwant\n%+v", test.name, got, test.want)
		}
		if len(test.want) == 0 {
			continue
		}
		if err := session.Validate(strings.NewReader(valid)); err != nil {
			t.Fatalf("%s: valid document after failure: %v", test.name, err)
		}
	}
}

func selfSelectorDiagnostics(err error) ([]selfSelectorDiagnostic, error) {
	var got []selfSelectorDiagnostic
	for _, child := range xsderrors.Flatten(err) {
		diagnostic, ok := errors.AsType[*xsderrors.Error](child)
		if !ok {
			return nil, fmt.Errorf("unstructured diagnostic: %w", child)
		}
		got = append(got, selfSelectorDiagnostic{
			Code:    diagnostic.Code(),
			Path:    diagnostic.Path(),
			Line:    diagnostic.Line(),
			Column:  diagnostic.Column(),
			Message: diagnostic.Message(),
		})
	}
	return got, nil
}

func identityDiagnostic(path string, line, column int, message string) selfSelectorDiagnostic {
	return selfSelectorDiagnostic{Code: xsderrors.CodeValidationIdentity, Path: path, Line: line, Column: column, Message: message}
}

func keyRequiresValue(constraint string) string {
	return fmt.Sprintf("identity constraint %q, field 1: a key requires a value, but the selected node is absent or nilled", constraint)
}

func keyrefUnresolved(constraint string) string {
	return fmt.Sprintf("keyref value does not match any value of referenced identity constraint %q", constraint)
}

func duplicateValue(constraint, firstSeen string) string {
	return fmt.Sprintf("duplicate value for identity constraint %q; first seen at %s", constraint, firstSeen)
}

// siblingSelfScopeSchema declares count sibling elements s0..s<count-1>, each
// with its own self-selector key on @id and keyref on @ref.
func siblingSelfScopeSchema(count int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence>`)
	for i := range count {
		fmt.Fprintf(&b, `<xs:element name="s%[1]d"><xs:complexType>`+
			`<xs:attribute name="id" type="xs:string"/><xs:attribute name="ref" type="xs:string"/></xs:complexType>`+
			`<xs:key name="k%[1]d"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>`+
			`<xs:keyref name="r%[1]d" refer="k%[1]d"><xs:selector xpath="."/><xs:field xpath="@ref"/></xs:keyref>`+
			`</xs:element>`, i)
	}
	b.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

// siblingSelfScopeDocument writes one sibling per line; attributes returns
// the attribute text of sibling i.
func siblingSelfScopeDocument(count int, attributes func(i int) string) string {
	var b strings.Builder
	b.WriteString("<root>\n")
	for i := range count {
		fmt.Fprintf(&b, "<s%d %s/>\n", i, attributes(i))
	}
	b.WriteString("</root>")
	return b.String()
}

func TestIdentitySelfSelectorSiblingScopesAreIndependent(t *testing.T) {
	t.Parallel()

	resolved := func(int) string { return `id="a" ref="a"` }
	valid := siblingSelfScopeDocument(3, resolved)
	requireSelfSelectorCases(t, siblingSelfScopeSchema(3), valid, []selfSelectorCase{
		{name: "same key in every sibling", xml: valid},
		{
			name: "one unresolved sibling ref",
			xml: siblingSelfScopeDocument(3, func(i int) string {
				if i == 1 {
					return `id="a" ref="zz"`
				}
				return resolved(i)
			}),
			want: []selfSelectorDiagnostic{identityDiagnostic("/root/s1", 3, 1, keyrefUnresolved("k1"))},
		},
		{
			name: "sibling key does not resolve another sibling's ref",
			xml: siblingSelfScopeDocument(3, func(i int) string {
				if i == 1 {
					return `ref="a"`
				}
				return `id="a"`
			}),
			want: []selfSelectorDiagnostic{
				identityDiagnostic("/root/s1", 3, 13, keyRequiresValue("k1")),
				identityDiagnostic("/root/s1", 3, 1, keyrefUnresolved("k1")),
			},
		},
	})

	const many = 64
	requireSelfSelectorCases(t, siblingSelfScopeSchema(many), siblingSelfScopeDocument(many, resolved), []selfSelectorCase{{
		name: "only the last of many siblings is unresolved",
		xml: siblingSelfScopeDocument(many, func(i int) string {
			if i == many-1 {
				return `id="a" ref="zz"`
			}
			return resolved(i)
		}),
		want: []selfSelectorDiagnostic{identityDiagnostic(fmt.Sprintf("/root/s%d", many-1), many+1, 1, keyrefUnresolved(fmt.Sprintf("k%d", many-1)))},
	}})
}

func TestIdentitySelfUnionSelectorSiblingsDetectOwnDuplicates(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="s" maxOccurs="unbounded">
      <xs:complexType>
        <xs:sequence>
          <xs:element name="c" minOccurs="0"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element>
        </xs:sequence>
        <xs:attribute name="id" type="xs:string"/>
      </xs:complexType>
      <xs:unique name="u"><xs:selector xpath=".|c"/><xs:field xpath="@id"/></xs:unique>
    </xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	const valid = "<root>\n<s id=\"a\"><c id=\"b\"/></s>\n<s id=\"a\"/>\n</root>"
	requireSelfSelectorCases(t, schema, valid, []selfSelectorCase{{
		name: "each sibling checks its own element and child",
		xml:  "<root>\n<s id=\"a\"><c id=\"b\"/></s>\n<s id=\"a\"><c id=\"a\"/></s>\n<s id=\"a\"/>\n<s id=\"x\"><c id=\"x\"/></s>\n</root>",
		want: []selfSelectorDiagnostic{
			identityDiagnostic("/root/s", 3, 22, duplicateValue("u", "/root/s/c")),
			identityDiagnostic("/root/s", 5, 22, duplicateValue("u", "/root/s/c")),
		},
	}})
}

func TestIdentitySelfKeyWithAttributeFieldReportsOnlyMissingSibling(t *testing.T) {
	t.Parallel()

	schema := func(constraint string) string {
		return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="s" maxOccurs="unbounded">
      <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
      <xs:` + constraint + ` name="k"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:` + constraint + `>
    </xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	}
	const valid = "<root>\n<s id=\"a\"/>\n<s id=\"b\"/>\n<s id=\"c\"/>\n</root>"
	const missing = "<root>\n<s id=\"a\"/>\n<s/>\n<s id=\"a\"/>\n</root>"
	requireSelfSelectorCases(t, schema("key"), valid, []selfSelectorCase{
		{name: "distinct ids", xml: valid},
		{name: "repeated id in separate scopes", xml: "<root>\n<s id=\"a\"/>\n<s id=\"a\"/>\n<s id=\"a\"/>\n</root>"},
		{
			name: "one sibling without the key",
			xml:  missing,
			want: []selfSelectorDiagnostic{identityDiagnostic("/root/s", 3, 4, keyRequiresValue("k"))},
		},
	})
	requireSelfSelectorCases(t, schema("unique"), valid, []selfSelectorCase{
		{name: "unique allows the missing field", xml: missing},
	})
}

// descendantSelfSchema declares root > x > y, each optional and carrying @id;
// root declares constraint (with its fields) over selector.
func descendantSelfSchema(constraint, selector string) string {
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="x" minOccurs="0">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="y" minOccurs="0"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element>
            </xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
    <xs:` + constraint + ` name="u"><xs:selector xpath="` + selector + `"/><xs:field xpath="@id"/></xs:` + constraint + `>
  </xs:element>
</xs:schema>`
}

func TestIdentityDescendantSelfSelectorDetectsDuplicateAcrossDepths(t *testing.T) {
	t.Parallel()

	const valid = "<root id=\"a\">\n<x id=\"b\">\n<y id=\"c\"/>\n</x>\n</root>"
	requireSelfSelectorCases(t, descendantSelfSchema("unique", ".//."), valid, []selfSelectorCase{
		{name: "distinct ids", xml: valid},
		{
			name: "scope element and grandchild share an id",
			xml:  "<root id=\"a\">\n<x id=\"b\">\n<y id=\"a\"/>\n</x>\n</root>",
			want: []selfSelectorDiagnostic{identityDiagnostic("/root", 5, 1, duplicateValue("u", "/root/x/y"))},
		},
		{
			name: "scope element and child share an id",
			xml:  "<root id=\"a\">\n<x id=\"a\"/>\n</root>",
			want: []selfSelectorDiagnostic{identityDiagnostic("/root", 3, 1, duplicateValue("u", "/root/x"))},
		},
	})
}

func TestIdentitySelfAndDescendantSelfBranchesSelectNodeOnce(t *testing.T) {
	t.Parallel()

	const valid = `<root id="a"/>`
	requireSelfSelectorCases(t, descendantSelfSchema("key", ". | .//. | ."), valid, []selfSelectorCase{
		{name: "scope element alone", xml: valid},
		{name: "scope element and child", xml: `<root id="a"><x id="b"/></root>`},
	})
}

func TestIdentityNestedSelfScopesSelectOnlyOwnElement(t *testing.T) {
	t.Parallel()

	schema := func(constraints string) string {
		return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="root" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
    ` + constraints + `
  </xs:element>
</xs:schema>`
	}
	const inner = `<xs:key name="inner"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>`
	const valid = "<root id=\"a\">\n<root id=\"b\"/>\n</root>"
	requireSelfSelectorCases(t, schema(inner), valid, []selfSelectorCase{{
		name: "outer self scope does not select nested elements",
		xml:  "<root id=\"a\">\n<root>\n<root id=\"a\"/>\n</root>\n</root>",
		want: []selfSelectorDiagnostic{identityDiagnostic("/root/root", 4, 1, keyRequiresValue("inner"))},
	}})
	const outer = `<xs:unique name="outer"><xs:selector xpath=".//root"/><xs:field xpath="@id"/></xs:unique>`
	requireSelfSelectorCases(t, schema(outer+inner), valid, []selfSelectorCase{{
		name: "descendant and self hits in one batch",
		xml:  "<root id=\"a\">\n<root id=\"b\">\n<root id=\"b\"/>\n</root>\n<root/>\n</root>",
		want: []selfSelectorDiagnostic{
			identityDiagnostic("/root/root", 4, 1, duplicateValue("outer", "/root/root/root")),
			identityDiagnostic("/root/root", 5, 7, keyRequiresValue("inner")),
		},
	}})
}

func TestIdentitySelectorsHitSameElementReportInDeclarationOrder(t *testing.T) {
	t.Parallel()

	keys := func(content string, selectors ...string) string {
		var b strings.Builder
		b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>` + content + `<xs:attribute name="id" type="xs:string"/></xs:complexType>`)
		for i, selector := range selectors {
			fmt.Fprintf(&b, `<xs:key name="k%d"><xs:selector xpath="%s"/><xs:field xpath="@id"/></xs:key>`, i+1, selector)
		}
		b.WriteString(`</xs:element></xs:schema>`)
		return b.String()
	}
	requireSelfSelectorCases(t, keys("", ".//.", ".", ".|.//."), `<root id="a"/>`, []selfSelectorCase{{
		name: "every source selects the scope element",
		xml:  `<root/>`,
		want: []selfSelectorDiagnostic{
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k1")),
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k2")),
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k3")),
		},
	}})
	requireSelfSelectorCases(t, keys("", ".", ".//.", "."), `<root id="a"/>`, []selfSelectorCase{{
		name: "dispatched hit between two self hits",
		xml:  `<root/>`,
		want: []selfSelectorDiagnostic{
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k1")),
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k2")),
			identityDiagnostic("/root", 1, 7, keyRequiresValue("k3")),
		},
	}})
	const child = `<xs:sequence><xs:element name="a" minOccurs="0"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element></xs:sequence>`
	requireSelfSelectorCases(t, keys(child, "a|.", ".", "a"), `<root id="r"><a id="a"/></root>`, []selfSelectorCase{{
		name: "child and self branches mixed across constraints",
		xml:  `<root><a/></root>`,
		want: []selfSelectorDiagnostic{
			identityDiagnostic("/root/a", 1, 10, keyRequiresValue("k1")),
			identityDiagnostic("/root/a", 1, 10, keyRequiresValue("k3")),
			identityDiagnostic("/root", 1, 11, keyRequiresValue("k1")),
			identityDiagnostic("/root", 1, 11, keyRequiresValue("k2")),
		},
	}})
}
