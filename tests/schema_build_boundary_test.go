package tests_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerSchemaTopologyMutationsStayInOwner(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal/compile")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, file)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, typeErr := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, files, info)
	if typeErr != nil {
		t.Fatalf("type-check internal/compile: %v", typeErr)
	}
	assertSchemaBuildHasNoMutableAccessors(t, pkg)
	for _, file := range files {
		if fset.Position(file.Pos()).Filename == filepath.Join(dir, "schema_build.go") {
			continue
		}
		checkCompilerSchemaTopologyFile(t, fset, info, file)
	}
}

func assertSchemaBuildHasNoMutableAccessors(t *testing.T, compilePackage *types.Package) {
	t.Helper()
	var runtimePackage *types.Package
	for _, imported := range compilePackage.Imports() {
		if imported.Path() == "github.com/jacoelho/xsd/internal/runtime" {
			runtimePackage = imported
			break
		}
	}
	if runtimePackage == nil {
		t.Fatal("compile package does not import runtime")
	}
	object := runtimePackage.Scope().Lookup("SchemaBuild")
	named, ok := object.Type().(*types.Named)
	if !ok {
		t.Fatal("runtime.SchemaBuild is not a named type")
	}
	methods := types.NewMethodSet(types.NewPointer(named))
	for method := range methods.Methods() {
		signature, ok := method.Obj().Type().(*types.Signature)
		if !ok {
			continue
		}
		for result := range signature.Results().Variables() {
			if mutableAccessorResultType(result.Type()) {
				t.Fatalf("runtime.SchemaBuild.%s returns mutable state", method.Obj().Name())
			}
		}
	}
}

func TestSchemaBuildMutableAccessorDetectionCoversDefinedAndAliasedTypes(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "pointer.go", `package runtime
type SchemaBuild struct{}
type definedPointer *int
type aliasedPointer = *int
type definedSlice []int
type aliasedMap = map[string]int
func (*SchemaBuild) DefinedPointer() definedPointer { return nil }
func (*SchemaBuild) AliasedPointer() aliasedPointer { return nil }
func (*SchemaBuild) DefinedSlice() definedSlice { return nil }
func (*SchemaBuild) AliasedMap() aliasedMap { return nil }
func calls(build *SchemaBuild) {
	build.DefinedPointer()
	build.AliasedPointer()
	build.DefinedSlice()
	build.AliasedMap()
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/runtime", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	detected := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && schemaBuildMutableAccessorCall(info, call) {
			detected++
		}
		return true
	})
	if detected != 4 {
		t.Fatalf("detected %d mutable accessors, want 4", detected)
	}
}

func TestSchemaBuildMethodAllowlistRejectsMutators(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "methods.go", `package runtime
type SchemaBuild struct{}
func (*SchemaBuild) TypeName() int { return 0 }
func (*SchemaBuild) Clear() {}
func (*SchemaBuild) RemoveElement() bool { return false }
func calls(build *SchemaBuild) {
	build.TypeName()
	build.Clear()
	build.RemoveElement()
	(build.Clear)()
	clear := build.Clear
	remove := (*SchemaBuild).RemoveElement
	_ = clear
	_ = remove
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/runtime", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	rejected := 0
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method, schemaBuildSelection := schemaBuildMethodSelection(info, selector)
		if schemaBuildSelection && !schemaBuildReadMethods[method] {
			rejected++
		}
		return true
	})
	if rejected != 5 {
		t.Fatalf("rejected %d SchemaBuild method references, want 5", rejected)
	}
}

func TestSchemaBuildReplacementDetectionCoversCompilerField(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "replacement.go", `package compile
import "github.com/jacoelho/xsd/internal/runtime"
type compiler struct { rt runtime.SchemaBuild }
type holder struct { build runtime.SchemaBuild }
type wrapper struct { *compiler }
func replace(c *compiler) {
	c.rt = runtime.SchemaBuild{}
	w := wrapper{c}
	w.rt = runtime.SchemaBuild{}
	local := runtime.SchemaBuild{}
	local = runtime.SchemaBuild{}
	pointer := &local
	*pointer = runtime.SchemaBuild{}
	values := []runtime.SchemaBuild{{}}
	values[0] = runtime.SchemaBuild{}
	h := holder{}
	h.build = runtime.SchemaBuild{}
	_ = local
	_ = values
	_ = h
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	detected := 0
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if ok {
			for _, lhs := range assign.Lhs {
				if compilerSchemaBuildExpr(info, lhs) {
					detected++
				}
			}
		}
		return true
	})
	if detected != 2 {
		t.Fatalf("detected %d SchemaBuild replacements, want 2", detected)
	}
}

func TestSchemaBuildWholeValueAliasDetection(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "whole_alias.go", `package compile
import "github.com/jacoelho/xsd/internal/runtime"
type compiler struct { rt runtime.SchemaBuild }
type holder struct { build runtime.SchemaBuild }
type wrapper struct { *compiler }
func consume(runtime.SchemaBuild) {}
func aliases(c *compiler) runtime.SchemaBuild {
	copy := c.rt
	consume(c.rt)
	value := holder{build: c.rt}
	unkeyed := holder{c.rt}
	values := []runtime.SchemaBuild{c.rt}
	w := wrapper{c}
	promoted := w.rt
	_ = copy
	_ = value
	_ = unkeyed
	_ = values
	_ = promoted
	return c.rt
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	detected := 0
	ast.Inspect(file, func(node ast.Node) bool {
		expr, ok := node.(ast.Expr)
		if ok && compilerSchemaBuildExpr(info, expr) {
			detected++
			return false
		}
		return true
	})
	if detected != 7 {
		t.Fatalf("detected %d whole-build aliases, want 7", detected)
	}
}

func TestSchemaBuildSendDetection(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "send.go", `package compile
import "github.com/jacoelho/xsd/internal/runtime"
type compiler struct { rt runtime.SchemaBuild }
func send(c *compiler, builds chan<- runtime.SchemaBuild, elements chan<- []runtime.ElementDecl) {
	builds <- c.rt
	elements <- c.rt.Elements
	local := runtime.SchemaBuild{}
	builds <- local
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	detected := 0
	ast.Inspect(file, func(node ast.Node) bool {
		send, ok := node.(*ast.SendStmt)
		if ok && mutableSchemaBuildField(info, send.Value) {
			detected++
		}
		return true
	})
	if detected != 2 {
		t.Fatalf("detected %d schema-build sends, want 2", detected)
	}
}

func TestSchemaTopologyAliasDetectionCoversIndexedAndRangeValues(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "alias.go", `package compile
import "github.com/jacoelho/xsd/internal/runtime"
type compiler struct { rt runtime.SchemaBuild }
func aliases(c *compiler) {
	members := c.rt.Substitutions[0]
	_ = members
	for _, nested := range c.rt.Substitutions { _ = nested }
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("github.com/jacoelho/xsd/internal/compile", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	indexedAliases := 0
	rangeAliases := 0
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE && len(node.Rhs) == 1 && mutableSchemaBuildField(info, node.Rhs[0]) {
				indexedAliases++
			}
		case *ast.RangeStmt:
			if schemaBuildTopologyExpr(info, node.X) && node.Value != nil && mutationCapableType(info.TypeOf(node.Value)) {
				rangeAliases++
			}
		}
		return true
	})
	if indexedAliases != 1 || rangeAliases != 1 {
		t.Fatalf("detected indexed aliases %d and range aliases %d, want 1 and 1", indexedAliases, rangeAliases)
	}
}

var schemaBuildReadCalls = map[string]bool{
	"builtin.cap": true,
	"builtin.len": true,
	"github.com/jacoelho/xsd/internal/compile.CheckIdentityConstraintNameAvailable":   true,
	"github.com/jacoelho/xsd/internal/compile.ResolveIdentityConstraintRefer":         true,
	"github.com/jacoelho/xsd/internal/compile.ValidateContentRestriction":             true,
	"github.com/jacoelho/xsd/internal/compile.ValidateIdentityReferences":             true,
	"github.com/jacoelho/xsd/internal/runtime.RestrictionChoiceLimitUpdates":          true,
	"github.com/jacoelho/xsd/internal/compile.simpleTypeListReachability.reachesList": true,
	"github.com/jacoelho/xsd/internal/compile.simpleValueFacetCache.read":             true,
}

var schemaBuildReadMethods = map[string]bool{
	"AnyTypeID":                   true,
	"ComplexTypeCount":            true,
	"ComplexTypeDerivation":       true,
	"ContentModel":                true,
	"DerivedSimpleIdentity":       true,
	"ElementName":                 true,
	"ElementRestriction":          true,
	"ElementType":                 true,
	"ForEachSubstitutionMember":   true,
	"HasSubstitutionMembers":      true,
	"SimpleTypeCount":             true,
	"SimpleTypeDerivation":        true,
	"SimpleTypeFinal":             true,
	"SimpleTypeIdentity":          true,
	"StringEnumerationContains":   true,
	"SubstitutionMemberByName":    true,
	"SubstitutionNames":           true,
	"TypeLabel":                   true,
	"TypeName":                    true,
	"ValueConstraintComplexType": true,
	"ValueConstraintSimpleType":  true,
	"Wildcard":                    true,
}

var schemaBuildPointerReadCalls = map[string]bool{
	"github.com/jacoelho/xsd/internal/compile.CheckComplexContentMixedDerivationBase":          true,
	"github.com/jacoelho/xsd/internal/compile.CheckContentModelElementDeclarationsConsistent": true,
	"github.com/jacoelho/xsd/internal/compile.CheckContentModelsUPA":                          true,
	"github.com/jacoelho/xsd/internal/compile.CheckElementDeclarationsConsistent":             true,
	"github.com/jacoelho/xsd/internal/compile.CheckSimpleContentRestrictionTextType":          true,
	"github.com/jacoelho/xsd/internal/compile.CheckSimpleContentDerivationBase":                true,
	"github.com/jacoelho/xsd/internal/compile.CompileContentModels":                           true,
	"github.com/jacoelho/xsd/internal/compile.ExtendSequenceModel":                            true,
	"github.com/jacoelho/xsd/internal/compile.NewNameInterner":                                true,
	"github.com/jacoelho/xsd/internal/compile.ValidateComplexExtensionModelAdmission":         true,
	"github.com/jacoelho/xsd/internal/compile.ValidateContentRestriction":                     true,
	"github.com/jacoelho/xsd/internal/compile.ValidateSubstitutionMembership":                 true,
	"github.com/jacoelho/xsd/internal/compile.newContentModelCompiler":                        true,
	"github.com/jacoelho/xsd/internal/runtime.ValidateAttributeDeclName":                      true,
	"github.com/jacoelho/xsd/internal/runtime.ElementValueConstraintType":                     true,
	"github.com/jacoelho/xsd/internal/runtime.RestrictionChoiceLimitUpdates":                  true,
	"github.com/jacoelho/xsd/internal/runtime.ValidateAttributeDeclValueConstraintRuntime":    true,
	"github.com/jacoelho/xsd/internal/runtime.ValidateAttributeUseSetRecord":                  true,
	"github.com/jacoelho/xsd/internal/runtime.ValidateElementDeclValueConstraintRuntime":      true,
	"github.com/jacoelho/xsd/internal/compile.AttributeUseMerger.Add":                         true,
}

func checkCompilerSchemaTopologyFile(t *testing.T, fset *token.FileSet, info *types.Info, file *ast.File) {
	t.Helper()
	parents := astParentMap(file)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if compilerSchemaBuildExpr(info, lhs) || schemaBuildTopologyExpr(info, lhs) {
					t.Fatalf("%s mutates compiler schema topology outside schema_build.go", fset.Position(lhs.Pos()))
				}
			}
			for _, rhs := range node.Rhs {
				if mutableSchemaBuildField(info, rhs) {
					t.Fatalf("%s creates mutable compiler schema topology alias outside schema_build.go", fset.Position(rhs.Pos()))
				}
			}
		case *ast.ValueSpec:
			for _, value := range node.Values {
				if mutableSchemaBuildField(info, value) {
					t.Fatalf("%s creates mutable compiler schema topology alias outside schema_build.go", fset.Position(value.Pos()))
				}
			}
		case *ast.IncDecStmt:
			if schemaBuildTopologyExpr(info, node.X) {
				t.Fatalf("%s mutates compiler schema topology outside schema_build.go", fset.Position(node.Pos()))
			}
		case *ast.RangeStmt:
			if schemaBuildTopologyExpr(info, node.X) && node.Value != nil && mutationCapableType(info.TypeOf(node.Value)) {
				t.Fatalf("%s creates mutable compiler schema topology range alias outside schema_build.go", fset.Position(node.Value.Pos()))
			}
		case *ast.UnaryExpr:
			if node.Op == token.AND && schemaBuildAddressExpr(info, node.X) {
				call := enclosingCall(node, parents)
				id := callID(info, call)
				if call == nil || !schemaBuildPointerReadCalls[id] {
					t.Fatalf("%s takes mutable compiler schema topology address for %s outside schema_build.go", fset.Position(node.Pos()), id)
				}
			}
		case *ast.CallExpr:
			if method, ok := schemaBuildMethodCall(info, node); ok && !schemaBuildReadMethods[method] {
				t.Fatalf("%s calls non-read-only compiler schema method %s outside schema_build.go", fset.Position(node.Pos()), method)
			}
			if schemaBuildMutableAccessorCall(info, node) {
				t.Fatalf("%s calls mutable compiler schema accessor outside schema_build.go", fset.Position(node.Pos()))
			}
			name := callName(node)
			id := callID(info, node)
			if (name == "append" || name == "delete") && len(node.Args) != 0 && schemaBuildTopologyExpr(info, node.Args[0]) {
				t.Fatalf("%s mutates compiler schema topology with %s outside schema_build.go", fset.Position(node.Pos()), name)
			}
			for _, arg := range node.Args {
				if mutableSchemaBuildField(info, arg) && !schemaBuildReadCalls[id] && !schemaBuildPointerReadCalls[id] {
					t.Fatalf("%s passes mutable compiler schema topology to %s outside schema_build.go", fset.Position(arg.Pos()), id)
				}
			}
		case *ast.CompositeLit:
			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if mutableSchemaBuildField(info, kv.Value) {
						t.Fatalf("%s stores mutable compiler schema topology outside schema_build.go", fset.Position(kv.Value.Pos()))
					}
					continue
				}
				if mutableSchemaBuildField(info, elt) {
					t.Fatalf("%s stores mutable compiler schema topology outside schema_build.go", fset.Position(elt.Pos()))
				}
			}
		case *ast.ReturnStmt:
			for _, result := range node.Results {
				address, takesAddress := result.(*ast.UnaryExpr)
				if mutableSchemaBuildField(info, result) ||
					(takesAddress && address.Op == token.AND && schemaBuildAddressExpr(info, address.X)) {
					t.Fatalf("%s returns mutable compiler schema topology outside schema_build.go", fset.Position(result.Pos()))
				}
			}
		case *ast.FuncDecl:
			if returnsSchemaBuildPointer(info, node.Type) {
				t.Fatalf("%s returns mutable *runtime.SchemaBuild outside schema_build.go", fset.Position(node.Name.Pos()))
			}
		case *ast.SendStmt:
			if mutableSchemaBuildField(info, node.Value) {
				t.Fatalf("%s sends mutable compiler schema topology outside schema_build.go", fset.Position(node.Value.Pos()))
			}
		case *ast.SelectorExpr:
			if method, ok := schemaBuildMethodSelection(info, node); ok && !schemaBuildReadMethods[method] {
				t.Fatalf("%s references non-read-only compiler schema method %s outside schema_build.go", fset.Position(node.Pos()), method)
			}
		}
		return true
	})
}

func schemaBuildTopologyExpr(info *types.Info, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if ok && isSchemaBuildField(info.Selections[sel]) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func compilerSchemaBuildExpr(info *types.Info, expr ast.Expr) bool {
	if !isSchemaBuildType(info.TypeOf(expr)) {
		return false
	}
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && isCompilerSchemaBuildField(info.Selections[selector]) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func mutableSchemaBuildField(info *types.Info, expr ast.Expr) bool {
	if compilerSchemaBuildExpr(info, expr) {
		return true
	}
	if !schemaBuildTopologyExpr(info, expr) {
		return false
	}
	return mutationCapableType(info.TypeOf(expr))
}

func mutationCapableType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Underlying().(type) {
	case *types.Map, *types.Slice:
		return true
	default:
		return false
	}
}

func schemaBuildAddressExpr(info *types.Info, expr ast.Expr) bool {
	if schemaBuildTopologyExpr(info, expr) {
		return true
	}
	return isSchemaBuildType(info.TypeOf(expr))
}

func schemaBuildMutableAccessorCall(info *types.Info, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := info.Selections[selector]
	if selection == nil || !isSchemaBuildType(selection.Recv()) {
		return false
	}
	signature, ok := selection.Obj().Type().(*types.Signature)
	if !ok {
		return false
	}
	results := signature.Results()
	for variable := range results.Variables() {
		if mutableAccessorResultType(variable.Type()) {
			return true
		}
	}
	return false
}

func schemaBuildMethodCall(info *types.Info, call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return schemaBuildMethodSelection(info, selector)
}

func schemaBuildMethodSelection(info *types.Info, selector *ast.SelectorExpr) (string, bool) {
	selection := info.Selections[selector]
	if selection == nil ||
		(selection.Kind() != types.MethodVal && selection.Kind() != types.MethodExpr) ||
		!isSchemaBuildType(selection.Recv()) {
		return "", false
	}
	return selection.Obj().Name(), true
}

func mutableAccessorResultType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch types.Unalias(typ).Underlying().(type) {
	case *types.Pointer, *types.Map, *types.Slice:
		return true
	default:
		return false
	}
}

func isSchemaBuildField(selection *types.Selection) bool {
	return selection != nil && selection.Kind() == types.FieldVal &&
		isSchemaBuildType(selection.Recv())
}

func isCompilerSchemaBuildField(selection *types.Selection) bool {
	if selection == nil || selection.Kind() != types.FieldVal || !isSchemaBuildType(selection.Obj().Type()) {
		return false
	}
	field := selection.Obj()
	pkg := field.Pkg()
	if pkg == nil || pkg.Path() != "github.com/jacoelho/xsd/internal/compile" {
		return false
	}
	compiler, ok := pkg.Scope().Lookup("compiler").Type().Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for candidate := range compiler.Fields() {
		if candidate == field {
			return true
		}
	}
	return false
}

func isSchemaBuildType(typ types.Type) bool {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "github.com/jacoelho/xsd/internal/runtime" && named.Obj().Name() == "SchemaBuild"
}

func enclosingCall(node ast.Node, parents map[ast.Node]ast.Node) *ast.CallExpr {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if call, ok := parent.(*ast.CallExpr); ok {
			return call
		}
		if _, ok := parent.(*ast.FuncDecl); ok {
			return nil
		}
	}
	return nil
}

func callName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

func callID(info *types.Info, call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	var obj types.Object
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fun]
	case *ast.SelectorExpr:
		if selection := info.Selections[fun]; selection != nil {
			obj = selection.Obj()
			if named := receiverNamedType(selection.Recv()); named != nil && named.Obj().Pkg() != nil {
				return named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + obj.Name()
			}
		} else {
			obj = info.Uses[fun.Sel]
		}
	}
	if obj == nil {
		return ""
	}
	if _, ok := obj.(*types.Builtin); ok {
		return "builtin." + obj.Name()
	}
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

func receiverNamedType(typ types.Type) *types.Named {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return nil
	}
	return named
}

func returnsSchemaBuildPointer(info *types.Info, typ *ast.FuncType) bool {
	if typ.Results == nil {
		return false
	}
	for _, field := range typ.Results.List {
		ptr, ok := info.TypeOf(field.Type).(*types.Pointer)
		if ok && isSchemaBuildType(ptr.Elem()) {
			return true
		}
	}
	return false
}
