package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

// nameOrderVariant is one way to write the same schema: its declarations in
// another order, or with an unrelated document that interns some of its names
// before the schema's own declarations.
type nameOrderVariant struct {
	name    string
	sources []xsd.SchemaSource
}

func nameOrderVariants(declarations []string, earlier string) []nameOrderVariant {
	schema := func(decls []string) []byte {
		return []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + strings.Join(decls, "") + `</xs:schema>`)
	}
	reversed := make([]string, len(declarations))
	for i, decl := range declarations {
		reversed[len(declarations)-1-i] = decl
	}
	return []nameOrderVariant{
		{name: "declared", sources: []xsd.SchemaSource{xsd.Bytes("schema.xsd", schema(declarations))}},
		{name: "reversed", sources: []xsd.SchemaSource{xsd.Bytes("schema.xsd", schema(reversed))}},
		{name: "names interned earlier", sources: []xsd.SchemaSource{
			xsd.Bytes("0-earlier.xsd", []byte(earlier)),
			xsd.Bytes("schema.xsd", schema(declarations)),
		}},
	}
}

func TestExpectedChildDiagnosticListsNamesInExpandedNameOrder(t *testing.T) {
	t.Parallel()

	members := []string{"eee", "jjj", "bbb", "hhh", "aaa", "ggg", "ddd", "iii", "ccc", "fff"}
	declarations := []string{
		`<xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="head"/></xs:sequence></xs:complexType></xs:element>`,
		`<xs:element name="head" abstract="true"/>`,
	}
	for _, member := range members {
		declarations = append(declarations, `<xs:element name="`+member+`" substitutionGroup="head"/>`)
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` +
		`<xs:complexType name="jjj"/><xs:complexType name="iii"/><xs:complexType name="hhh"/></xs:schema>`
	const want = `unexpected child element "zzz"; expected next child ["aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh", ...]`

	for _, variant := range nameOrderVariants(declarations, earlier) {
		engine, err := xsd.Compile(variant.sources...)
		if err != nil {
			t.Fatalf("%s: Compile() error = %v", variant.name, err)
		}
		err = engine.Validate(strings.NewReader(`<root><zzz/></root>`))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Validate() error = %v, want it to contain %q", variant.name, err, want)
		}
	}
}

func TestExpectedChildDiagnosticListsAWildcardAlternativeWhateverTheNumberOfElementAlternatives(t *testing.T) {
	t.Parallel()

	elements := ""
	for _, name := range []string{"eee", "bbb", "hhh", "aaa", "ggg", "ddd", "iii", "ccc", "fff"} {
		elements += `<xs:element name="` + name + `"/>`
	}
	const wildcard = `<xs:any namespace="##other" processContents="lax"/>`
	const want = `unexpected child element "zzz"; expected next child [an element in a non-empty namespace other than "", "aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", ...]`
	for _, variant := range []struct{ name, choice string }{
		{name: "wildcard last", choice: elements + wildcard},
		{name: "wildcard first", choice: wildcard + elements},
	} {
		name, choice := variant.name, variant.choice
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:choice>` + choice + `</xs:choice></xs:complexType></xs:element></xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("%s: Compile() error = %v", name, err)
		}
		err = engine.Validate(strings.NewReader(`<root><zzz/></root>`))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Validate() error = %v, want it to contain %q", name, err, want)
		}
	}
}

func TestExpectedChildDiagnosticGivesWildcardsAtMostHalfTheListWhileElementNamesRemain(t *testing.T) {
	t.Parallel()

	var wildcards, elements string
	for _, n := range []string{"7", "3", "9", "1", "5", "2", "8", "4", "6"} {
		wildcards += `<xs:any namespace="urn:` + n + `" processContents="lax"/>`
		elements += `<xs:element name="e` + n + `"/>`
	}
	const want = `unexpected child element "zzz"; expected next child [` +
		`an element in namespace ["urn:1"], an element in namespace ["urn:2"], an element in namespace ["urn:3"], an element in namespace ["urn:4"], ` +
		`"e1", "e2", "e3", "e4", ...]`
	for _, variant := range []struct{ name, choice string }{
		{name: "wildcards first", choice: wildcards + elements},
		{name: "elements first", choice: elements + wildcards},
	} {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:choice>` + variant.choice + `</xs:choice></xs:complexType></xs:element></xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("%s: Compile() error = %v", variant.name, err)
		}
		err = engine.Validate(strings.NewReader(`<root><zzz/></root>`))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Validate() error = %v, want it to contain %q", variant.name, err, want)
		}
	}
}

func TestExpectedChildDiagnosticListsWildcardNamespacesInTextOrder(t *testing.T) {
	t.Parallel()

	declarations := []string{
		`<xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="urn:j urn:i urn:h urn:g urn:f urn:e urn:d urn:c urn:b urn:a"/></xs:sequence></xs:complexType></xs:element>`,
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:c"><xs:element name="c"/></xs:schema>`
	const want = `expected next child [an element in namespace ["urn:a", "urn:b", "urn:c", "urn:d", "urn:e", "urn:f", "urn:g", "urn:h", ...]]`

	for _, variant := range nameOrderVariants(declarations, earlier) {
		engine, err := xsd.Compile(variant.sources...)
		if err != nil {
			t.Fatalf("%s: Compile() error = %v", variant.name, err)
		}
		err = engine.Validate(strings.NewReader(`<root><zzz/></root>`))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Validate() error = %v, want it to contain %q", variant.name, err, want)
		}
	}
}

func TestUniqueParticleAttributionDiagnosticNamesTheFirstOverlappingExpandedName(t *testing.T) {
	t.Parallel()

	declarations := []string{
		`<xs:element name="root"><xs:complexType><xs:choice><xs:element ref="head"/><xs:any namespace="##any"/></xs:choice></xs:complexType></xs:element>`,
		`<xs:element name="head"/>`,
		`<xs:element name="ccc" substitutionGroup="head"/>`,
		`<xs:element name="bbb" substitutionGroup="head"/>`,
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="head"/><xs:complexType name="ccc"/></xs:schema>`
	const want = `UPA violation: overlapping particles in choice "bbb"`

	var first string
	for _, variant := range nameOrderVariants(declarations, earlier) {
		_, err := xsd.Compile(variant.sources...)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Compile() error = %v, want it to contain %q", variant.name, err, want)
		}
		_, message, _ := strings.Cut(err.Error(), "UPA")
		if first == "" {
			first = message
		} else if message != first {
			t.Fatalf("%s: Compile() diagnostic = %q, want %q as for the declared order", variant.name, message, first)
		}
	}
}

func TestElementDeclarationsConsistentReportsTheSameConflictWhateverTheNameOrder(t *testing.T) {
	t.Parallel()

	members := []string{"aaa", "zzz", "mmm", "ggg", "bbb", "kkk", "ddd", "hhh", "eee", "jjj", "fff"}
	declarations := []string{
		`<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="zzz" type="xs:int"/><xs:element name="aaa" type="xs:int"/><xs:element ref="head"/></xs:sequence></xs:complexType></xs:element>`,
		`<xs:element name="head" abstract="true"/>`,
	}
	for _, member := range members {
		declarations = append(declarations, `<xs:element name="`+member+`" substitutionGroup="head"/>`)
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` +
		`<xs:attribute name="zzz"/><xs:attribute name="mmm"/><xs:attribute name="ggg"/><xs:attribute name="bbb"/><xs:attribute name="kkk"/><xs:attribute name="ddd"/><xs:attribute name="hhh"/><xs:attribute name="eee"/><xs:attribute name="jjj"/><xs:attribute name="fff"/></xs:schema>`
	variants := nameOrderVariants(declarations, earlier)

	compileText := func(variant nameOrderVariant, steps int) string {
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxContentModelAnalysisSteps: steps}, variant.sources...)
		if err == nil {
			return ""
		}
		_, message, _ := strings.Cut(err.Error(), ": ")
		return message
	}
	const conflict = "element declarations with the same name must have the same type"
	for steps := 1; ; steps++ {
		want := compileText(variants[0], steps)
		for _, variant := range variants[1:] {
			if got := compileText(variant, steps); got != want {
				t.Fatalf("MaxContentModelAnalysisSteps %d, %s: Compile() diagnostic = %q, want %q as for the declared order", steps, variant.name, got, want)
			}
		}
		if strings.Contains(want, conflict) {
			return
		}
		if steps > 100_000 {
			t.Fatalf("no step limit up to %d reached the Element Declarations Consistent conflict; last diagnostic %q", steps, want)
		}
	}
}

func TestSubstitutionGroupRestrictionReportsTheSameFailureWhateverTheNameOrder(t *testing.T) {
	t.Parallel()

	members := []string{"aaa", "mmm", "ggg", "bbb", "kkk", "ddd", "hhh", "eee", "jjj", "fff"}
	var baseBranches string
	for _, member := range members[1:] {
		baseBranches += `<xs:element ref="` + member + `"/>`
	}
	declarations := []string{
		`<xs:complexType name="base"><xs:choice>` + baseBranches + `</xs:choice></xs:complexType>`,
		`<xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element ref="head"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:element name="head" abstract="true"/>`,
	}
	for _, member := range members {
		declarations = append(declarations, `<xs:element name="`+member+`" substitutionGroup="head"/>`)
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` +
		`<xs:attribute name="mmm"/><xs:attribute name="ggg"/><xs:attribute name="bbb"/><xs:attribute name="kkk"/><xs:attribute name="ddd"/><xs:attribute name="hhh"/><xs:attribute name="eee"/><xs:attribute name="jjj"/><xs:attribute name="fff"/></xs:schema>`
	variants := nameOrderVariants(declarations, earlier)

	compileText := func(variant nameOrderVariant, steps int) string {
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxContentModelAnalysisSteps: steps}, variant.sources...)
		if err == nil {
			return ""
		}
		_, message, _ := strings.Cut(err.Error(), ": ")
		return message
	}
	for steps := 1; ; steps++ {
		want := compileText(variants[0], steps)
		for _, variant := range variants[1:] {
			if got := compileText(variant, steps); got != want {
				t.Fatalf("MaxContentModelAnalysisSteps %d, %s: Compile() diagnostic = %q, want %q as for the declared order", steps, variant.name, got, want)
			}
		}
		if !strings.Contains(want, "exceeds MaxContentModelAnalysisSteps") {
			if !strings.Contains(want, "restriction") {
				t.Fatalf("MaxContentModelAnalysisSteps %d: Compile() diagnostic = %q, want a content restriction failure", steps, want)
			}
			return
		}
		if steps > 100_000 {
			t.Fatalf("no step limit up to %d reached the restriction failure; last diagnostic %q", steps, want)
		}
	}
}

func TestExpectedChildDiagnosticAtTheTraversalLimitIsTheSameInAnyDeclarationOrder(t *testing.T) {
	t.Parallel()

	declarations := []string{
		`<xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="head"/></xs:sequence></xs:complexType></xs:element>`,
		`<xs:element name="head" abstract="true"/>`,
	}
	for i := range 270 {
		declarations = append(declarations, fmt.Sprintf(`<xs:element name="n%03d" substitutionGroup="head"/>`, i))
	}
	earlier := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:attribute name="n269"/><xs:attribute name="n100"/></xs:schema>`
	const want = `unexpected child element "zzz"; expected next child ["n000", "n001", "n002", "n003", "n004", "n005", "n006", "n007", ...]`

	for _, variant := range nameOrderVariants(declarations, earlier) {
		engine, err := xsd.Compile(variant.sources...)
		if err != nil {
			t.Fatalf("%s: Compile() error = %v", variant.name, err)
		}
		err = engine.Validate(strings.NewReader(`<root><zzz/></root>`))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: Validate() error = %v, want it to contain %q", variant.name, err, want)
		}
	}
}
