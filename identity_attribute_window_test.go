package xsd_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

// Attribute fields are matched only against selections whose depth lies in a
// schema-wide window derived from every attribute field path. These tests
// cover paths whose step counts or descendant prefixes set the window edges.
// Documents put every start and end tag on its own line, so end-of-element
// diagnostics are at column 1 of the end tag's line.

// attributeWindowSchema declares root > s* > a* (a nests recursively) with b*
// under a; s, a and b each accept attributes x and y. b is mixed so its end tag
// can sit on its own line.
func attributeWindowSchema(constraints ...string) string {
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="xy">
    <xs:attribute name="x" type="xs:string"/>
    <xs:attribute name="y" type="xs:string"/>
  </xs:attributeGroup>
  <xs:element name="a">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="a" minOccurs="0" maxOccurs="unbounded"/>
        <xs:element name="b" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType mixed="true"><xs:attributeGroup ref="xy"/></xs:complexType>
        </xs:element>
      </xs:sequence>
      <xs:attributeGroup ref="xy"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="s" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence><xs:element ref="a" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>
            <xs:attributeGroup ref="xy"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    ` + strings.Join(constraints, "\n    ") + `
  </xs:element>
</xs:schema>`
}

func uniqueConstraint(name, selector, field string) string {
	return fmt.Sprintf(`<xs:unique name="%s"><xs:selector xpath="%s"/><xs:field xpath="%s"/></xs:unique>`, name, selector, field)
}

func documentLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

func TestIdentityAttributeFieldsMixedStepCounts(t *testing.T) {
	t.Parallel()

	grandchildY := uniqueConstraint("u1", ".//s", "a/b/@y")
	ownX := uniqueConstraint("u2", ".//b", "@x")
	// Lines: root 1, s 2/8, a 3/9, b 4/10; b ends at 5/11, a at 6/12, s at 7/13.
	twoGrandchildren := func(x1, y1, x2, y2 string) string {
		return documentLines(
			"<root>",
			"<s>", "<a>", fmt.Sprintf(`<b x="%s" y="%s">`, x1, y1), "</b>", "</a>", "</s>",
			"<s>", "<a>", fmt.Sprintf(`<b x="%s" y="%s">`, x2, y2), "</b>", "</a>", "</s>",
			"</root>")
	}
	valid := twoGrandchildren("1", "v", "2", "w")
	for _, order := range [][]string{{grandchildY, ownX}, {ownX, grandchildY}} {
		requireSelfSelectorCases(t, attributeWindowSchema(order...), valid, []selfSelectorCase{
			{name: "distinct values", xml: valid},
			{
				name: "grandchild attribute repeats across selections",
				xml:  twoGrandchildren("1", "v", "2", "v"),
				want: []selfSelectorDiagnostic{identityDiagnostic("/root/s", 13, 1, duplicateValue("u1", "/root/s"))},
			},
			{
				name: "own and grandchild attributes both repeat",
				xml:  twoGrandchildren("1", "v", "1", "v"),
				want: []selfSelectorDiagnostic{
					identityDiagnostic("/root/s/a/b", 11, 1, duplicateValue("u2", "/root/s/a/b")),
					identityDiagnostic("/root/s", 13, 1, duplicateValue("u1", "/root/s")),
				},
			},
		})
	}

	// The window at b spans a's depth, but a's own-attribute field must not
	// capture b's attribute.
	ownXOfA := uniqueConstraint("u3", ".//a", "@x")
	requireSelfSelectorCases(t, attributeWindowSchema(grandchildY, ownXOfA), valid, []selfSelectorCase{
		{name: "equal x on b under different a", xml: twoGrandchildren("1", "v", "1", "w")},
		{
			name: "equal x on a",
			xml: documentLines(
				"<root>",
				"<s>", `<a x="1">`, "</a>", "</s>",
				"<s>", `<a x="1">`, "</a>", "</s>",
				"</root>"),
			want: []selfSelectorDiagnostic{identityDiagnostic("/root/s/a", 8, 1, duplicateValue("u3", "/root/s/a"))},
		},
	})

	// Both branches of a one-attribute union belong to the same field path set.
	ownOrGrandchildX := uniqueConstraint("u5", ".//s", "@x|a/b/@x")
	requireSelfSelectorCases(t, attributeWindowSchema(ownOrGrandchildX), valid, []selfSelectorCase{{
		name: "grandchild branch of a one-attribute union repeats",
		xml:  twoGrandchildren("1", "v", "1", "w"),
		want: []selfSelectorDiagnostic{identityDiagnostic("/root/s", 13, 1, duplicateValue("u5", "/root/s"))},
	}})

	childX := uniqueConstraint("u4", ".//s", "a/@x")
	twoChildren := func(x1, x2 string) string {
		return documentLines(
			"<root>",
			"<s>", fmt.Sprintf(`<a x="%s">`, x1), "</a>", "</s>",
			"<s>", fmt.Sprintf(`<a x="%s">`, x2), "</a>", "</s>",
			"</root>")
	}
	requireSelfSelectorCases(t, attributeWindowSchema(childX), twoChildren("1", "2"), []selfSelectorCase{{
		name: "child attribute below the root repeats",
		xml:  twoChildren("1", "1"),
		want: []selfSelectorDiagnostic{identityDiagnostic("/root/s", 9, 1, duplicateValue("u4", "/root/s"))},
	}})
}

func TestIdentityDescendantAttributeFieldReachesDeepSelections(t *testing.T) {
	t.Parallel()

	// s1 holds x four levels down (line 6), s2 two levels down (line 14);
	// s2 ends at line 17.
	deep := func(x1, x2 string) string {
		return documentLines(
			"<root>",
			"<s>", "<a>", "<a>", "<a>", fmt.Sprintf(`<a x="%s">`, x1), "</a>", "</a>", "</a>", "</a>", "</s>",
			"<s>", "<a>", fmt.Sprintf(`<a x="%s">`, x2), "</a>", "</a>", "</s>",
			"</root>")
	}
	valid := deep("1", "2")
	repeated := []selfSelectorDiagnostic{identityDiagnostic("/root/s", 17, 1, duplicateValue("u", "/root/s"))}
	ownAndDeep := documentLines(
		"<root>",
		`<s x="1">`, "</s>",
		"<s>", "<a>", `<a x="1">`, "</a>", "</a>", "</s>",
		"</root>")

	requireSelfSelectorCases(t, attributeWindowSchema(uniqueConstraint("u", "s", ".//@x")), valid, []selfSelectorCase{
		{name: "distinct deep values", xml: valid},
		{name: "deep values at different depths repeat", xml: deep("1", "1"), want: repeated},
		{
			name: "selected element's own value repeats a deep value",
			xml:  ownAndDeep,
			want: []selfSelectorDiagnostic{identityDiagnostic("/root/s", 9, 1, duplicateValue("u", "/root/s"))},
		},
	})
	requireSelfSelectorCases(t, attributeWindowSchema(uniqueConstraint("u", "s", ".//a/@x")), valid, []selfSelectorCase{
		{name: "deep values at different depths repeat", xml: deep("1", "1"), want: repeated},
		{name: "selected element's own value is not an a attribute", xml: ownAndDeep},
	})
	requireSelfSelectorCases(t, attributeWindowSchema(uniqueConstraint("u", "s", "./a/@x")), valid, []selfSelectorCase{
		{name: "deep values below the child are not selected", xml: deep("1", "1")},
	})
}

func TestIdentityAttributeWildcardFieldWidensWindow(t *testing.T) {
	t.Parallel()

	// Lines: s 2/6, a 3/7; the second s ends at line 9.
	selections := func(sAttribute, aAttribute, a1, a2 string) string {
		return documentLines(
			"<root>",
			fmt.Sprintf(`<s %s="1">`, sAttribute), fmt.Sprintf(`<a %s="%s">`, aAttribute, a1), "</a>", "</s>",
			fmt.Sprintf(`<s %s="2">`, sAttribute), fmt.Sprintf(`<a %s="%s">`, aAttribute, a2), "</a>", "</s>",
			"</root>")
	}
	repeated := []selfSelectorDiagnostic{identityDiagnostic("/root/s", 9, 1, duplicateValue("u2", "/root/s"))}

	schema := attributeWindowSchema(uniqueConstraint("u1", ".//s", "@x"), uniqueConstraint("u2", ".//s", "a/@*"))
	requireSelfSelectorCases(t, schema, selections("x", "y", "v", "w"), []selfSelectorCase{{
		name: "child wildcard value repeats",
		xml:  selections("x", "y", "v", "v"),
		want: repeated,
	}})
	schema = attributeWindowSchema(uniqueConstraint("u1", ".//s", "@*"), uniqueConstraint("u2", ".//s", "a/@x"))
	requireSelfSelectorCases(t, schema, selections("y", "x", "v", "w"), []selfSelectorCase{{
		name: "child exact-name value repeats beside an own wildcard",
		xml:  selections("y", "x", "v", "v"),
		want: repeated,
	}})
}

func TestIdentityNestedRecursiveSameConstraintAcrossDepths(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="node">
    <xs:complexType>
      <xs:sequence><xs:element ref="node" minOccurs="0"/></xs:sequence>
      <xs:attribute name="id" type="xs:string" use="required"/>
      <xs:attribute name="alt" type="xs:string"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="node" minOccurs="0"/></xs:sequence>
    </xs:complexType>
    <xs:key name="nodeID"><xs:selector xpath=".//node"/><xs:field xpath="@id"/></xs:key>
    <xs:unique name="altChild"><xs:selector xpath=".//node"/><xs:field xpath="node/@alt"/></xs:unique>
    <xs:unique name="altDeep"><xs:selector xpath=".//node"/><xs:field xpath="node/node/@alt"/></xs:unique>
  </xs:element>
</xs:schema>`
	engine := compileIdentityDispatchTestEngine(t, schema)
	for _, levels := range []int{1, 2, 16, 256} {
		t.Run(fmt.Sprint(levels), func(t *testing.T) {
			t.Parallel()
			session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: levels + 2})
			if err != nil {
				t.Fatal(err)
			}
			clean := nestedNodeDocument(levels, levels)
			if err := session.Validate(strings.NewReader(clean)); err != nil {
				t.Fatalf("clean document: %v", err)
			}
			if levels < 4 {
				// A planted collision needs two distinct node pairs whose
				// child and grandchild alt values coincide.
				return
			}
			got, diagErr := selfSelectorDiagnostics(session.Validate(strings.NewReader(nestedNodeDocument(levels, 3))))
			if diagErr != nil {
				t.Fatal(diagErr)
			}
			if want := nestedNodeCollisionDiagnostics(levels); !slices.Equal(got, want) {
				t.Fatalf("diagnostics =\n%+v\nwant\n%+v", got, want)
			}
			if err := session.Validate(strings.NewReader(clean)); err != nil {
				t.Fatalf("clean document after failure: %v", err)
			}
		})
	}
}

// nestedNodeDocument nests levels node elements under root, one tag per line.
// Node k (1-based) has id nk and alt ak, except the deepest node, whose alt
// copies node lastAlt's; lastAlt == levels keeps every alt distinct.
func nestedNodeDocument(levels, lastAlt int) string {
	lines := []string{"<root>"}
	for level := 1; level <= levels; level++ {
		alt := lastAlt
		if level != levels {
			alt = level
		}
		lines = append(lines, fmt.Sprintf(`<node id="n%d" alt="a%d">`, level, alt))
	}
	for range levels {
		lines = append(lines, "</node>")
	}
	return documentLines(append(lines, "</root>")...)
}

// nestedNodeCollisionDiagnostics derives the diagnostics for
// nestedNodeDocument(levels, 3). The deepest alt equals node 3's, so altChild
// sees it from nodes 2 and levels-1 and altDeep from nodes 1 and levels-2.
// Deeper nodes end first, so each duplicate is reported where the shallower
// node ends: node k ends on line 2*levels+2-k.
func nestedNodeCollisionDiagnostics(levels int) []selfSelectorDiagnostic {
	nodePath := func(level int) string { return "/root" + strings.Repeat("/node", level) }
	return []selfSelectorDiagnostic{
		identityDiagnostic(nodePath(2), 2*levels, 1, duplicateValue("altChild", nodePath(levels-1))),
		identityDiagnostic(nodePath(1), 2*levels+1, 1, duplicateValue("altDeep", nodePath(levels-2))),
	}
}
