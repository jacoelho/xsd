package schema

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
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

func TestSchemaProgramWorkIgnoresWhichModelsAreEntered(t *testing.T) {
	t.Parallel()

	var all strings.Builder
	for i := range 8 {
		fmt.Fprintf(&all, `<xs:element name="e%d"/>`, i)
	}
	all.WriteString(`<xs:element ref="head"/>`)
	text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" abstract="true"/>
  <xs:element name="m1" substitutionGroup="head"/>
  <xs:element name="m2" substitutionGroup="head"/>
  <xs:group name="unusedAll"><xs:all>` + all.String() + `</xs:all></xs:group>
  <xs:group name="unusedSeq"><xs:sequence><xs:element name="u"/></xs:sequence></xs:group>
  <xs:group name="nested"><xs:sequence><xs:element name="n"/></xs:sequence></xs:group>
  <xs:complexType name="A"><xs:sequence><xs:element name="a"/><xs:group ref="nested"/></xs:sequence></xs:complexType>
  <xs:complexType name="B"><xs:all>` + all.String() + `</xs:all></xs:complexType>
  <xs:complexType name="C"><xs:sequence><xs:element ref="head" maxOccurs="3"/></xs:sequence></xs:complexType>
</xs:schema>`
	spread := compiledTestBuild(t, text)
	// Every type enters one model in collapsed; spread keeps their own.
	// Neither build is audited, so newSchemaProgram is called directly.
	collapsed := compiledTestBuild(t, text)
	for i := range collapsed.ComplexTypes {
		collapsed.ComplexTypes[i].Content = collapsed.ComplexTypes[0].Content
	}
	if entered := len(enteredContentModels(spread.ComplexTypes)); entered < 4 || entered >= len(spread.CompiledModels) {
		t.Fatalf("fixture: %d entered of %d compiled models, want at least 4 and some unentered", entered, len(spread.CompiledModels))
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
