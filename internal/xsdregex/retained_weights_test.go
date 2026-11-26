package xsdregex

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// TestRetainedByteWeightsCoverStructSizes keeps each weight a sound upper
// bound of the memory unit it estimates.
func TestRetainedByteWeightsCoverStructSizes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		weight uintptr
		size   uintptr
	}{
		{"instruction", instructionBytes, unsafe.Sizeof(instruction{})},
		{"linearAtom", linearAtomBytes, unsafe.Sizeof(linearAtom{})},
		{"runeRange", runeRangeBytes, unsafe.Sizeof(runeRange{})},
		{"int", intBytes, unsafe.Sizeof(int(0))},
		{"word", wordBytes, unsafe.Sizeof(uint64(0))},
		{"pattern", patternBytes, unsafe.Sizeof(Pattern{}) + unsafe.Sizeof(linearPattern{})},
	} {
		if test.weight < test.size {
			t.Errorf("%s weight %d < Sizeof %d", test.name, test.weight, test.size)
		}
	}
}

func TestResourceLimitsDocStatesRetainedWeightsAndBounds(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../docs/RESOURCE_LIMITS.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(data), "\n", " ")
	f := fixedCompileLimits
	for _, want := range []string{
		fmt.Sprintf("source %d MiB", f.patternBytes>>20),
		fmt.Sprintf("%s AST nodes", commaInt(f.nodes)),
		fmt.Sprintf("%s NFA states", commaInt(f.states)),
		fmt.Sprintf("%s retained character ranges", commaInt(f.retainedRanges)),
		fmt.Sprintf("nesting depth %s", commaInt(f.depth)),
		fmt.Sprintf("%d bytes of fixed overhead", patternBytes),
		fmt.Sprintf("%d bytes per character range", runeRangeBytes),
		fmt.Sprintf("%d bytes per atom", linearAtomBytes),
		fmt.Sprintf("%d bytes per state", instructionBytes),
		fmt.Sprintf("%d bytes per scalar successor", intBytes),
		fmt.Sprintf("%d bytes per closure bitset word", wordBytes),
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/RESOURCE_LIMITS.md does not state %q", want)
		}
	}
}

func commaInt(n uint64) string {
	s := strconv.FormatUint(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// retainedSliceFields names every slice field of Pattern and of each struct
// type reachable from its fields, with the RetainedBytes term that charges it.
var retainedSliceFields = map[string]string{
	"Pattern.states":             "instructionBytes",
	"Pattern.scalarNext":         "intBytes",
	"Pattern.closureBits":        "wordBytes",
	"linearPattern.atoms":        "linearAtomBytes",
	"linearPattern.literalBytes": "bytes",
	"rangeSet.ranges":            "runeRangeBytes through Pattern.retainedRanges",
}

func TestRetainedSliceFieldsAreAllAccountedFor(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := goparser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = file
	}
	structs := map[string]*ast.StructType{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
			}
		}
	}
	reachable := map[string]bool{}
	var walk func(name string)
	walk = func(name string) {
		st, ok := structs[name]
		if !ok || reachable[name] {
			return
		}
		reachable[name] = true
		for _, field := range st.Fields.List {
			ast.Inspect(field.Type, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					walk(id.Name)
				}
				return true
			})
		}
	}
	walk("Pattern")
	found := map[string]bool{}
	for name := range reachable {
		for _, field := range structs[name].Fields.List {
			if !containsSlice(field.Type) {
				continue
			}
			for _, id := range field.Names {
				key := name + "." + id.Name
				found[key] = true
				if _, ok := retainedSliceFields[key]; !ok {
					t.Errorf("slice field %s is not accounted for in RetainedBytes; charge it and list it in retainedSliceFields", key)
				}
			}
		}
	}
	for key := range retainedSliceFields {
		if !found[key] {
			t.Errorf("retainedSliceFields lists %s, which is no longer a slice field of a type reachable from Pattern", key)
		}
	}
}

func containsSlice(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.ArrayType:
		return e.Len == nil
	case *ast.StarExpr:
		return containsSlice(e.X)
	}
	return false
}
