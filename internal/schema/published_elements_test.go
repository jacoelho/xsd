package schema

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// publishedElementsSchema has unreached declarations in a named group no type
// references, which compiles before every global element, under a zero-count
// element and under a zero-count compositor.
const publishedElementsSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  %s
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
  <xs:element name="other" type="xs:string" substitutionGroup="member"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    %s
    <xs:element name="pair"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
    <xs:element ref="head" maxOccurs="unbounded"/>
    <xs:element name="counted" minOccurs="2" maxOccurs="20"/>
    <xs:element name="seq"><xs:complexType><xs:sequence>
      <xs:element name="s1" minOccurs="0"/><xs:element name="s2" minOccurs="0"/><xs:element name="s3" minOccurs="0"/>
    </xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const (
	unreachedGroup = `<xs:group name="dead"><xs:sequence>
    <xs:element name="g0" type="xs:int" default="1"/>
    <xs:element name="head" nillable="true"/>
  </xs:sequence></xs:group>`
	unreachedLocals = `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
      <xs:element name="g1"><xs:complexType><xs:all><xs:element name="g2"/></xs:all></xs:complexType></xs:element>
    </xs:sequence></xs:complexType></xs:element>
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="g3"/></xs:sequence>`
)

func TestPublicationOmitsUnreachedElementDeclarations(t *testing.T) {
	t.Parallel()

	published := func(t *testing.T, group, locals string) []ExpandedName {
		t.Helper()
		schema, err := publishSchema(compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, group, locals)), unlimitedContentModelWork)
		if err != nil {
			t.Fatalf("publishSchema() error = %v", err)
		}
		names := make([]ExpandedName, len(schema.program.Elements.meta))
		for i := range names {
			name, ok := schema.DiagnosticElementName(ElementID(i))
			if !ok {
				t.Fatalf("DiagnosticElementName(%d) is not published", i)
			}
			names[i] = name
		}
		return names
	}
	want := published(t, "", "")
	if got := published(t, unreachedGroup, unreachedLocals); !slices.Equal(got, want) {
		t.Fatalf("published element declarations = %v, want %v as without unreached declarations", got, want)
	}
}

func TestPublishedElementIDsNameTheDeclarationsTheBuildNamed(t *testing.T) {
	t.Parallel()

	build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
	names := newNameReadView(&build.Names)
	buildName := func(id ElementID) ExpandedName {
		q := build.Elements[id].Name
		return ExpandedName{Namespace: names.Namespace(q.Namespace), Local: build.Names.locals[q.Local]}
	}
	if head := build.GlobalElements[mustBuildQName(t, build, "head")]; head == 0 || build.ReachableElements[0] {
		t.Fatalf("fixture: global head has build ID %d after a reached declaration, want unreached declarations first", head)
	}
	var wantContent []ExpandedName
	counted := false
	for _, root := range enteredContentForTest(t, build).Models {
		wantContent = appendContentElementNames(wantContent, &build.CompiledContent.Models[root], buildName)
		for _, row := range build.CompiledContent.Models[root].Rows {
			counted = counted || row.Counted
		}
	}
	if !counted {
		t.Fatal("fixture publishes no counted row")
	}
	wantGlobals := make(map[QName]ExpandedName, len(build.GlobalElements))
	wantMembers := make(map[QName][]ExpandedName, len(build.GlobalElements))
	for q, id := range build.GlobalElements {
		wantGlobals[q] = buildName(id)
		build.Substitutions.ForEachMember(id, func(member ElementID) bool {
			wantMembers[q] = append(wantMembers[q], buildName(member))
			return true
		})
	}

	schema, err := publishSchema(build, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	publishedName := func(id ElementID) ExpandedName {
		name, ok := schema.DiagnosticElementName(id)
		if !ok {
			t.Fatalf("published holder names element %d, outside the %d published declarations", id, len(schema.program.Elements.meta))
		}
		return name
	}
	var gotContent []ExpandedName
	for i := range schema.program.CompiledModels {
		gotContent = appendContentElementNames(gotContent, &schema.program.CompiledModels[i], publishedName)
	}
	if !slices.Equal(gotContent, wantContent) {
		t.Errorf("published content particles name %v, want %v", gotContent, wantContent)
	}
	for q, id := range schema.program.GlobalElements {
		if got := publishedName(id); got != wantGlobals[q] {
			t.Errorf("published global %v names %v", wantGlobals[q], got)
		}
		var members []ExpandedName
		schema.program.Substitutions.ForEachMember(id, func(member ElementID) bool {
			members = append(members, publishedName(member))
			return true
		})
		if !slices.Equal(members, wantMembers[q]) {
			t.Errorf("published substitution members of %v = %v, want %v", wantGlobals[q], members, wantMembers[q])
		}
	}
}

// newSchemaProgram is followed by steps that can still reject the build, so
// it must leave the element IDs of the tables it shares with the build alone.
func TestBuildingSchemaProgramLeavesSharedElementIDsUnchanged(t *testing.T) {
	t.Parallel()

	schema := fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals)
	build, want := compiledTestBuild(t, schema), compiledTestBuild(t, schema)
	if _, _, err := newSchemaProgram(build, unlimitedContentModelWork); err != nil {
		t.Fatalf("newSchemaProgram() error = %v", err)
	}
	if !reflect.DeepEqual(build.CompiledContent, want.CompiledContent) || !reflect.DeepEqual(build.Substitutions, want.Substitutions) {
		t.Fatal("newSchemaProgram() changed the content tables or substitution table it shares with the build")
	}
}

func appendContentElementNames(names []ExpandedName, model *compiledModelRead, name func(ElementID) ExpandedName) []ExpandedName {
	add := func(p compiledParticleRead) {
		if p.Kind == particleElement {
			names = append(names, name(p.Element))
		}
	}
	for _, row := range model.Rows {
		for _, edge := range row.Edges {
			add(edge.Particle)
		}
		if row.Counted {
			add(row.CountParticle)
		}
	}
	for _, term := range model.All {
		add(term.Particle)
	}
	return names
}

func TestPublicationRejectsElementHoldersWithoutPublishedIDs(t *testing.T) {
	t.Parallel()

	unreached := func(t *testing.T, build *schemaBuild) ElementID {
		t.Helper()
		id := slices.Index(build.ReachableElements, false)
		if id < 0 {
			t.Fatal("fixture has no unreached element declaration")
		}
		return ElementID(id) // #nosec G115 -- id indexes an element-declaration table.
	}
	for _, test := range []struct {
		name    string
		corrupt func(*testing.T, *schemaBuild)
		message string
	}{
		{name: "global element unreached", message: "global element declaration has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.ReachableElements[build.GlobalElements[mustBuildQName(t, build, "other")]] = false
		}},
		{name: "substitution member unreached", message: "substitution member has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.Substitutions.entries[0].member = unreached(t, build)
		}},
		{name: "substitution head unreached", message: "substitution head has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			head := build.GlobalElements[mustBuildQName(t, build, "head")]
			build.Substitutions.spans[unreached(t, build)] = build.Substitutions.spans[head]
		}},
		{name: "substitution table shorter than the declarations", message: "substitution table does not cover the element declarations", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.Substitutions.spans = build.Substitutions.spans[:len(build.Substitutions.spans)-1]
		}},
		{name: "content particle unreached", message: "published content model names an unreachable element declaration", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			for _, root := range enteredContentForTest(t, build).Models {
				if all := build.CompiledContent.Models[root].All; len(all) != 0 {
					all[0].Particle.Element = unreached(t, build)
					return
				}
			}
			t.Fatal("fixture publishes no xs:all model")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
			test.corrupt(t, build)
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("publishSchema() error = %v, want %q", err, test.message)
			}
		})
	}
}

func mustBuildQName(t *testing.T, build *schemaBuild, local string) QName {
	t.Helper()
	q, ok := build.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("name %q not interned", local)
	}
	return q
}
