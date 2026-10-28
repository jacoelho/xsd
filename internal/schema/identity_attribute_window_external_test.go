package schema_test

import (
	"fmt"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

// The oracle is IdentityPathProgramRead.Matches over every element path of the
// given depth built from the names the field paths use plus one unused name,
// so every base depth some path can match is reached. The window must contain
// every matching base depth and end just past the highest; its start must be
// the lowest only where the contract promises it: no ".//" path and a depth of
// at least the most steps of any path.
func TestIdentityAttributeFieldBaseDepthsAreSound(t *testing.T) {
	t.Parallel()

	const maxDepth = 5
	tests := []struct {
		name string
		// fields holds one identity constraint per entry, in declaration order.
		fields []string
	}{
		{name: "no identity constraints"},
		{name: "element field only", fields: []string{"a"}},
		{name: "own attribute", fields: []string{"@x"}},
		{name: "child attribute", fields: []string{"a/@x"}},
		{name: "grandchild attribute", fields: []string{"a/b/@y"}},
		{name: "descendant-or-self attribute", fields: []string{".//@x"}},
		{name: "descendant child attribute", fields: []string{".//a/@x"}},
		{name: "child attribute wildcard", fields: []string{"a/@*"}},
		{name: "union of shallow and deep paths", fields: []string{"@x|a/b/@y"}},
		{name: "union of shallow and deep paths for one attribute", fields: []string{"@x|a/b/@x"}},
		{name: "shallow constraint first", fields: []string{"@x", "a/b/@y"}},
		{name: "deep constraint first", fields: []string{"a/b/@y", "@x"}},
		{name: "descendant path deeper than direct path", fields: []string{".//a/b/c/@x", "a/@y"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := publishedIdentityAttributeWindowSchema(t, test.fields)
			paths := publishedAttributeFieldPaths(t, rt)
			if wantPaths := strings.Contains(strings.Join(test.fields, " "), "@"); wantPaths != (len(paths) > 0) {
				t.Fatalf("published attribute field paths = %d, want some = %t", len(paths), wantPaths)
			}
			alphabet := identityAttributeWindowNames(t, rt)
			dispatch := rt.IdentityDispatch()
			descendant, mostSteps := false, 0
			for _, path := range paths {
				descendant = descendant || path.Descendant()
				mostSteps = max(mostSteps, path.StepCount())
			}
			for depth := range maxDepth + 1 {
				lowest, highest := -1, -1
				forEachIdentityNamePath(alphabet, depth, func(namePath []xsdSchema.RuntimeName) {
					for base := range depth + 2 {
						for index := range paths {
							if !paths[index].Matches(rt, namePath, base, depth) {
								continue
							}
							if lowest < 0 || base < lowest {
								lowest = base
							}
							highest = max(highest, base)
						}
					}
				})
				from, to := dispatch.AttributeFieldBaseDepths(depth)
				if from < 0 || from > to || to > depth+1 {
					t.Fatalf("depth %d: window [%d, %d) violates 0 <= from <= to <= depth+1", depth, from, to)
				}
				if lowest < 0 {
					if from != to {
						t.Fatalf("depth %d: window [%d, %d), want empty: no base depth matches", depth, from, to)
					}
					continue
				}
				if lowest < from || highest >= to {
					t.Fatalf("depth %d: window [%d, %d) misses matching base depths in [%d, %d]", depth, from, to, lowest, highest)
				}
				if to != highest+1 {
					t.Fatalf("depth %d: window [%d, %d), want end %d past the highest matching base depth", depth, from, to, highest+1)
				}
				if !descendant && depth >= mostSteps && from != lowest {
					t.Fatalf("depth %d: window [%d, %d), want start %d at the lowest matching base depth", depth, from, to, lowest)
				}
			}
		})
	}
}

func TestIdentityAttributeFieldBaseDepthsWithoutAttributeFieldsAreEmpty(t *testing.T) {
	t.Parallel()

	var dispatch xsdSchema.IdentityDispatchRead
	for _, depth := range []int{0, 1, 2, 256} {
		if from, to := dispatch.AttributeFieldBaseDepths(depth); from != to {
			t.Fatalf("zero dispatch at depth %d: window [%d, %d), want empty", depth, from, to)
		}
	}
}

// Deep windows are worked out by hand because an exhaustive Matches oracle
// cannot reach this depth.
func TestIdentityAttributeFieldBaseDepthsWhenDeep(t *testing.T) {
	t.Parallel()

	const depth = 256
	tests := []struct {
		name     string
		fields   []string
		from, to int
	}{
		{name: "own attribute", fields: []string{"@x"}, from: 256, to: 257},
		{name: "child attribute", fields: []string{"a/@x"}, from: 255, to: 256},
		{name: "own and grandchild attributes", fields: []string{"@x", "a/b/@y"}, from: 254, to: 257},
		{name: "descendant-or-self attribute", fields: []string{".//@x"}, from: 0, to: 257},
		{name: "descendant child attribute", fields: []string{".//a/@x"}, from: 0, to: 256},
		{name: "own and descendant child attributes", fields: []string{"@x", ".//a/@y"}, from: 0, to: 257},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := publishedIdentityAttributeWindowSchema(t, test.fields)
			from, to := rt.IdentityDispatch().AttributeFieldBaseDepths(depth)
			if from != test.from || to != test.to {
				t.Fatalf("window [%d, %d) at depth %d, want [%d, %d)", from, to, depth, test.from, test.to)
			}
		})
	}
}

func publishedIdentityAttributeWindowSchema(t *testing.T, fields []string) *xsdSchema.Schema {
	t.Helper()
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for _, name := range []string{"a", "b", "c", "other"} {
		fmt.Fprintf(&b, `<xs:element name="%s"/>`, name)
	}
	b.WriteString(`<xs:attribute name="x"/><xs:attribute name="y"/>`)
	b.WriteString(`<xs:element name="root"><xs:complexType><xs:sequence>`)
	b.WriteString(`<xs:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/>`)
	b.WriteString(`</xs:sequence></xs:complexType>`)
	for index, field := range fields {
		fmt.Fprintf(&b, `<xs:unique name="u%d"><xs:selector xpath=".//*"/><xs:field xpath="%s"/></xs:unique>`, index, field)
	}
	b.WriteString(`</xs:element></xs:schema>`)
	rt, err := publishSchema(compiledCompilerRuntime(t, b.String()).RuntimeForTest())
	if err != nil {
		t.Fatalf("publish schema: %v", err)
	}
	return rt
}

func publishedAttributeFieldPaths(t *testing.T, rt *xsdSchema.Schema) []xsdSchema.IdentityPathProgramRead {
	t.Helper()
	dispatch := rt.IdentityDispatch()
	attributes := []xsdSchema.QName{mustQName(t, rt, "x"), mustQName(t, rt, "y")}
	var paths []xsdSchema.IdentityPathProgramRead
	appendFields := func(fields xsdSchema.IdentityCompiledFieldProgramReads) {
		for fieldIndex := range fields.Len() {
			field, ok := fields.At(fieldIndex)
			if !ok {
				t.Fatalf("field %d is missing", fieldIndex)
			}
			for pathIndex := range field.PathCount() {
				path, ok := field.Path(pathIndex)
				if !ok {
					t.Fatalf("field %d path %d is missing", fieldIndex, pathIndex)
				}
				paths = append(paths, path)
			}
		}
	}
	for id := xsdSchema.IdentityConstraintID(0); xsdSchema.ValidIdentityConstraintID(id, dispatch.ConstraintCount()); id++ {
		program, ok := dispatch.Program(id)
		if !ok {
			t.Fatalf("identity constraint %d is missing", id)
		}
		for _, attribute := range attributes {
			appendFields(program.AttributeFields(attribute))
		}
		appendFields(program.AttributeWildcardFields())
	}
	return paths
}

func identityAttributeWindowNames(t *testing.T, rt *xsdSchema.Schema) []xsdSchema.RuntimeName {
	t.Helper()
	locals := []string{"a", "b", "c", "other"}
	names := make([]xsdSchema.RuntimeName, 0, len(locals))
	for _, local := range locals {
		names = append(names, xsdSchema.RuntimeName{Known: true, Local: local, Name: mustQName(t, rt, local)})
	}
	return names
}

// forEachIdentityNamePath calls visit with every path of exactly depth names
// drawn from alphabet. visit must not retain the slice.
func forEachIdentityNamePath(alphabet []xsdSchema.RuntimeName, depth int, visit func([]xsdSchema.RuntimeName)) {
	digits := make([]int, depth)
	path := make([]xsdSchema.RuntimeName, depth)
	for {
		for index, digit := range digits {
			path[index] = alphabet[digit]
		}
		visit(path)
		position := depth - 1
		for position >= 0 && digits[position] == len(alphabet)-1 {
			digits[position] = 0
			position--
		}
		if position < 0 {
			return
		}
		digits[position]++
	}
}
