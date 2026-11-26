package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestIdentityTextBytesEqualsCanonicalTextLength(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	builtin := func(name string) value.TypeID {
		id, ok := builtinTypeID(name)
		if !ok {
			t.Fatalf("builtin %s missing", name)
		}
		return id
	}
	complete := func(spec value.TypeSpec) value.TypeID {
		return completeCanonicalProjectionType(t, builder, spec)
	}
	listOf := func(item string) value.TypeID {
		return complete(value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: builtin(item)})
	}
	type shape struct {
		name     string
		id       value.TypeID
		lexicals []string
	}
	samples := map[string][]string{
		"string":           {"abc", " a  b "},
		"normalizedString": {"a\tb"},
		"token":            {"  a   b  "},
		"boolean":          {"1", "true", "0", "false"},
		"decimal":          {"01.50", "-0", "+7", "100", "0.000"},
		"integer":          {"007", "-0", "+12"},
		"int":              {"7", "-2147483648"},
		"long":             {"0010"},
		"byte":             {"-128"},
		"float":            {"1", "1.5E2", "-0", "INF", "NaN", "0.001"},
		"double":           {"1", "1.5E2", "-0", "-INF", "NaN", "123456789"},
		"duration":         {"P1D", "PT0S", "P1Y2M3DT4H5M6.70S", "-P1M"},
		"dateTime":         {"2020-01-01T00:00:00+01:00", "2020-01-01T00:00:00", "2020-01-01T24:00:00Z", "2020-01-01T00:00:00.500Z"},
		"time":             {"13:20:00-05:00", "24:00:00"},
		"date":             {"2020-01-01", "2020-01-01+14:00"},
		"gYearMonth":       {"2020-02", "2020-02Z"},
		"gYear":            {"2020", "2020+01:00", "-0001"},
		"gMonthDay":        {"--02-29"},
		"gDay":             {"---15", "---15Z"},
		"gMonth":           {"--12"},
		"hexBinary":        {"0fb7", "0FB7", ""},
		"base64Binary":     {"AQID", "AQ ID", ""},
		"anyURI":           {"urn:a", ""},
		"QName":            {"p:x", "x"},
		"NOTATION":         {"p:x"},
		"ID":               {"item"},
		"IDREF":            {"ref"},
		"NCName":           {"abc"},
	}
	var shapes []shape
	for name, lexicals := range samples {
		shapes = append(shapes, shape{name, builtin(name), lexicals})
		var listLexicals []string
		listLexicals = append(listLexicals, "")
		if len(lexicals) > 0 && lexicals[0] != "" {
			listLexicals = append(listLexicals, lexicals[0], lexicals[0]+"  "+lexicals[len(lexicals)-1], "\t"+lexicals[0]+" ")
		}
		shapes = append(shapes, shape{"list(" + name + ")", listOf(name), listLexicals})
	}
	shapes = append(shapes, shape{
		name: "union(list(int),string)",
		id: complete(value.TypeSpec{
			Variety: value.Union, Base: value.NoType, ListItem: value.NoType,
			Union: []value.TypeID{listOf("int"), builtin("string")},
		}),
		lexicals: []string{"1 2 3", "x", ""},
	})
	program, err := sealAll(builder)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range shapes {
		for _, lexical := range s.lexicals {
			t.Run(s.name+"/"+lexical, func(t *testing.T) {
				t.Parallel()
				identity, err := program.Validate(s.id, lexical, lawResolver(), value.NeedIdentity, lawLimits(), nil)
				if err != nil {
					t.Skipf("lexical not in the value space: %v", err)
				}
				canonical, err := program.Validate(s.id, lexical, lawResolver(), value.NeedCanonical, lawLimits(), nil)
				if err != nil {
					t.Fatal(err)
				}
				both, err := program.Validate(s.id, lexical, lawResolver(), value.NeedIdentity|value.NeedCanonical, lawLimits(), nil)
				if err != nil {
					t.Fatal(err)
				}
				want := len(canonical.CanonicalText())
				if got := identity.IdentityTextBytes(); got != want {
					t.Errorf("IdentityTextBytes() with identity alone = %d, want %d (canonical %q)", got, want, canonical.CanonicalText())
				}
				if got := both.IdentityTextBytes(); got != want {
					t.Errorf("IdentityTextBytes() with canonical = %d, want %d", got, want)
				}
				without, err := program.Validate(s.id, lexical, lawResolver(), value.NeedCanonical, lawLimits(), nil)
				if err != nil {
					t.Fatal(err)
				}
				if without.HasIdentity() || without.IdentityTextBytes() != 0 {
					t.Errorf("without identity: HasIdentity %t, IdentityTextBytes %d, want false and 0", without.HasIdentity(), without.IdentityTextBytes())
				}
			})
		}
	}
}
