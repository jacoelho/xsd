package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// exportAllowlists is the complete exported top-level surface of each internal
// package whose exports are consumed across package boundaries. A new export
// must be a deliberate edit here, with a consumer outside its package.
var exportAllowlists = map[string][]string{
	"schema": {
		"AttributeDeclRead",
		"AttributeID",
		"AttributeUseRead",
		"AttributeUseSetRead",
		"AttributeUseSlots",
		"Compile",
		"CompileMappedSources",
		"ComplexRef",
		"ComplexTypeID",
		"ContentCompletionComplete",
		"ContentCompletionInvalid",
		"ContentCompletionStatus",
		"ContentExpectation",
		"ContentExpectations",
		"ContentInput",
		"ContentMatchAssessUndeclared",
		"ContentMatchDeclared",
		"ContentMatchInvalid",
		"ContentMatchKind",
		"ContentMatchSkip",
		"ContentMatchStrictMissing",
		"ContentScratch",
		"ContentState",
		"ContentTransition",
		"ContentTransitionInvalid",
		"ContentTransitionNoMatch",
		"ContentTransitionStatus",
		"DerivationExtension",
		"DerivationMask",
		"DerivationRestriction",
		"ElementFrameRead",
		"ElementID",
		"ElementIdentityConstraintIDs",
		"ElementStartInfo",
		"ElementTextContent",
		"ElementValueConstraints",
		"ExpandedName",
		"ExpectedNextChild",
		"IdentityCompiledFieldProgramRead",
		"IdentityCompiledFieldProgramReads",
		"IdentityConstraintID",
		"IdentityConstraintIDs",
		"IdentityConstraintProgramRead",
		"IdentityDispatchRead",
		"IdentityFieldDispatchRead",
		"IdentityFieldDispatchReads",
		"IdentityKey",
		"IdentityKeyRef",
		"IdentityKind",
		"IdentityNamespaceResolver",
		"IdentityPathProgramRead",
		"IdentityPathProgramReads",
		"IdentitySelectorDispatchRead",
		"IdentitySelectorDispatchReads",
		"IdentityStep",
		"IdentityUnique",
		"Limits",
		"MissingRequiredContent",
		"NewContentScratch",
		"NoElement",
		"NoIdentityConstraint",
		"NoSimpleType",
		"NoWildcard",
		"NormalizeOptions",
		"Options",
		"ProcessContents",
		"ProcessLax",
		"ProcessSkip",
		"QName",
		"RuntimeName",
		"Schema",
		"SimpleRef",
		"SimpleTypeID",
		"TypeDerivationScratch",
		"TypeID",
		"TypeInfo",
		"ValueConstraintRead",
		"WildcardAny",
		"WildcardID",
		"WildcardList",
		"WildcardLocal",
		"WildcardOther",
		"WildcardTargetNamespace",
		"WildcardView",
	},
	"value": {
		"AnySimpleTypeID",
		"Atomic",
		"BoundFacet",
		"Builder",
		"BuilderOptions",
		"BuiltinInteger",
		"BuiltinKind",
		"BuiltinNone",
		"BuiltinType",
		"BuiltinTypeCount",
		"BuiltinTypeID",
		"BuiltinTypeName",
		"CardinalityFacet",
		"CompareDecimalValues",
		"ConstructorUses",
		"DateTimeValue",
		"DateValue",
		"DecimalValue",
		"ErrFacet",
		"ErrLimit",
		"ErrMetadata",
		"ErrPatternLimit",
		"ErrPatternTotalLimit",
		"EvaluationLimits",
		"ExpandedName",
		"FacetAllowed",
		"FacetEnumeration",
		"FacetFractionDigits",
		"FacetLength",
		"FacetMask",
		"FacetMaxExclusive",
		"FacetMaxInclusive",
		"FacetMaxLength",
		"FacetMinExclusive",
		"FacetMinInclusive",
		"FacetMinLength",
		"FacetPattern",
		"FacetSpec",
		"FacetTotalDigits",
		"FacetWhiteSpace",
		"FloatValue",
		"InputRequirements",
		"List",
		"ListIdentityKey",
		"LiteralSpec",
		"NeedCanonical",
		"NeedIdentity",
		"Needs",
		"NewBuilder",
		"NewUntypedValue",
		"NoType",
		"ParseBooleanValue",
		"ParseDateTimeValue",
		"ParseDateValue",
		"ParseDecimalCanonical",
		"ParseDecimalValue",
		"ParseFloatValue",
		"ParseTimeValue",
		"Pattern",
		"PatternAccount",
		"PrimitiveAnyURI",
		"PrimitiveBase64Binary",
		"PrimitiveBoolean",
		"PrimitiveDateTime",
		"PrimitiveDecimal",
		"PrimitiveDouble",
		"PrimitiveDuration",
		"PrimitiveFloat",
		"PrimitiveHexBinary",
		"PrimitiveIdentityKey",
		"PrimitiveKind",
		"PrimitiveNotation",
		"PrimitiveQName",
		"PrimitiveString",
		"Program",
		"QNameResolver",
		"Resolver",
		"Scratch",
		"TemporalValue",
		"TimeValue",
		"TypeID",
		"TypeSpec",
		"TypeView",
		"Union",
		"ValidateFloatLexical",
		"Value",
		"Variety",
		"WhitespaceCollapse",
		"WhitespaceMode",
		"WhitespacePreserve",
		"WhitespaceReplace",
	},
	"validate": {
		"CheckXMLWellFormed",
		"Limits",
		"NewSessionPool",
		"NormalizeOptions",
		"Options",
		"Session",
		"SessionPool",
	},
}

func TestInternalExportSurfaceMatchesAllowlist(t *testing.T) {
	t.Parallel()
	for pkg, want := range exportAllowlists {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			got := exportedTopLevel(t, filepath.Join("..", "internal", pkg))
			want = slices.Sorted(slices.Values(want))
			if !slices.Equal(got, want) {
				t.Fatalf("internal/%s exported surface changed.\nunexpected: %v\nmissing: %v\nExport only identifiers consumed outside the package, then update exportAllowlists.",
					pkg, difference(got, want), difference(want, got))
			}
		})
	}
}

func exportedTopLevel(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names = append(names, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							names = append(names, s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								names = append(names, n.Name)
							}
						}
					}
				}
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func difference(a, b []string) []string {
	var out []string
	for _, n := range a {
		if !slices.Contains(b, n) {
			out = append(out, n)
		}
	}
	return out
}
