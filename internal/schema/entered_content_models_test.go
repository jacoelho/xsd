package schema

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

// compiledTestBuild returns the mutable build of text, compiled but not yet
// published.
func compiledTestBuild(t *testing.T, text string) *schemaBuild {
	t.Helper()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(text))}); err != nil {
		t.Fatal(err)
	}
	if err := c.IndexForTest(); err != nil {
		t.Fatal(err)
	}
	if err := c.CompileGlobalsForTest(); err != nil {
		t.Fatal(err)
	}
	return c.RuntimeForTest()
}

func enteredModelsSchema() string {
	var all strings.Builder
	for i := range 8 {
		fmt.Fprintf(&all, `<xs:element name="e%d"/>`, i)
	}
	all.WriteString(`<xs:element ref="head"/>`)
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" abstract="true"/>
  <xs:element name="m1" substitutionGroup="head"/>
  <xs:element name="m2" substitutionGroup="head"/>
  <xs:group name="nested"><xs:sequence><xs:element name="n"/></xs:sequence></xs:group>
  <xs:complexType name="A"><xs:sequence><xs:element name="a"/><xs:group ref="nested"/></xs:sequence></xs:complexType>
  <xs:complexType name="B"><xs:all>` + all.String() + `</xs:all></xs:complexType>
  <xs:complexType name="C"><xs:sequence><xs:element ref="head" maxOccurs="3"/></xs:sequence></xs:complexType>
</xs:schema>`
}

func enterFirstTypeContent(build *schemaBuild, redirect func(contentModelNode) bool) {
	for i := range build.ComplexTypes {
		if redirect(build.Models[build.ComplexTypes[i].Content]) {
			build.ComplexTypes[i].Content = build.ComplexTypes[0].Content
		}
	}
}

func TestSchemaProgramWorkIgnoresWhichModelsAreEntered(t *testing.T) {
	t.Parallel()

	spread := compiledTestBuild(t, enteredModelsSchema())
	collapsed := compiledTestBuild(t, enteredModelsSchema())
	enterFirstTypeContent(collapsed, func(model contentModelNode) bool { return model.Kind != modelAll })
	spreadContent, err := enterContentModels(spread.ComplexTypes, &spread.CompiledContent)
	if err != nil {
		t.Fatal(err)
	}
	if entered := len(spreadContent.Models); entered < 4 || entered >= len(spread.CompiledContent.Models) {
		t.Fatalf("fixture: %d entered of %d compiled models, want at least 4 and some unentered", entered, len(spread.CompiledContent.Models))
	}

	total := func(build *schemaBuild) int {
		units := 0
		if _, err := newSchemaProgram(build, func(steps int) error { units += steps; return nil }); err != nil {
			t.Fatalf("newSchemaProgram() error = %v", err)
		}
		return units
	}
	want := total(spread)
	if got := total(collapsed); got != want {
		t.Fatalf("work with one entered model = %d, want %d as with every root entered", got, want)
	}
	exhausted := errors.New("content work exhausted")
	for name, build := range map[string]*schemaBuild{"spread": spread, "collapsed": collapsed} {
		remaining := want - 1
		_, err := newSchemaProgram(build, func(steps int) error {
			remaining -= steps
			if remaining < 0 {
				return exhausted
			}
			return nil
		})
		if !errors.Is(err, exhausted) {
			t.Fatalf("%s: newSchemaProgram() with %d units = %v, want work exhaustion", name, want-1, err)
		}
	}
}

func TestEnterContentModelsMapsEveryTypeToItsPublishedModel(t *testing.T) {
	t.Parallel()

	kinds := func(kinds ...compiledModelKind) *compiledContent {
		content := &compiledContent{Sources: make([]contentModelID, len(kinds)), Models: make([]compiledModelRead, len(kinds))}
		for i, kind := range kinds {
			content.Sources[i] = contentModelID(i)
			content.Models[i].Kind = kind
		}
		return content
	}
	types := func(contents ...contentModelID) []complexTypeMetadata {
		types := make([]complexTypeMetadata, len(contents))
		for i, content := range contents {
			types[i].Content = content
		}
		return types
	}
	for _, test := range []struct {
		name          string
		models        *compiledContent
		types         []complexTypeMetadata
		wantModels    []int
		wantPublished []contentModelID
	}{
		{
			// Empty roots fold to the lowest empty ID; unentered models drop.
			name:          "empty roots share the lowest empty model",
			models:        kinds(compiledModelEmpty, compiledModelAll, compiledModelEmpty, compiledModelDFA, compiledModelEmpty),
			types:         types(2, 1, 0, 3, 1, 4),
			wantModels:    []int{0, 1, 3},
			wantPublished: []contentModelID{0, 1, 0, 2, 1, 0},
		},
		{
			name:          "no empty models",
			models:        kinds(compiledModelDFA, compiledModelDFA, compiledModelAny),
			types:         types(2, 2, 0),
			wantModels:    []int{0, 2},
			wantPublished: []contentModelID{1, 1, 0},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := enterContentModels(test.types, test.models)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Models, test.wantModels) || !slices.Equal(got.Published, test.wantPublished) {
				t.Fatalf("enterContentModels() = %v %v, want %v %v", got.Models, got.Published, test.wantModels, test.wantPublished)
			}
		})
	}
}

func TestPublicationRejectsCompiledAllModelNoComplexTypeEnters(t *testing.T) {
	t.Parallel()

	build := compiledTestBuild(t, enteredModelsSchema())
	if _, err := enterContentModels(build.ComplexTypes, &build.CompiledContent); err != nil {
		t.Fatalf("enterContentModels() with every all model entered = %v", err)
	}
	enterFirstTypeContent(build, func(model contentModelNode) bool { return model.Kind == modelAll })
	_, err := publishSchema(build, unlimitedContentModelWork)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if !strings.Contains(err.Error(), "all content model") {
		t.Fatalf("publishSchema() error = %v, want the unentered all model reported", err)
	}
}
