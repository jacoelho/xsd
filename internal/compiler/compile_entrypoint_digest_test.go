package compiler

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func TestCompileEntrypointDigestSimpleTypes(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, simpleTypeEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSimpleFacetInheritance(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, simpleFacetInheritanceEntrypointDigestFixture())
}

func TestCompileEntrypointDigestComplexDerivation(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, complexDerivationEntrypointDigestFixture())
}

func TestCompileEntrypointDigestComplexFinalDerivation(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, complexFinalDerivationEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSimpleContentNestedRestriction(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, simpleContentNestedRestrictionEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSimpleContentBaseTypeDerivation(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, simpleContentBaseTypeDerivationEntrypointDigestFixture())
}

func TestCompileEntrypointDigestComplexBlockDefault(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, complexBlockDefaultEntrypointDigestFixture())
}

func TestCompileEntrypointDigestAnyTypeExtension(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, anyTypeExtensionEntrypointDigestFixture())
}

func TestCompileEntrypointDigestAttrAndModelGroups(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, attrAndModelGroupEntrypointDigestFixture())
}

func TestCompileEntrypointDigestUnusedAttributeGroupLocalAttribute(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, unusedAttributeGroupLocalAttributeEntrypointDigestFixture())
}

func TestCompileEntrypointDigestGroupElementRefs(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, groupElementRefsEntrypointDigestFixture())
}

func TestCompileEntrypointDigestAttributeRestriction(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, attributeRestrictionEntrypointDigestFixture())
}

func TestCompileEntrypointDigestAttributeWildcardIntersection(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, attributeWildcardIntersectionEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSubstitutionIdentity(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, substitutionIdentityEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSubstitutionFinal(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, substitutionFinalEntrypointDigestFixture())
}

func TestCompileEntrypointDigestSubstitutionInheritedType(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, substitutionInheritedTypeEntrypointDigestFixture())
}

func TestCompileEntrypointDigestCyclicSubstitutionGroup(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, cyclicSubstitutionGroupEntrypointDigestFixture())
}

func TestCompileEntrypointDigestIdentityKeyrefs(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, identityKeyrefsEntrypointDigestFixture())
}

func TestCompileEntrypointDigestImportsIncludes(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, importIncludeEntrypointDigestFixture())
}

func TestCompileEntrypointDigestAdditionalAddB049(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, additionalAddB049EntrypointDigestFixture())
}

func TestCompileEntrypointDigestAdditionalAddB054(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, additionalAddB054EntrypointDigestFixture())
}

func TestCompileEntrypointDigestW3CAdditionalAddB150(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/additional/test93568.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CAdditionalAddB183(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/additional/test113911.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestAdditionalIsDefault072(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, additionalIsDefault072EntrypointDigestFixture())
}

func TestCompileEntrypointDigestChameleonForwardRefs(t *testing.T) {
	assertCompileEntrypointFixtureDigest(t, chameleonForwardRefsEntrypointDigestFixture())
}

func TestCompileEntrypointDigestW3CAnnotations(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	roots := []string{
		"xsdtests/msData/annotations/annotZ002.xsd",
		"xsdtests/msData/annotations/annotZ004.xsd",
		"xsdtests/msData/attribute/attLa005.xsd",
		"xsdtests/msData/attribute/attZ007.xsd",
	}
	for _, root := range roots {
		t.Run(filepath.Base(root), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(base, root)); err != nil {
				t.Skipf("W3C test fixture unavailable: %v", err)
			}
			assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
		})
	}
}

func TestCompileEntrypointDigestW3CComplexTypeCtE018(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctE018_a.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtF006(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctF006.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtF007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctF007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtH001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctH001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtJ002(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctJ002.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtO004(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctO004.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtO007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctO007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CComplexTypeCtZ012b(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/complexType/ctZ012b.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CStringLength006(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/string_length006.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CDecimalTotalDigits003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/decimal_totalDigits003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CDecimalFractionDigits008(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/decimal_fractionDigits008.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CTimeMaxInclusive007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/time_maxInclusive007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationLength003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/NOTATION_length003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationNotatE003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/notations/notatE003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationNotatF025(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/notations/notatF025.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationNotatF057(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/notations/notatF057.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationNotatG001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/notations/notatG001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CNotationNotatG002(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/notations/notatG002.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CLanguageEnumeration001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/language_enumeration001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIDREFSEnumeration001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/Schemas/IDREFS_enumeration001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CPositiveIntegerMaxExclusive001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/datatypes/Facets/positiveInteger/positiveInteger_maxExclusive001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CElementElemK007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/element/elemK007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CElementElemP007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/element/elemP007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CErrataErrC008(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/errata10/errC008.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CGroupA001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/group/groupA001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CGroupD001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/group/groupD001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CGroupH014(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/group/groupH014.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CGroupH021(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/group/groupH021.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CModelGroupsMgA013(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/modelGroups/mgA013.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CModelGroupsMgH014(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/modelGroups/mgH014.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CModelGroupsMgQ001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/modelGroups/mgQ001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesEb040(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesEb040.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa002(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa002.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa022(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa022.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa070(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa070.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa071(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa071.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa080(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa080.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa122(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa122.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa144(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa144.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa147(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa147.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa161(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa161.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa167(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa167.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa180(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa180.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHa181(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHa181.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHb001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHb001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHb003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHb003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHb007(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHb007.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesHb009(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesHb009.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesIa006(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesIa006.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesIb001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesIb001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesIj008(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesIj008.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesJq010(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesJq010.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesL001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesL001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesL003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesL003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesS002(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesS002.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesS008(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesS008.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesT002(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesT002.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesV003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesV003.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ005(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ005.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ012(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ012.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ013(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ013.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ023(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ023.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ028(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ028.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ030a(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ030_a.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ031(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ031.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ039(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ039.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CSchemaSchC2(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/schema/schC2_a.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CSchemaSchZ012b(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/schema/schZ012_b.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CParticlesZ001(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/particles/particlesZ001.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestSunXSD005(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/sunData/combined/xsd005/xsd005.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CAdditionalAddB091(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/additional/addB091.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CAdditionalAddB093(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/additional/addB093.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CElementElemZ027A(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/element/elemZ027a.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CElementElemZ027E(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/element/elemZ027e.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIdentityIdA035(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idA035.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIdentityIdA039(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idA039.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIdentityIdH013(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idH013.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIdentityIdZ003(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idZ003.xml"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointDigestW3CIdentityIdZ010(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idZ010.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	assertCompileEntrypointFSDigest(t, os.DirFS(base), root)
}

func TestCompileEntrypointW3CIdentityIdZ010RuntimeNamePlan(t *testing.T) {
	base := filepath.Join("..", "..", "testdata")
	root := "xsdtests/msData/identityConstraint/idZ010.xsd"
	if _, err := os.Stat(filepath.Join(base, root)); err != nil {
		t.Skipf("W3C test fixture unavailable: %v", err)
	}
	loader := newLoader(Root{FS: os.DirFS(base), Location: root}, LoadConfig{})
	loadDocs, err := loader.LoadDocuments(root)
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	ir, err := schemair.Resolve(loadDocs, schemair.ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	name := schemair.Name{Namespace: "bar", Local: "A"}
	if !hasRuntimeNameSymbol(ir.RuntimeNames, name) {
		t.Fatalf("runtime name plan missing symbol %#v", name)
	}
}

func TestCompileEntrypointDigestFixtures(t *testing.T) {
	fixtures := []entrypointDigestFixture{
		simpleTypeEntrypointDigestFixture(),
		simpleFacetInheritanceEntrypointDigestFixture(),
		complexDerivationEntrypointDigestFixture(),
		complexFinalDerivationEntrypointDigestFixture(),
		simpleContentNestedRestrictionEntrypointDigestFixture(),
		simpleContentBaseTypeDerivationEntrypointDigestFixture(),
		complexBlockDefaultEntrypointDigestFixture(),
		anyTypeExtensionEntrypointDigestFixture(),
		attrAndModelGroupEntrypointDigestFixture(),
		unusedAttributeGroupLocalAttributeEntrypointDigestFixture(),
		groupElementRefsEntrypointDigestFixture(),
		attributeRestrictionEntrypointDigestFixture(),
		attributeWildcardIntersectionEntrypointDigestFixture(),
		substitutionIdentityEntrypointDigestFixture(),
		substitutionFinalEntrypointDigestFixture(),
		substitutionInheritedTypeEntrypointDigestFixture(),
		cyclicSubstitutionGroupEntrypointDigestFixture(),
		identityKeyrefsEntrypointDigestFixture(),
		importIncludeEntrypointDigestFixture(),
		additionalAddB049EntrypointDigestFixture(),
		additionalAddB054EntrypointDigestFixture(),
		chameleonForwardRefsEntrypointDigestFixture(),
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			assertCompileEntrypointFixtureDigest(t, fixture)
		})
	}
}

type entrypointDigestFixture struct {
	name string
	fs   fstest.MapFS
	root string
}

func simpleTypeEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "simple restriction list union facets",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:maxLength value="10"/>
      <xs:enumeration value="A"/>
      <xs:enumeration value="B"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Codes">
    <xs:list itemType="tns:Code"/>
  </xs:simpleType>
  <xs:simpleType name="AnyCode">
    <xs:union memberTypes="tns:Code xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="Tokens">
    <xs:restriction base="xs:NMTOKENS">
      <xs:whiteSpace value="collapse"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:AnyCode"/>
</xs:schema>`),
	}
}

func simpleFacetInheritanceEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "simple facet inheritance",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="ST_final" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="ST_final">
  <xsd:element name="test" type="Test"/>
  <xsd:simpleType name="Test1">
    <xsd:restriction base="xsd:string">
      <xsd:pattern value="1*"/>
      <xsd:length value="2"/>
    </xsd:restriction>
  </xsd:simpleType>
  <xsd:simpleType name="Test">
    <xsd:restriction base="Test1">
      <xsd:length value="2"/>
    </xsd:restriction>
  </xsd:simpleType>
</xsd:schema>`),
	}
}

func complexDerivationEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "complex extension attr defaults",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="code" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="baseAttr" type="xs:string" default="base"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="name" type="xs:string"/>
        </xs:sequence>
        <xs:attribute name="id" type="xs:ID" use="required"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`),
	}
}

func complexFinalDerivationEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "complex final derivation",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="final" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="final">
  <xsd:element name="b" type="xsd:string"/>
  <xsd:complexType name="A" final="extension">
    <xsd:sequence>
      <xsd:element name="c" type="xsd:string"/>
    </xsd:sequence>
  </xsd:complexType>
  <xsd:complexType name="B">
    <xsd:complexContent>
      <xsd:extension base="A">
        <xsd:sequence>
          <xsd:element name="d" type="xsd:string"/>
        </xsd:sequence>
      </xsd:extension>
    </xsd:complexContent>
  </xsd:complexType>
</xsd:schema>`),
	}
}

func simpleContentNestedRestrictionEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "simple content nested restriction",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema targetNamespace="http://foo.com" xmlns="http://foo.com" xmlns:xsd="http://www.w3.org/2001/XMLSchema" elementFormDefault="unqualified">
  <xsd:element name="root">
    <xsd:complexType>
      <xsd:sequence>
        <xsd:element name="child" minOccurs="3" maxOccurs="7">
          <xsd:complexType>
            <xsd:simpleContent>
              <xsd:extension base="mytype">
                <xsd:attribute name="attr" use="optional">
                  <xsd:simpleType>
                    <xsd:restriction>
                      <xsd:simpleType>
                        <xsd:restriction base="xsd:string">
                          <xsd:minLength value="3"/>
                        </xsd:restriction>
                      </xsd:simpleType>
                      <xsd:maxLength value="10"/>
                      <xsd:minLength value="5"/>
                    </xsd:restriction>
                  </xsd:simpleType>
                </xsd:attribute>
              </xsd:extension>
            </xsd:simpleContent>
          </xsd:complexType>
        </xsd:element>
      </xsd:sequence>
    </xsd:complexType>
  </xsd:element>
  <xsd:simpleType name="mytype">
    <xsd:restriction base="xsd:string">
      <xsd:minLength value="3"/>
      <xsd:maxLength value="10"/>
    </xsd:restriction>
  </xsd:simpleType>
</xsd:schema>`),
	}
}

func simpleContentBaseTypeDerivationEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "simple content base type derivation",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="baseTD" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="baseTD">
  <xsd:element name="root" type="Test"/>
  <xsd:complexType name="Test">
    <xsd:simpleContent>
      <xsd:restriction base="Test2"/>
    </xsd:simpleContent>
  </xsd:complexType>
  <xsd:complexType name="Test2">
    <xsd:simpleContent>
      <xsd:extension base="xsd:int"/>
    </xsd:simpleContent>
  </xsd:complexType>
</xsd:schema>`),
	}
}

func complexBlockDefaultEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "complex block default",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="foo" elementFormDefault="qualified" targetNamespace="foo" blockDefault="extension">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:element name="foo" type="empty"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Dr">
    <xs:complexContent>
      <xs:restriction base="B">
        <xs:sequence>
          <xs:element name="foo" type="empty"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="De">
    <xs:complexContent>
      <xs:extension base="B"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="empty"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence maxOccurs="unbounded">
        <xs:element name="item" type="B"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
	}
}

func anyTypeExtensionEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "complex anyType extension inherits wildcard content",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="varies">
    <xs:complexContent mixed="false">
      <xs:extension base="xs:anyType"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:varies"/>
</xs:schema>`),
	}
}

func attrAndModelGroupEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "attribute group model group wildcard",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:attributeGroup name="attrs">
    <xs:attribute name="code" type="xs:string" fixed="X"/>
    <xs:anyAttribute namespace="##other" processContents="lax"/>
  </xs:attributeGroup>
  <xs:group name="children">
    <xs:choice>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:int"/>
      <xs:any namespace="##other" processContents="skip"/>
    </xs:choice>
  </xs:group>
  <xs:complexType name="Root">
    <xs:sequence>
      <xs:group ref="tns:children" maxOccurs="unbounded"/>
    </xs:sequence>
    <xs:attributeGroup ref="tns:attrs"/>
  </xs:complexType>
  <xs:element name="root" type="tns:Root"/>
</xs:schema>`),
	}
}

func unusedAttributeGroupLocalAttributeEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "unused attribute group local attribute",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="foo" targetNamespace="foo">
  <xs:attributeGroup name="foo">
    <xs:attribute name="a" type="s"/>
  </xs:attributeGroup>
  <xs:simpleType name="s">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`),
	}
}

func groupElementRefsEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "group element refs",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="ElemDecl/typeDef" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="ElemDecl/typeDef">
  <xsd:element name="root">
    <xsd:complexType>
      <xsd:sequence>
        <xsd:group ref="Group"/>
      </xsd:sequence>
    </xsd:complexType>
  </xsd:element>
  <xsd:element name="Global" type="xsd:boolean"/>
  <xsd:group name="Group">
    <xsd:sequence>
      <xsd:element ref="Global"/>
      <xsd:element name="Local" type="xsd:decimal"/>
    </xsd:sequence>
  </xsd:group>
</xsd:schema>`),
	}
}

func attributeRestrictionEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "attribute restriction override prohibit",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:foo" xmlns:foo="urn:foo" elementFormDefault="qualified" targetNamespace="urn:foo">
  <xs:complexType name="base">
    <xs:attribute name="a" type="xs:string"/>
    <xs:attribute name="b" type="xs:string"/>
    <xs:attribute name="c" type="xs:string"/>
  </xs:complexType>
  <xs:element name="base" type="base"/>
  <xs:element name="default">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="base"/>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
  <xs:element name="override">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="base">
          <xs:attribute name="a">
            <xs:simpleType>
              <xs:restriction base="xs:string">
                <xs:enumeration value="fixed"/>
              </xs:restriction>
            </xs:simpleType>
          </xs:attribute>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
  <xs:element name="add">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="base">
          <xs:attribute name="d" type="xs:string"/>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
  <xs:element name="prohibit">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="base">
          <xs:attribute name="c" use="prohibited"/>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
	}
}

func attributeWildcardIntersectionEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "attribute wildcard intersection",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:foo" xmlns:a="urn:a" xmlns:b="urn:b" xmlns:c="urn:c" elementFormDefault="qualified" targetNamespace="urn:foo">
  <xs:element name="emptywc">
    <xs:complexType>
      <xs:attributeGroup ref="skip_A"/>
      <xs:attributeGroup ref="lax_B"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="justA">
    <xs:complexType>
      <xs:attributeGroup ref="skip_A"/>
      <xs:anyAttribute processContents="skip" namespace="urn:a urn:b urn:c"/>
    </xs:complexType>
  </xs:element>
  <xs:attributeGroup name="skip_A">
    <xs:anyAttribute processContents="skip" namespace="urn:a"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="lax_B">
    <xs:anyAttribute processContents="lax" namespace="urn:b"/>
  </xs:attributeGroup>
</xs:schema>`),
	}
}

func substitutionIdentityEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "substitution identity",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="member" type="xs:string" substitutionGroup="tns:head"/>
  <xs:complexType name="Root">
    <xs:sequence>
      <xs:element ref="tns:head"/>
      <xs:element name="item" maxOccurs="unbounded">
        <xs:complexType>
          <xs:attribute name="id" type="xs:ID" use="required"/>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:Root">
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
	}
}

func substitutionFinalEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "substitution final",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="urn:test" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" elementFormDefault="qualified">
  <xsd:element name="root">
    <xsd:complexType>
      <xsd:sequence>
        <xsd:element ref="Head" maxOccurs="unbounded"/>
      </xsd:sequence>
    </xsd:complexType>
  </xsd:element>
  <xsd:element name="Head" type="HeadType" final="extension"/>
  <xsd:complexType name="HeadType">
    <xsd:sequence>
      <xsd:element name="Ear"/>
      <xsd:element name="Eye"/>
    </xsd:sequence>
  </xsd:complexType>
  <xsd:element name="Member1" type="HeadType" substitutionGroup="Head"/>
  <xsd:element name="Member3" substitutionGroup="Head">
    <xsd:complexType>
      <xsd:complexContent>
        <xsd:extension base="HeadType">
          <xsd:sequence>
            <xsd:element name="Nose"/>
          </xsd:sequence>
        </xsd:extension>
      </xsd:complexContent>
    </xsd:complexType>
  </xsd:element>
</xsd:schema>`),
	}
}

func substitutionInheritedTypeEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "substitution inherited type",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema xmlns="ElemDecl/typeDef" xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="ElemDecl/typeDef">
  <xsd:element name="Head" type="xsd:boolean"/>
  <xsd:element name="root" substitutionGroup="Head"/>
</xsd:schema>`),
	}
}

func cyclicSubstitutionGroupEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "cyclic substitution group",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:element name="foo" substitutionGroup="tns:bar" type="xs:string"/>
  <xs:element name="bar" substitutionGroup="tns:foo" type="xs:string"/>
</xs:schema>`),
	}
}

func identityKeyrefsEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "identity keyrefs",
		root: "root.xsd",
		fs: schemaFS(`
<schema xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.vehicle.org" xmlns:v="http://www.vehicle.org" elementFormDefault="qualified">
  <element name="vehicle">
    <complexType>
      <attribute name="plateNumber" type="integer"/>
      <attribute name="state" type="string"/>
    </complexType>
  </element>
  <element name="state">
    <complexType>
      <sequence>
        <element name="code" type="string" maxOccurs="unbounded"/>
        <element ref="v:vehicle" maxOccurs="unbounded" minOccurs="0"/>
        <element ref="v:person" maxOccurs="unbounded" minOccurs="0"/>
      </sequence>
    </complexType>
    <key name="reg">
      <selector xpath=".//v:vehicle"/>
      <field xpath="@plateNumber"/>
    </key>
  </element>
  <element name="root">
    <complexType>
      <sequence>
        <element ref="v:state" maxOccurs="unbounded"/>
      </sequence>
    </complexType>
    <key name="state">
      <selector xpath=".//v:state"/>
      <field xpath="v:code"/>
    </key>
    <keyref name="vehicleState" refer="v:state">
      <selector xpath=".//v:vehicle"/>
      <field xpath="@state"/>
    </keyref>
    <key name="regKey">
      <selector xpath=".//v:vehicle"/>
      <field xpath="@state"/>
      <field xpath="@plateNumber"/>
    </key>
    <keyref name="carRef" refer="v:regKey">
      <selector xpath=".//v:car"/>
      <field xpath="@regState"/>
      <field xpath="@regPlate"/>
    </keyref>
  </element>
  <element name="person">
    <complexType>
      <sequence>
        <element name="car" maxOccurs="unbounded">
          <complexType>
            <attribute name="regState" type="string"/>
            <attribute name="regPlate" type="integer"/>
          </complexType>
        </element>
      </sequence>
    </complexType>
  </element>
</schema>`),
	}
}

func importIncludeEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "include import chameleon",
		root: "root.xsd",
		fs: fstest.MapFS{
			"root.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:root" xmlns:dep="urn:dep" targetNamespace="urn:root" elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:included"/>
        <xs:element ref="dep:external"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)},
			"common.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:element name="included" type="xs:string"/>
</xs:schema>`)},
			"dep.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:dep" elementFormDefault="qualified">
  <xs:element name="external" type="xs:string"/>
</xs:schema>`)},
		},
	}
}

func additionalAddB049EntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "w3c addB049 nested keyref",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="foo" xmlns:r="foo" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="A" type="r:A" maxOccurs="10">
          <xs:keyref name="dummy" refer="r:pNumKey">
            <xs:selector xpath="r:part"/>
            <xs:field xpath="@ref-number"/>
          </xs:keyref>
        </xs:element>
        <xs:element name="B" type="r:B" maxOccurs="10"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="pNumKey">
      <xs:selector xpath="r:B/r:part"/>
      <xs:field xpath="@key-number"/>
    </xs:key>
  </xs:element>
  <xs:complexType name="A">
    <xs:sequence>
      <xs:element name="part" maxOccurs="unbounded" minOccurs="0">
        <xs:complexType>
          <xs:simpleContent>
            <xs:extension base="xs:string">
              <xs:attribute name="ref-number" type="xs:integer"/>
            </xs:extension>
          </xs:simpleContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:sequence>
      <xs:element name="part" maxOccurs="unbounded">
        <xs:complexType>
          <xs:simpleContent>
            <xs:extension base="xs:string">
              <xs:attribute name="key-number" type="xs:integer"/>
            </xs:extension>
          </xs:simpleContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`),
	}
}

func additionalAddB054EntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "w3c addB054 simpleContent nested simpleType restriction",
		root: "root.xsd",
		fs: schemaFS(`
<xs:schema targetNamespace="http://xsdtesting" xmlns="http://xsdtesting" xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="confuse">
    <xs:simpleContent>
      <xs:extension base="xs:decimal" />
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="myType">
    <xs:simpleContent>
      <xs:restriction base="confuse">
        <xs:simpleType>
          <xs:restriction base="xs:integer" />
        </xs:simpleType>
        <xs:maxInclusive value="16" />
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="myType"/>
</xs:schema>`),
	}
}

func additionalIsDefault072EntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "w3c isDefault072 default QName attribute context",
		root: "root.xsd",
		fs: schemaFS(`
<xsd:schema elementFormDefault="qualified" attributeFormDefault="qualified" xmlns:tns="http://schemas.microsoft.com/2003/10/Serialization/" targetNamespace="http://schemas.microsoft.com/2003/10/Serialization/" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <xsd:complexType name="Array">
    <xsd:sequence minOccurs="0">
      <xsd:element name="Item" type="xsd:anyType" minOccurs="0" maxOccurs="unbounded" nillable="true" form="unqualified"/>
    </xsd:sequence>
    <xsd:attribute name="ItemType" type="xsd:QName" default="xsd:anyType" />
    <xsd:attribute name="Dimensions" default="1" form="unqualified">
      <xsd:simpleType>
        <xsd:list itemType="xsd:int" />
      </xsd:simpleType>
    </xsd:attribute>
    <xsd:attribute default="0" name="LowerBounds" form="unqualified">
      <xsd:simpleType>
        <xsd:list itemType="xsd:int" />
      </xsd:simpleType>
    </xsd:attribute>
  </xsd:complexType>
  <xsd:element name="Array" type="tns:Array"/>
</xsd:schema>`),
	}
}

func chameleonForwardRefsEntrypointDigestFixture() entrypointDigestFixture {
	return entrypointDigestFixture{
		name: "chameleon forward refs",
		root: "root.xsd",
		fs: fstest.MapFS{
			"root.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:include schemaLocation="included.xsd"/>
</xs:schema>`)},
			"included.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:element name="root" type="complexType"/>
  <xs:complexType name="complexType">
    <xs:group ref="group"/>
    <xs:attributeGroup ref="attGroup"/>
  </xs:complexType>
  <xs:attributeGroup name="attGroup">
    <xs:attribute ref="att"/>
  </xs:attributeGroup>
  <xs:attribute name="att" type="simpleType"/>
  <xs:simpleType name="simpleType">
    <xs:restriction base="xs:string">
      <xs:enumeration value="yes"/>
      <xs:enumeration value="no"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:group name="group">
    <xs:sequence>
      <xs:element ref="root" minOccurs="0"/>
    </xs:sequence>
  </xs:group>
</xs:schema>`)},
		},
	}
}

func assertCompileEntrypointFixtureDigest(t *testing.T, fixture entrypointDigestFixture) {
	t.Helper()
	assertCompileEntrypointFSDigest(t, fixture.fs, fixture.root)
}

func assertCompileEntrypointFSDigest(t *testing.T, fsys fs.FS, root string) {
	t.Helper()
	roots, rootsErr := buildPrepareRootsRuntime(fsys, root)
	loadDocs, loadDocsErr := buildLoadDocumentsRuntime(fsys, root)
	assertEntrypointRuntimeDigest(t, roots, rootsErr, loadDocs, loadDocsErr)
}

func schemaFS(schema string) fstest.MapFS {
	return fstest.MapFS{"root.xsd": {Data: []byte(schema)}}
}

func buildPrepareRootsRuntime(fsys fs.FS, root string) (*runtime.Schema, error) {
	prepared, err := PrepareRoots(LoadConfig{Roots: []Root{{FS: fsys, Location: root}}})
	if err != nil {
		return nil, err
	}
	return prepared.Build(BuildConfig{})
}

func buildLoadDocumentsRuntime(fsys fs.FS, root string) (*runtime.Schema, error) {
	loader := newLoader(Root{FS: fsys, Location: root}, LoadConfig{})
	loadDocs, err := loader.LoadDocuments(root)
	if err != nil {
		return nil, err
	}
	prepared, err := Prepare(loadDocs)
	if err != nil {
		return nil, err
	}
	return prepared.Build(BuildConfig{})
}

func assertEntrypointRuntimeDigest(t *testing.T, roots *runtime.Schema, rootsErr error, loadDocs *runtime.Schema, loadDocsErr error) {
	t.Helper()
	if (rootsErr != nil) != (loadDocsErr != nil) {
		t.Fatalf("entrypoint error mismatch: PrepareRoots=%v LoadDocuments=%v", rootsErr, loadDocsErr)
	}
	if rootsErr != nil {
		return
	}
	if roots.BuildHash != loadDocs.BuildHash {
		t.Fatalf("build hash mismatch: PrepareRoots=%x LoadDocuments=%x\n%s", roots.BuildHash, loadDocs.BuildHash, entrypointRuntimeDiff(roots, loadDocs))
	}
	if got, want := loadDocs.CanonicalDigest(), roots.CanonicalDigest(); got != want {
		t.Fatalf("canonical digest mismatch: PrepareRoots=%x LoadDocuments=%x\n%s", want, got, entrypointRuntimeDiff(roots, loadDocs))
	}
	if diff := entrypointRuntimeDiff(roots, loadDocs); diff != "" {
		t.Fatalf("entrypoint runtime mismatch:\n%s", diff)
	}
}

func entrypointRuntimeDiff(roots, loadDocs *runtime.Schema) string {
	var out strings.Builder
	checks := []struct {
		name          string
		roots, loaded int
	}{
		{"symbols", roots.Symbols.Count(), loadDocs.Symbols.Count()},
		{"namespaces", roots.Namespaces.Count(), loadDocs.Namespaces.Count()},
		{"global types", len(roots.GlobalTypes), len(loadDocs.GlobalTypes)},
		{"global elements", len(roots.GlobalElements), len(loadDocs.GlobalElements)},
		{"global attributes", len(roots.GlobalAttributes), len(loadDocs.GlobalAttributes)},
		{"types", len(roots.Types), len(loadDocs.Types)},
		{"ancestors", len(roots.Ancestors.IDs), len(loadDocs.Ancestors.IDs)},
		{"ancestor masks", len(roots.Ancestors.Masks), len(loadDocs.Ancestors.Masks)},
		{"complex types", len(roots.ComplexTypes), len(loadDocs.ComplexTypes)},
		{"elements", len(roots.Elements), len(loadDocs.Elements)},
		{"attributes", len(roots.Attributes), len(loadDocs.Attributes)},
		{"attr uses", len(roots.AttrIndex.Uses), len(loadDocs.AttrIndex.Uses)},
		{"attr hash tables", len(roots.AttrIndex.HashTables), len(loadDocs.AttrIndex.HashTables)},
		{"dfa models", len(roots.Models.DFA), len(loadDocs.Models.DFA)},
		{"nfa models", len(roots.Models.NFA), len(loadDocs.Models.NFA)},
		{"all models", len(roots.Models.All), len(loadDocs.Models.All)},
		{"all substitutions", len(roots.Models.AllSubst), len(loadDocs.Models.AllSubst)},
		{"wildcards", len(roots.Wildcards), len(loadDocs.Wildcards)},
		{"wildcard namespaces", len(roots.WildcardNS), len(loadDocs.WildcardNS)},
		{"identity constraints", len(roots.ICs), len(loadDocs.ICs)},
		{"element identity refs", len(roots.ElemICs), len(loadDocs.ElemICs)},
		{"identity selectors", len(roots.ICSelectors), len(loadDocs.ICSelectors)},
		{"identity fields", len(roots.ICFields), len(loadDocs.ICFields)},
		{"paths", len(roots.Paths), len(loadDocs.Paths)},
		{"validator meta", len(roots.Validators.Meta), len(loadDocs.Validators.Meta)},
		{"string validators", len(roots.Validators.String), len(loadDocs.Validators.String)},
		{"list validators", len(roots.Validators.List), len(loadDocs.Validators.List)},
		{"union validators", len(roots.Validators.Union), len(loadDocs.Validators.Union)},
		{"union members", len(roots.Validators.UnionMembers), len(loadDocs.Validators.UnionMembers)},
		{"facets", len(roots.Facets), len(loadDocs.Facets)},
		{"patterns", len(roots.Patterns), len(loadDocs.Patterns)},
		{"enum keys", len(roots.Enums.Keys), len(loadDocs.Enums.Keys)},
		{"enum hashes", len(roots.Enums.Hashes), len(loadDocs.Enums.Hashes)},
		{"values bytes", len(roots.Values.Blob), len(loadDocs.Values.Blob)},
		{"notations", len(roots.Notations), len(loadDocs.Notations)},
	}
	for _, check := range checks {
		if check.roots != check.loaded {
			fmt.Fprintf(&out, "%s: roots=%d LoadDocuments=%d\n", check.name, check.roots, check.loaded)
		}
	}
	for _, check := range runtimeTableEqualityChecks(roots, loadDocs) {
		if !check.equal {
			fmt.Fprintf(&out, "%s differs\n", check.name)
		}
	}
	if diff := runtimeTypeDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeSymbolDiff(roots, loadDocs, 12); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeValidatorDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeElementDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeComplexTypeDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeAttributeDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeAttrIndexDiff(roots, loadDocs, 12); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeWildcardDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeModelDiff(roots, loadDocs, 4); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimePathDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if diff := runtimeValueBlobDiff(roots, loadDocs, 8); diff != "" {
		out.WriteString(diff)
	}
	if !reflect.DeepEqual(roots.Facets, loadDocs.Facets) {
		fmt.Fprintf(&out, "facets: roots=%v LoadDocuments=%v\n", roots.Facets, loadDocs.Facets)
	}
	return out.String()
}

type entrypointTableCheck struct {
	name  string
	equal bool
}

func runtimeSymbolDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(roots.Symbols.Count(), loadDocs.Symbols.Count())
	for id := 1; id < n && limit > 0; id++ {
		g := runtimeSymbolName(roots, runtime.SymbolID(id))
		d := runtimeSymbolName(loadDocs, runtime.SymbolID(id))
		if g == d {
			continue
		}
		fmt.Fprintf(&out, "symbol[%d]: roots=%s LoadDocuments=%s\n", id, g, d)
		limit--
	}
	if roots.Symbols.Count() != loadDocs.Symbols.Count() && limit > 0 {
		for id := n; id < roots.Symbols.Count() && limit > 0; id++ {
			fmt.Fprintf(&out, "symbol[%d]: roots=%s LoadDocuments=<missing>\n", id, runtimeSymbolName(roots, runtime.SymbolID(id)))
			limit--
		}
		for id := n; id < loadDocs.Symbols.Count() && limit > 0; id++ {
			fmt.Fprintf(&out, "symbol[%d]: roots=<missing> LoadDocuments=%s\n", id, runtimeSymbolName(loadDocs, runtime.SymbolID(id)))
			limit--
		}
	}
	return out.String()
}

func runtimeTypeDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Types), len(loadDocs.Types))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Types[id]
		d := loadDocs.Types[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "type[%d]: roots={name:%s kind:%d base:%d deriv:%d validator:%d complex:%d} LoadDocuments={name:%s kind:%d base:%d deriv:%d validator:%d complex:%d}\n",
			id,
			runtimeSymbolName(roots, g.Name), g.Kind, g.Base, g.Derivation, g.Validator, g.Complex.ID,
			runtimeSymbolName(loadDocs, d.Name), d.Kind, d.Base, d.Derivation, d.Validator, d.Complex.ID,
		)
		limit--
	}
	return out.String()
}

func runtimeSymbolName(schema *runtime.Schema, id runtime.SymbolID) string {
	if schema == nil || id == 0 || int(id) >= len(schema.Symbols.NS) {
		return ""
	}
	ns := schema.Namespaces.Bytes(schema.Symbols.NS[id])
	local := schema.Symbols.LocalBytes(id)
	return string(ns) + ":" + string(local)
}

func runtimeValidatorDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Validators.Meta), len(loadDocs.Validators.Meta))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Validators.Meta[id]
		d := loadDocs.Validators.Meta[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "validator[%d]: roots={kind:%d ws:%d flags:%d facets:%d/%d index:%d} LoadDocuments={kind:%d ws:%d flags:%d facets:%d/%d index:%d}\n",
			id,
			g.Kind, g.WhiteSpace, g.Flags, g.Facets.Off, g.Facets.Len, g.Index,
			d.Kind, d.WhiteSpace, d.Flags, d.Facets.Off, d.Facets.Len, d.Index,
		)
		limit--
	}
	return out.String()
}

func runtimeElementDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Elements), len(loadDocs.Elements))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Elements[id]
		d := loadDocs.Elements[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "element[%d]: roots={name:%s type:%d subst:%d block:%d final:%d ic:%d/%d} LoadDocuments={name:%s type:%d subst:%d block:%d final:%d ic:%d/%d}\n",
			id,
			runtimeSymbolName(roots, g.Name), g.Type, g.SubstHead, g.Block, g.Final, g.ICOff, g.ICLen,
			runtimeSymbolName(loadDocs, d.Name), d.Type, d.SubstHead, d.Block, d.Final, d.ICOff, d.ICLen,
		)
		limit--
	}
	return out.String()
}

func runtimeComplexTypeDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.ComplexTypes), len(loadDocs.ComplexTypes))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.ComplexTypes[id]
		d := loadDocs.ComplexTypes[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "complex[%d %s]: roots={content:%d mixed:%v attrs:%d/%d model:%d/%d any:%d text:%d} LoadDocuments={content:%d mixed:%v attrs:%d/%d model:%d/%d any:%d text:%d}\n",
			id,
			runtimeComplexTypeName(roots, runtime.ComplexTypeRef{ID: uint32(id)}),
			g.Content, g.Mixed, g.Attrs.Off, g.Attrs.Len, g.Model.Kind, g.Model.ID, g.AnyAttr, g.TextValidator,
			d.Content, d.Mixed, d.Attrs.Off, d.Attrs.Len, d.Model.Kind, d.Model.ID, d.AnyAttr, d.TextValidator,
		)
		limit--
	}
	return out.String()
}

func runtimeComplexTypeName(schema *runtime.Schema, ref runtime.ComplexTypeRef) string {
	if schema == nil || ref.ID == 0 {
		return ""
	}
	for _, typ := range schema.Types {
		if typ.Complex.ID == ref.ID {
			return runtimeSymbolName(schema, typ.Name)
		}
	}
	return ""
}

func runtimeAttributeDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Attributes), len(loadDocs.Attributes))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Attributes[id]
		d := loadDocs.Attributes[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "attribute[%d]: roots={name:%s validator:%d def:%v fixed:%v} LoadDocuments={name:%s validator:%d def:%v fixed:%v}\n",
			id,
			runtimeSymbolName(roots, g.Name), g.Validator, g.Default.Present, g.Fixed.Present,
			runtimeSymbolName(loadDocs, d.Name), d.Validator, d.Default.Present, d.Fixed.Present,
		)
		limit--
	}
	return out.String()
}

func runtimeAttrIndexDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.AttrIndex.Uses), len(loadDocs.AttrIndex.Uses))
	for id := 0; id < n && limit > 0; id++ {
		g := roots.AttrIndex.Uses[id]
		d := loadDocs.AttrIndex.Uses[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "attrUse[%d]: roots={name:%s validator:%d use:%d def:%v fixed:%v} LoadDocuments={name:%s validator:%d use:%d def:%v fixed:%v}\n",
			id,
			runtimeSymbolName(roots, g.Name), g.Validator, g.Use, g.Default.Present, g.Fixed.Present,
			runtimeSymbolName(loadDocs, d.Name), d.Validator, d.Use, d.Default.Present, d.Fixed.Present,
		)
		limit--
	}
	return out.String()
}

func runtimeWildcardDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Wildcards), len(loadDocs.Wildcards))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Wildcards[id]
		d := loadDocs.Wildcards[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "wildcard[%d]: roots=%+v LoadDocuments=%+v\n", id, g, d)
		limit--
	}
	return out.String()
}

func runtimePathDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	n := min(len(roots.Paths), len(loadDocs.Paths))
	for id := 1; id < n && limit > 0; id++ {
		g := roots.Paths[id]
		d := loadDocs.Paths[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "path[%d]: roots=%v LoadDocuments=%v\n", id, g.Ops, d.Ops)
		limit--
	}
	return out.String()
}

func runtimeValueBlobDiff(roots, loadDocs *runtime.Schema, limit int) string {
	_ = limit
	if reflect.DeepEqual(roots.Values, loadDocs.Values) {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "values blob: roots=%q LoadDocuments=%q\n", roots.Values.Blob, loadDocs.Values.Blob)
	return out.String()
}

func runtimeModelDiff(roots, loadDocs *runtime.Schema, limit int) string {
	var out strings.Builder
	nfaN := min(len(roots.Models.NFA), len(loadDocs.Models.NFA))
	for id := 1; id < nfaN && limit > 0; id++ {
		g := roots.Models.NFA[id]
		d := loadDocs.Models.NFA[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "nfa[%d]: roots={matchers:%d follow:%d nullable:%v start:%d/%d accept:%d/%d} LoadDocuments={matchers:%d follow:%d nullable:%v start:%d/%d accept:%d/%d}\n",
			id,
			len(g.Matchers), len(g.Follow), g.Nullable, g.Start.Off, g.Start.Len, g.Accept.Off, g.Accept.Len,
			len(d.Matchers), len(d.Follow), d.Nullable, d.Start.Off, d.Start.Len, d.Accept.Off, d.Accept.Len,
		)
		limit--
	}
	dfaN := min(len(roots.Models.DFA), len(loadDocs.Models.DFA))
	for id := 1; id < dfaN && limit > 0; id++ {
		g := roots.Models.DFA[id]
		d := loadDocs.Models.DFA[id]
		if reflect.DeepEqual(g, d) {
			continue
		}
		fmt.Fprintf(&out, "dfa[%d]: roots={states:%d transitions:%d wildcards:%d start:%d} LoadDocuments={states:%d transitions:%d wildcards:%d start:%d}\n",
			id,
			len(g.States), len(g.Transitions), len(g.Wildcards), g.Start,
			len(d.States), len(d.Transitions), len(d.Wildcards), d.Start,
		)
		limit--
	}
	return out.String()
}

func hasRuntimeNameSymbol(plan schemair.RuntimeNamePlan, name schemair.Name) bool {
	for _, op := range plan.Ops {
		if op.Kind == schemair.RuntimeNameSymbol && op.Name == name {
			return true
		}
	}
	return false
}

func runtimeTableEqualityChecks(roots, loadDocs *runtime.Schema) []entrypointTableCheck {
	return []entrypointTableCheck{
		{"symbols table", reflect.DeepEqual(roots.Symbols, loadDocs.Symbols)},
		{"namespace table", reflect.DeepEqual(roots.Namespaces, loadDocs.Namespaces)},
		{"global types", reflect.DeepEqual(roots.GlobalTypes, loadDocs.GlobalTypes)},
		{"global elements", reflect.DeepEqual(roots.GlobalElements, loadDocs.GlobalElements)},
		{"global attributes", reflect.DeepEqual(roots.GlobalAttributes, loadDocs.GlobalAttributes)},
		{"types", reflect.DeepEqual(roots.Types, loadDocs.Types)},
		{"ancestors", reflect.DeepEqual(roots.Ancestors, loadDocs.Ancestors)},
		{"complex types", reflect.DeepEqual(roots.ComplexTypes, loadDocs.ComplexTypes)},
		{"elements", reflect.DeepEqual(roots.Elements, loadDocs.Elements)},
		{"attributes", reflect.DeepEqual(roots.Attributes, loadDocs.Attributes)},
		{"attr index", reflect.DeepEqual(roots.AttrIndex, loadDocs.AttrIndex)},
		{"validators", reflect.DeepEqual(roots.Validators, loadDocs.Validators)},
		{"facets", reflect.DeepEqual(roots.Facets, loadDocs.Facets)},
		{"patterns", len(roots.Patterns) == len(loadDocs.Patterns)},
		{"enums", reflect.DeepEqual(roots.Enums, loadDocs.Enums)},
		{"values", reflect.DeepEqual(roots.Values, loadDocs.Values)},
		{"models", reflect.DeepEqual(roots.Models, loadDocs.Models)},
		{"wildcards", reflect.DeepEqual(roots.Wildcards, loadDocs.Wildcards)},
		{"wildcard namespaces", reflect.DeepEqual(roots.WildcardNS, loadDocs.WildcardNS)},
		{"identity constraints", reflect.DeepEqual(roots.ICs, loadDocs.ICs)},
		{"element identity refs", reflect.DeepEqual(roots.ElemICs, loadDocs.ElemICs)},
		{"identity selectors", reflect.DeepEqual(roots.ICSelectors, loadDocs.ICSelectors)},
		{"identity fields", reflect.DeepEqual(roots.ICFields, loadDocs.ICFields)},
		{"paths", reflect.DeepEqual(roots.Paths, loadDocs.Paths)},
		{"notations", reflect.DeepEqual(roots.Notations, loadDocs.Notations)},
	}
}
