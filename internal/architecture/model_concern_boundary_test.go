package architecture_test

import (
	"go/ast"
	"go/parser"
	"slices"
	"testing"
)

func TestModelBuiltinsAdaptersOnlyUsedByModelOrBuiltinsPackage(t *testing.T) {
	t.Parallel()

	const modelImport = "github.com/jacoelho/xsd/internal/model"
	allowedScopes := []string{
		"internal/model",
		"internal/builtins",
	}
	adapterNames := []string{
		"GetBuiltin",
		"GetBuiltinNS",
		"NewBuiltinSimpleType",
	}

	assertModelSelectorBoundary(t, modelImport, allowedScopes, adapterNames, "builtins")
}

func TestModelTypedValueAdaptersOnlyUsedByModelPackage(t *testing.T) {
	t.Parallel()

	const modelImport = "github.com/jacoelho/xsd/internal/model"
	allowedScopes := []string{
		"internal/model",
	}
	adapterNames := []string{
		"NormalizeValue",
		"ParseValueForType",
		"NewDecimalValue",
		"NewXSDDurationValue",
		"NewIntegerValue",
		"NewBooleanValue",
		"NewHexBinaryValue",
		"NewBase64BinaryValue",
		"NewDateTimeValue",
		"NewFloatValue",
		"NewDoubleValue",
		"NewStringValue",
		"NewLongValue",
		"NewIntValue",
		"NewShortValue",
		"NewByteValue",
		"NewUnsignedLongValue",
		"NewUnsignedIntValue",
		"NewUnsignedShortValue",
		"NewUnsignedByteValue",
		"ParseDecimal",
		"ParseInteger",
		"ParseBoolean",
		"ParseFloat",
		"ParseDouble",
		"ParseDateTime",
		"ParseLong",
		"ParseInt",
		"ParseShort",
		"ParseByte",
		"ParseUnsignedLong",
		"ParseUnsignedInt",
		"ParseUnsignedShort",
		"ParseUnsignedByte",
		"ParseHexBinary",
		"ParseBase64Binary",
		"ParseAnyURI",
		"ParseString",
	}

	assertModelSelectorBoundary(t, modelImport, allowedScopes, adapterNames, "typedvalue")
}

func TestModelFacetAdaptersOnlyUsedByModelPackage(t *testing.T) {
	t.Parallel()

	const modelImport = "github.com/jacoelho/xsd/internal/model"
	allowedScopes := []string{
		"internal/model",
	}
	adapterNames := []string{
		"ApplyFacets",
		"ValidateValueAgainstFacets",
		"IsLengthFacet",
		"IsQNameOrNotationType",
		"ValuesEqual",
		"TypedValueForFacet",
		"ValidateFacetApplicability",
		"NewMinInclusive",
		"NewMaxInclusive",
		"NewMinExclusive",
		"NewMaxExclusive",
		"FormatEnumerationValues",
		"ParseDurationToTimeDuration",
	}

	assertModelSelectorBoundary(t, modelImport, allowedScopes, adapterNames, "facetvalue")
}

func assertModelSelectorBoundary(t *testing.T, importPath string, allowedScopes, adapterNames []string, concern string) {
	t.Helper()

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		allowedScope := withinAnyScope(file.relPath, allowedScopes)
		aliases := importAliasesForPath(parsed, importPath, "model")
		if len(aliases) == 0 {
			return
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !aliases[pkgIdent.Name] {
				return true
			}
			if !slices.Contains(adapterNames, selector.Sel.Name) {
				return true
			}
			if allowedScope {
				return true
			}
			t.Fatalf("model %s boundary violation: %s uses %s.%s", concern, file.relPath, pkgIdent.Name, selector.Sel.Name)
			return false
		})
	})
}
