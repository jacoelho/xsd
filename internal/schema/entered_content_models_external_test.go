package schema_test

import (
	"fmt"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

// enteredModelsSchema mixes the models publication keeps with those it drops:
// unused groups, including an indexed all-group, and a group used only nested
// in T1. Types compile in name order and the nested group right after T1, so
// T2, T3 and T4 publish below their source IDs. S1 and U1 are both a whole
// group reference and share the model compiled for S1, so the roots in type
// order are not ascending. Two indexed all-groups (T3, T4) sit beside
// sequences, anyType, a lax wildcard and an attribute-only extension.
var enteredModelsSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="orphanSeq"><xs:sequence><xs:element name="orphanChild"/></xs:sequence></xs:group>
  <xs:group name="orphanAll"><xs:all>` + allElements("orphanAll") + `</xs:all></xs:group>
  <xs:group name="nested"><xs:sequence><xs:element name="nestedChild"/></xs:sequence></xs:group>
  <xs:group name="whole"><xs:sequence><xs:element name="wholeChild"/></xs:sequence></xs:group>
  <xs:element name="head" abstract="true"/>
  <xs:element name="member" substitutionGroup="head"/>
  <xs:complexType name="S1"><xs:group ref="whole"/></xs:complexType>
  <xs:complexType name="U1"><xs:group ref="whole"/></xs:complexType>
  <xs:complexType name="T1"><xs:sequence><xs:element name="a1"/><xs:group ref="nested"/></xs:sequence></xs:complexType>
  <xs:complexType name="T2"><xs:sequence><xs:element name="a2"/></xs:sequence></xs:complexType>
  <xs:complexType name="T3"><xs:all>` + allElements("b") + `<xs:element ref="head"/></xs:all></xs:complexType>
  <xs:complexType name="T4"><xs:all>` + allElements("c") + `</xs:all></xs:complexType>
  <xs:complexType name="Base"><xs:sequence><xs:element name="baseChild"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:attribute name="extra"/>
  </xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="s1" type="S1"/>
  <xs:element name="u1" type="U1"/>
  <xs:element name="t1" type="T1"/>
  <xs:element name="t2" type="T2"/>
  <xs:element name="t3" type="T3"/>
  <xs:element name="t4" type="T4"/>
  <xs:element name="g" type="T2"/>
  <xs:element name="e" type="Base"/>
  <xs:element name="any" type="xs:anyType"/>
  <xs:element name="lax"><xs:complexType><xs:sequence>
    <xs:any processContents="lax" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

// allElements declares eight required elements, enough for an indexed
// all-group.
func allElements(prefix string) string {
	var b strings.Builder
	for i := range 8 {
		fmt.Fprintf(&b, `<xs:element name="%s%d"/>`, prefix, i)
	}
	return b.String()
}

func allChildren(prefix string, skip int) string {
	var b strings.Builder
	for i := 7; i >= 0; i-- {
		if i != skip {
			fmt.Fprintf(&b, "<%s%d/>", prefix, i)
		}
	}
	return b.String()
}

func TestPublishedSchemaValidatesEveryTypeAgainstItsOwnModel(t *testing.T) {
	t.Parallel()

	build := mutableSchemaBuild(t, enteredModelsSchema)
	roots := make(map[xsdSchema.ContentModelID]bool, len(build.ComplexTypes))
	for i := range build.ComplexTypes {
		roots[build.ComplexTypes[i].Content] = true
	}
	userRoot := func(name string) xsdSchema.ContentModelID {
		typ, ok := build.GlobalTypes[mustQName(t, &build.Names, name)].Complex()
		if !ok {
			t.Fatalf("%s is not a complex type", name)
		}
		return build.ComplexTypes[typ].Content
	}
	// The cases below catch a misdirected root only if published indexes
	// differ from source IDs, so the shifted types must sit above a dropped
	// model, and the extension must share its base's model.
	firstOrphan := xsdSchema.NoContentModel
	for id := range build.CompiledModels {
		if !roots[xsdSchema.ContentModelID(id)] {
			firstOrphan = xsdSchema.ContentModelID(id)
			break
		}
	}
	for _, name := range []string{"T2", "T3", "T4"} {
		if root := userRoot(name); firstOrphan == xsdSchema.NoContentModel || root < firstOrphan {
			t.Fatalf("fixture: %s root %d is not above a dropped model (first dropped %d)", name, root, firstOrphan)
		}
	}
	if userRoot("Base") != userRoot("Derived") {
		t.Fatalf("fixture: Derived root %d does not share Base root %d", userRoot("Derived"), userRoot("Base"))
	}
	// Publication must sort the roots: U1 shares S1's model, which sits below
	// the models of the types compiled between them.
	if userRoot("S1") != userRoot("U1") || userRoot("U1") >= userRoot("T4") {
		t.Fatalf("fixture: U1 root %d does not share S1 root %d below T4 root %d", userRoot("U1"), userRoot("S1"), userRoot("T4"))
	}
	compiled := len(build.CompiledModels)

	published, err := publishSchema(build)
	if err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
	if got := published.PublishedContentModelCountForTest(); got != len(roots) || got >= compiled {
		t.Fatalf("published %d content models, want the %d distinct roots of %d compiled", got, len(roots), compiled)
	}

	const xsi = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	for _, tt := range []struct {
		name, doc string
		// expected names the missing or unexpected particle the diagnostic
		// must mention; empty means the document is valid.
		expected string
	}{
		{name: "nested group", doc: `<t1><a1/><nestedChild/></t1>`},
		{name: "nested group child missing", doc: `<t1><a1/></t1>`, expected: "nestedChild"},
		{name: "shared whole group", doc: `<u1><wholeChild/></u1>`},
		{name: "shared whole group child missing", doc: `<u1/>`, expected: "wholeChild"},
		{name: "shared whole group first type", doc: `<s1><wholeChild/></s1>`},
		{name: "sequence", doc: `<t2><a2/></t2>`},
		{name: "sequence wrong child", doc: `<t2><orphanChild/></t2>`, expected: "a2"},
		{name: "indexed all with substitution", doc: `<t3>` + allChildren("b", -1) + `<member/></t3>`},
		{name: "indexed all abstract head", doc: `<t3>` + allChildren("b", -1) + `<head/></t3>`, expected: "head"},
		{name: "indexed all missing term", doc: `<t3>` + allChildren("b", 3) + `<member/></t3>`, expected: "b3"},
		{name: "indexed all repeated member", doc: `<t3>` + allChildren("b", -1) + `<member/><member/></t3>`, expected: "member"},
		{name: "second indexed all", doc: `<t4>` + allChildren("c", -1) + `</t4>`},
		{name: "second indexed all missing term", doc: `<t4>` + allChildren("c", 3) + `</t4>`, expected: "c3"},
		{name: "second indexed all foreign term", doc: `<t4>` + allChildren("c", -1) + `<b0/></t4>`, expected: "b0"},
		{name: "unused all names", doc: `<t4>` + allChildren("orphanAll", -1) + `</t4>`, expected: "orphanAll7"},
		{name: "shared model through xsi:type", doc: `<e ` + xsi + ` xsi:type="Derived" extra="x"><baseChild/></e>`},
		{name: "shared model child missing", doc: `<e ` + xsi + ` xsi:type="Derived"/>`, expected: "baseChild"},
		{name: "anyType reaches declared child", doc: `<any><u/><g><a2/></g><v>text</v></any>`},
		{name: "anyType declared child invalid", doc: `<any><g/></any>`, expected: "a2"},
		{name: "lax wildcard reaches declared child", doc: `<lax><g><a2/></g><u/></lax>`},
		{name: "lax wildcard declared child invalid", doc: `<lax><g/></lax>`, expected: "a2"},
	} {
		err := validateWithRuntime(published, tt.doc)
		if tt.expected == "" {
			if err != nil {
				t.Errorf("%s: Validate() error = %v", tt.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.expected) {
			t.Errorf("%s: Validate() error = %v, want a diagnostic naming %s", tt.name, err, tt.expected)
		}
	}
}

func TestPublishedSchemaKeepsUnusedAllIndexOffTheNextModel(t *testing.T) {
	t.Parallel()

	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="unused"><xs:all>`+allElements("u")+`</xs:all></xs:group>
  <xs:element name="small"><xs:complexType><xs:all>
    <xs:element name="x"/><xs:element name="y"/>
  </xs:all></xs:complexType></xs:element>
</xs:schema>`)
	// A small all-group carries no index of its own, so only an index taken
	// from the unentered large group could change how it matches; that needs
	// the large group to precede it with no entered model between them.
	roots := make(map[xsdSchema.ContentModelID]bool, len(build.ComplexTypes))
	for i := range build.ComplexTypes {
		roots[build.ComplexTypes[i].Content] = true
	}
	unused, small := -1, -1
	for id, model := range build.CompiledModels {
		if model.Kind == xsdSchema.CompiledModelAll {
			switch len(model.All) {
			case 8:
				unused = id
			case 2:
				small = id
			}
		}
	}
	if unused < 0 || small <= unused || roots[xsdSchema.ContentModelID(unused)] || !roots[xsdSchema.ContentModelID(small)] {
		t.Fatalf("fixture: unused all model %d must be unentered and precede the small all root %d", unused, small)
	}
	for id := unused + 1; id < small; id++ {
		if roots[xsdSchema.ContentModelID(id)] {
			t.Fatalf("fixture: root %d lies between unused all model %d and small all root %d", id, unused, small)
		}
	}

	published, err := publishSchema(build)
	if err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
	for _, doc := range []string{`<small><y/><x/></small>`, `<small><x/><y/></small>`} {
		if err := validateWithRuntime(published, doc); err != nil {
			t.Errorf("Validate(%s) error = %v", doc, err)
		}
	}
	if err := validateWithRuntime(published, `<small><x/></small>`); err == nil || !strings.Contains(err.Error(), `"y"`) {
		t.Errorf("Validate(<small><x/></small>) error = %v, want a diagnostic naming y", err)
	}
}

func TestCompileAppliesStateLimitToUnusedGroups(t *testing.T) {
	t.Parallel()

	const repeat = `<xs:group name="unused"><xs:sequence>
    <xs:sequence minOccurs="18" maxOccurs="18"><xs:element name="a"/><xs:element name="b"/></xs:sequence>
  </xs:sequence></xs:group>`
	schema := func(groups string) string {
		return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + groups + `
  <xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	}
	compile := func(text string) error {
		_, err := xsdSchema.Compile(xsdSchema.Options{MaxContentModelStates: 8}, []source.Source{source.Bytes("schema.xsd", []byte(text))})
		return err
	}
	if err := compile(schema("")); err != nil {
		t.Fatalf("Compile(without unused group) error = %v", err)
	}
	expectCategoryCode(t, compile(schema(repeat)), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}
