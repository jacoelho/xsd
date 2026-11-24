package schema

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

const reachabilityFixture = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence>
    <xs:element name="list"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
      <xs:key name="shared"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key>
    </xs:element>
  </xs:sequence></xs:group>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
      <xs:element name="mid"><xs:complexType><xs:group ref="G"/><xs:attribute name="a"/></xs:complexType>
        <xs:key name="hidden"><xs:selector xpath="list"/><xs:field xpath="."/></xs:key>
      </xs:element>
    </xs:sequence></xs:complexType></xs:element>
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="absent"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element></xs:sequence>
    <xs:element name="live"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element>
    <xs:element name="seq"><xs:complexType><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b" minOccurs="0"/><xs:element name="c" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

func TestDetachingUnreachableComponentsSpendsNoBudget(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(reachabilityFixture))}); err != nil {
		t.Fatal(err)
	}
	if err := c.IndexForTest(); err != nil {
		t.Fatal(err)
	}
	if err := c.CompileGlobalsForTest(); err != nil {
		t.Fatal(err)
	}
	dependency, content := c.dependencyWork.remaining, c.contentWork.remaining
	if _, err := c.detachUnreachableComponents(); err != nil {
		t.Fatalf("detachUnreachableComponents() error = %v", err)
	}
	if c.dependencyWork.remaining != dependency || c.contentWork.remaining != content {
		t.Fatalf("detachUnreachableComponents() spent %d dependency and %d content-model steps, want none",
			dependency-c.dependencyWork.remaining, content-c.contentWork.remaining)
	}
}

func TestPublicationRejectsReachabilityCorruption(t *testing.T) {
	t.Parallel()

	identity := func(t *testing.T, build *schemaBuild, detached bool) *identityConstraintSpec {
		t.Helper()
		for i := range build.Identities {
			if build.Identities[i].Detached == detached && build.Identities[i].Declarer != NoElement {
				return &build.Identities[i]
			}
		}
		t.Fatalf("fixture has no identity constraint with a declarer and Detached %v", detached)
		return nil
	}
	run := func(t *testing.T, build *schemaBuild) (compiledModelRead, []compiledModelEdgeRead) {
		t.Helper()
		content := enteredContentForTest(t, build)
		for _, root := range content.Models {
			if build.CompiledContent.Runs[root] != nil {
				return build.CompiledContent.Models[root], build.CompiledContent.Runs[root]
			}
		}
		t.Fatal("fixture publishes no content model with an edge run")
		return compiledModelRead{}, nil
	}
	for _, test := range []struct {
		name    string
		corrupt func(*testing.T, *schemaBuild)
		message string
	}{
		{name: "constraint of an unreached declarer not detached", message: "published element reads disagree with identity constraint detachment", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			identity(t, build, true).Detached = false
		}},
		{name: "constraint of a reached declarer detached", message: "published element reads disagree with identity constraint detachment", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			identity(t, build, false).Detached = true
		}},
		{name: "constraint attached to a declaration other than its declarer", message: "identity constraint is attached to an element declaration other than its declarer", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			constraint := identity(t, build, false)
			original := constraint.Declarer
			for i := range build.Elements {
				if id := ElementID(i); id != original {
					constraint.Declarer = id
					return
				}
			}
		}},
		{name: "constraint detached from its reached declarer", message: "identity constraint is not attached to its declaring element declaration", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			constraint := identity(t, build, false)
			build.Elements[constraint.Declarer].Identity = nil
		}},
		{name: "row edges outside the edge run", message: "published content-model row edges are not a window of the model's edge run", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			model, _ := run(t, build)
			for i := range model.Rows {
				if len(model.Rows[i].Edges) != 0 {
					model.Rows[i].Edges = slices.Clone(model.Rows[i].Edges)
					return
				}
			}
			t.Fatal("run model has no row with edges")
		}},
		{name: "content model naming an invalid element", message: "published content model references an invalid element declaration", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			_, edges := run(t, build)
			edges[0].Particle.Element = noPublishedElement - 1
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, reachabilityFixture)
			test.corrupt(t, build)
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("publishSchema() error = %v, want %q", err, test.message)
			}
		})
	}
}

func enteredContentForTest(t *testing.T, build *schemaBuild) enteredContent {
	t.Helper()
	content, err := enterContentModels(build.ComplexTypes, &build.CompiledContent)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
