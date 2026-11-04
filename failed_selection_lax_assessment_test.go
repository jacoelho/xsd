package xsd_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// After a failed type selection or particle match, the element is assessed
// laxly (cvc-assess-elt): its global declaration governs when one exists,
// otherwise anyType. Its subtree therefore keeps reporting its own
// diagnostics, which the corpus cannot observe past the first one.
const failedSelectionSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" type="xs:int"/>
  <xs:element name="abs" type="xs:int" abstract="true"/>
  <xs:element name="ided"><xs:complexType><xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element>
  <xs:element name="refd"><xs:complexType><xs:attribute name="ref" type="xs:IDREF"/></xs:complexType></xs:element>
  <xs:element name="unavail" type="absent"/>
  <xs:element name="unavailBlocked" type="absent" block="#all"/>
  <xs:element name="unavailNil" type="absent" nillable="true"/>
  <xs:element name="unavailFixed" type="absent" fixed="v"/>
  <xs:element name="unavailUniq" type="absent">
    <xs:unique name="unavailRows"><xs:selector xpath="*"/><xs:field xpath="."/></xs:unique>
  </xs:element>
  <xs:element name="unavailParent">
    <xs:complexType><xs:sequence><xs:element name="c" type="absent"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="seqParent">
    <xs:complexType><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="uniqSeqParent">
    <xs:complexType><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
    <xs:unique name="seqChildren"><xs:selector xpath="*"/><xs:field xpath="."/></xs:unique>
  </xs:element>
  <xs:element name="attrSeqParent">
    <xs:complexType><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
    <xs:unique name="seqChildIDs"><xs:selector xpath="*"/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
  <xs:element name="strictParent">
    <xs:complexType><xs:sequence><xs:any processContents="strict"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="nilParent" nillable="true">
    <xs:complexType><xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="simpleParent" type="xs:string"/>
  <xs:element name="scParent">
    <xs:complexType><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  </xs:element>
  <xs:element name="emptyParent"><xs:complexType/></xs:element>
  <xs:element name="absParent">
    <xs:complexType><xs:sequence><xs:element ref="abs"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="declParent">
    <xs:complexType><xs:sequence><xs:element name="c" type="xs:string"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="uniq">
    <xs:complexType><xs:sequence><xs:element name="row" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="rows"><xs:selector xpath="row"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`

const failedSelectionNamespaces = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:missing" xmlns:o="urn:o"`

func compileFailedSelectionSchema(t *testing.T) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("failed-selection.xsd", []byte(failedSelectionSchema)))
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

// codePaths renders each diagnostic as code@path, omitting messages and
// positions so cascades are pinned only by what reported them.
func codePaths(t *testing.T, err error) []string {
	t.Helper()
	var out []string
	for _, leaf := range xsderrors.Flatten(err) {
		x, ok := errors.AsType[*xsderrors.Error](leaf)
		if !ok {
			t.Fatalf("diagnostic %T %v is not structured", leaf, leaf)
		}
		out = append(out, string(x.Code())+"@"+x.Path())
	}
	return out
}

func TestFailedSelectionAssessesElementLaxly(t *testing.T) {
	engine := compileFailedSelectionSchema(t)
	for _, test := range []struct {
		name, doc string
		want      []string
	}{
		{
			name: "root with unresolved xsi:type",
			doc:  `<u` + failedSelectionNamespaces + ` xsi:type="p:Missing"><bad>x</bad></u>`,
			want: []string{"validation.type@/u", "validation.facet@/u/bad"},
		},
		{
			name: "root with unresolved xsi:type and malformed nil",
			doc:  `<u` + failedSelectionNamespaces + ` xsi:type="p:Missing" xsi:nil="bad"><bad>x</bad></u>`,
			want: []string{"validation.type@/u", "validation.nil@/u", "validation.facet@/u/bad"},
		},
		{
			name: "undeclared root without xsi:type",
			doc:  `<u><bad>x</bad></u>`,
			want: []string{"validation.root@/", "validation.facet@/u/bad"},
		},
		{
			name: "root with unavailable type",
			doc:  `<unavail><bad>x</bad></unavail>`,
			want: []string{"validation.element@/unavail", "validation.facet@/unavail/bad"},
		},
		{
			// The replacement drops the declaration, so the instance's own
			// xsi:type is resolved against anyType and then governs.
			name: "unavailable type with resolving xsi:type",
			doc:  `<unavail` + failedSelectionNamespaces + ` xsi:type="xs:int">x</unavail>`,
			want: []string{"validation.element@/unavail", "validation.facet@/unavail"},
		},
		{
			name: "unavailable type with valid xsi:type value",
			doc:  `<unavail` + failedSelectionNamespaces + ` xsi:type="xs:int">1</unavail>`,
			want: []string{"validation.element@/unavail"},
		},
		{
			name: "unavailable type ignores the dropped declaration's block",
			doc:  `<unavailBlocked` + failedSelectionNamespaces + ` xsi:type="xs:int">x</unavailBlocked>`,
			want: []string{"validation.element@/unavailBlocked", "validation.facet@/unavailBlocked"},
		},
		{
			name: "unavailable type with unresolved xsi:type",
			doc:  `<unavail` + failedSelectionNamespaces + ` xsi:type="p:Missing">x</unavail>`,
			want: []string{"validation.element@/unavail", "validation.type@/unavail"},
		},
		{
			// The dropped declaration's nillable, fixed value and identity
			// constraints no longer apply; anyType governs.
			// Without a declaration xsi:nil is checked only lexically, so
			// neither a non-nillable nor a nilled-with-content error applies.
			name: "unavailable type ignores the dropped declaration's non-nillable",
			doc:  `<unavail` + failedSelectionNamespaces + ` xsi:nil="true"/>`,
			want: []string{"validation.element@/unavail"},
		},
		{
			name: "unavailable type ignores the dropped declaration's nillable",
			doc:  `<unavailNil` + failedSelectionNamespaces + ` xsi:nil="true">x</unavailNil>`,
			want: []string{"validation.element@/unavailNil"},
		},
		{
			name: "unavailable type ignores the dropped declaration's fixed value",
			doc:  `<unavailFixed>w</unavailFixed>`,
			want: []string{"validation.element@/unavailFixed"},
		},
		{
			name: "unavailable type ignores the dropped declaration's unique",
			doc:  `<unavailUniq><bad>1</bad><bad>1</bad></unavailUniq>`,
			want: []string{"validation.element@/unavailUniq"},
		},
		{
			name: "unavailable child type with resolving xsi:type",
			doc:  `<unavailParent` + failedSelectionNamespaces + `><c xsi:type="xs:int">x</c></unavailParent>`,
			want: []string{"validation.element@/unavailParent/c", "validation.facet@/unavailParent/c"},
		},
		{
			name: "abstract root keeps its type",
			doc:  `<abs>x</abs>`,
			want: []string{"validation.element@/abs", "validation.facet@/abs"},
		},
		{
			name: "abstract root with unresolved xsi:type",
			doc:  `<abs` + failedSelectionNamespaces + ` xsi:type="p:M">1</abs>`,
			want: []string{"validation.element@/abs", "validation.type@/abs"},
		},
		{
			name: "unexpected undeclared child subtree",
			doc:  `<seqParent><a>1</a><zz><bad>x</bad></zz></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.facet@/seqParent/zz/bad"},
		},
		{
			name: "unexpected child uses its global declaration",
			doc:  `<seqParent><a>1</a><bad>x</bad></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.facet@/seqParent/bad"},
		},
		{
			name: "unexpected children keep ID typing",
			doc:  `<seqParent><a>1</a><ided id="x"/><ided id="x"/></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.element@/seqParent", "validation.type@/seqParent/ided"},
		},
		{
			name: "strict wildcard without declaration",
			doc:  `<strictParent><zz><bad>x</bad></zz></strictParent>`,
			want: []string{"validation.element@/strictParent", "validation.facet@/strictParent/zz/bad"},
		},
		{
			name: "child of nilled parent",
			doc:  `<nilParent` + failedSelectionNamespaces + ` xsi:nil="true"><bad>x</bad></nilParent>`,
			want: []string{"validation.nil@/nilParent", "validation.facet@/nilParent/bad"},
		},
		{
			name: "child of simple-type parent",
			doc:  `<simpleParent><bad>x</bad></simpleParent>`,
			want: []string{"validation.content@/simpleParent", "validation.facet@/simpleParent/bad"},
		},
		{
			name: "child of simple-content parent",
			doc:  `<scParent><bad>x</bad></scParent>`,
			want: []string{"validation.content@/scParent", "validation.facet@/scParent/bad"},
		},
		{
			name: "child of empty parent",
			doc:  `<emptyParent><bad>x</bad></emptyParent>`,
			want: []string{"validation.element@/emptyParent", "validation.facet@/emptyParent/bad"},
		},
		{
			name: "abstract child keeps its type",
			doc:  `<absParent><abs>x</abs></absParent>`,
			want: []string{"validation.element@/absParent/abs", "validation.facet@/absParent/abs"},
		},
		{
			name: "declared child with unresolved xsi:type reports one type error",
			doc:  `<declParent` + failedSelectionNamespaces + `><c xsi:type="p:Missing"><bad>x</bad></c></declParent>`,
			want: []string{"validation.type@/declParent/c", "validation.content@/declParent/c", "validation.facet@/declParent/c/bad"},
		},
		{
			name: "identity scope opens in unexpected child",
			doc:  `<seqParent><a>1</a><uniq><row>1</row><row>1</row></uniq></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.identity@/seqParent/uniq/row"},
		},
		{
			// A laxly assessed anyType element selected by a "." field has
			// no simple value (cvc-identity-constraint 3).
			name: "unexpected text-only child selected by self field",
			doc:  `<uniqSeqParent><a>1</a><zz>1</zz></uniqSeqParent>`,
			want: []string{"validation.element@/uniqSeqParent", "validation.identity@/uniqSeqParent/zz"},
		},
		{
			name: "unexpected element-only child selected by self field",
			doc:  `<uniqSeqParent><a>1</a><zz><y/></zz></uniqSeqParent>`,
			want: []string{"validation.element@/uniqSeqParent", "validation.identity@/uniqSeqParent/zz"},
		},
		{
			name: "unexpected global child selected by self field keeps its value",
			doc:  `<uniqSeqParent><a>1</a><bad>1</bad></uniqSeqParent>`,
			want: []string{"validation.element@/uniqSeqParent"},
		},
		{
			name: "unexpected global children selected by self field collide",
			doc:  `<uniqSeqParent><a>1</a><bad>1</bad><bad>1</bad></uniqSeqParent>`,
			want: []string{"validation.element@/uniqSeqParent", "validation.element@/uniqSeqParent", "validation.identity@/uniqSeqParent/bad"},
		},
		{
			name: "invalid unexpected global child selected by self field contributes no value",
			doc:  `<uniqSeqParent><a>1</a><bad>x</bad></uniqSeqParent>`,
			want: []string{"validation.element@/uniqSeqParent", "validation.facet@/uniqSeqParent/bad"},
		},
		{
			// Lax assessment leaves the attribute without a declaration, so
			// the field holds an unknown value.
			name: "undeclared attribute on unexpected child",
			doc:  `<attrSeqParent><a>1</a><zz id="1"/></attrSeqParent>`,
			want: []string{"validation.element@/attrSeqParent"},
		},
		{
			name: "equal undeclared attributes on unexpected children",
			doc:  `<attrSeqParent><a>1</a><zz id="1"/><zz id="1"/></attrSeqParent>`,
			want: []string{"validation.element@/attrSeqParent", "validation.element@/attrSeqParent"},
		},
		{
			name: "ID from unexpected child resolves IDREF",
			doc:  `<seqParent><a>1</a><ided id="x"/><refd ref="x"/></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.element@/seqParent"},
		},
		{
			name: "dangling IDREF in unexpected child",
			doc:  `<seqParent><a>1</a><refd ref="nope"/></seqParent>`,
			want: []string{"validation.element@/seqParent", "validation.type@/seqParent/refd"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, maxErrors := range []int{10, 2, 1} {
				want := test.want[:min(maxErrors, len(test.want))]
				session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: maxErrors})
				if err != nil {
					t.Fatal(err)
				}
				// A second run after a valid document proves a stop inside a
				// laxly assessed subtree leaves no ID, identity or frame state.
				for range 2 {
					if got := codePaths(t, session.Validate(strings.NewReader(test.doc))); !slices.Equal(got, want) {
						t.Fatalf("MaxErrors=%d: diagnostics = %v, want %v", maxErrors, got, want)
					}
					for _, valid := range []string{
						`<seqParent><a>1</a></seqParent>`,
						`<uniq><row>1</row><row>2</row></uniq>`,
						`<ided id="x"/>`,
						`<uniqSeqParent><a>1</a></uniqSeqParent>`,
						`<attrSeqParent><a>1</a></attrSeqParent>`,
					} {
						if err := session.Validate(strings.NewReader(valid)); err != nil {
							t.Fatalf("MaxErrors=%d: valid %s after failed selection: %v", maxErrors, valid, err)
						}
					}
				}
			}
		})
	}
}

func TestFailedSelectionLaxAssessmentPreservesFatalErrors(t *testing.T) {
	engine := compileFailedSelectionSchema(t)
	for _, doc := range []string{
		`<seqParent><a>1</a><zz><bad>x</bad></zz></wrong>`,
		`<seqParent><a>1</a><zz><bad>x</bad></zz>`,
		`<seqParent><a>1</a><zz><bad>x</wrong></zz></seqParent>`,
	} {
		for _, maxErrors := range []int{0, 1, 10} {
			err := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: maxErrors})
			got := codePaths(t, err)
			if len(got) != 1 || !strings.HasPrefix(got[0], string(xsderrors.CodeValidationXML)+"@") {
				t.Fatalf("%s MaxErrors=%d: diagnostics = %v, want only %s", doc, maxErrors, got, xsderrors.CodeValidationXML)
			}
			if xsderrors.IsConclusiveValidation(err) {
				t.Fatalf("%s MaxErrors=%d: malformed XML reported as conclusive", doc, maxErrors)
			}
		}
	}
}
