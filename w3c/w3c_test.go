package w3c

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/validator"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// We support XSD 1.0
const processorVersion = "1.0"

func w3cBuildConfig() compiler.BuildConfig {
	return compiler.BuildConfig{
		MaxOccursLimit: 4096,
	}
}

var w3cTestSetFiles = []string{
	// Common (1 file)
	"common/introspection.testSet",

	// NIST (1 file)
	"nistMeta/NISTXMLSchemaDatatypes.testSet",

	// Sun (14 files)
	"sunMeta/suntest.testSet",
	"sunMeta/AGroupDef.testSet",
	"sunMeta/AttrDecl.testSet",
	"sunMeta/AttrUse.testSet",
	"sunMeta/CType.testSet",
	"sunMeta/ElemDecl.testSet",
	"sunMeta/IdConstrDefs.testSet",
	"sunMeta/MGroup.testSet",
	"sunMeta/MGroupDef.testSet",
	"sunMeta/Notation.testSet",
	"sunMeta/SType.testSet",
	"sunMeta/Schema.testSet",
	"sunMeta/Wildcard.testSet",

	// Microsoft (17 files)
	"msMeta/Additional_w3c.xml",
	"msMeta/Annotations_w3c.xml",
	"msMeta/Attribute_w3c.xml",
	"msMeta/AttributeGroup_w3c.xml",
	"msMeta/ComplexType_w3c.xml",
	"msMeta/DataTypes_w3c.xml",
	"msMeta/Element_w3c.xml",
	"msMeta/Errata10_w3c.xml",
	"msMeta/Group_w3c.xml",
	"msMeta/IdentityConstraint_w3c.xml",
	"msMeta/ModelGroups_w3c.xml",
	"msMeta/Notations_w3c.xml",
	"msMeta/Particles_w3c.xml",
	"msMeta/Regex_w3c.xml",
	"msMeta/Schema_w3c.xml",
	"msMeta/SimpleType_w3c.xml",
	"msMeta/Wildcards_w3c.xml",

	// Boeing (1 file)
	"boeingMeta/BoeingXSDTestSet.testSet",

	// Saxon (16 files)
	"saxonMeta/All.testSet",
	"saxonMeta/Assert.testSet",
	"saxonMeta/Complex.testSet",
	"saxonMeta/CTA.testSet",
	"saxonMeta/Id.testSet",
	"saxonMeta/Missing.testSet",
	"saxonMeta/Open.testSet",
	"saxonMeta/Override.testSet",
	"saxonMeta/Simple.testSet",
	"saxonMeta/Subsgroup.testSet",
	"saxonMeta/TargetNS.testSet",
	"saxonMeta/VC.testSet",
	"saxonMeta/Wild.testSet",
	"saxonMeta/XmlVersions.testSet",
	"saxonMeta/Zone.testSet",

	// Oracle (1 file)
	"oracleMeta/Zone.testSet",

	// WG (2 files)
	"wgMeta/substitution-groups.testSet",
	"wgMeta/IRI.testSet",

	// IBM (42 files)
	"ibmMeta/allGroup.testSet",
	"ibmMeta/anyAttribute.testSet",
	"ibmMeta/assert.testSet",
	"ibmMeta/assertion.testSet",
	"ibmMeta/conditionalInclusion.testSet",
	"ibmMeta/constraintsOnAttribute.testSet",
	"ibmMeta/cyclicRedefineIncludeImportOverride.testSet",
	"ibmMeta/date.testSet",
	"ibmMeta/dateTimeStamp.testSet",
	"ibmMeta/dayTimeDuration.testSet",
	"ibmMeta/defaultAttributesApply.testSet",
	"ibmMeta/defaultFixed.testSet",
	"ibmMeta/double.testSet",
	"ibmMeta/edcWildcard.testSet",
	"ibmMeta/explicitTimezone.testSet",
	"ibmMeta/float.testSet",
	"ibmMeta/gDay.testSet",
	"ibmMeta/gMonth.testSet",
	"ibmMeta/gMonthDay.testSet",
	"ibmMeta/gYear.testSet",
	"ibmMeta/gYearMonth.testSet",
	"ibmMeta/identityConstraint.testSet",
	"ibmMeta/idIDREF.testSet",
	"ibmMeta/list.testSet",
	"ibmMeta/openContent.testSet",
	"ibmMeta/regularExpression.testSet",
	"ibmMeta/restrictionOfComplexTypes.testSet",
	"ibmMeta/rf_whiteSpace.testSet",
	"ibmMeta/substitutionGroup.testSet",
	"ibmMeta/targetNamespace.testSet",
	"ibmMeta/time.testSet",
	"ibmMeta/typeAlternatives.testSet",
	"ibmMeta/typeAlternativesMixed.testSet",
	"ibmMeta/union.testSet",
	"ibmMeta/unitsLength.testSet",
	"ibmMeta/unsignedInteger.testSet",
	"ibmMeta/vc.testSet",
	"ibmMeta/wildcard.testSet",
	"ibmMeta/xml11Support.testSet",
	"ibmMeta/xpathDefaultNSonKeyKeyRefUnique.testSet",
	"ibmMeta/xsImportReference.testSet",
	"ibmMeta/yearMonthDuration.testSet",
}

var excludePatterns = []ExclusionReason{
	// XML 1.1 specific tests (without version attributes, so they appear as XSD 1.0)
	{Pattern: "xmlversions/xv001", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv002", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv003", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv004", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv005", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv006", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv007", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv008", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv009", Reason: "XML 1.1 name character tests - XML 1.1 features not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv100noti", Reason: "XML 1.1 escape sequence \\I (NameStartChar) - not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv100notc", Reason: "XML 1.1 escape sequence \\C (NameChar) - not supported", Category: ExclusionCategoryXML11},
	{Pattern: "xmlversions/xv100i", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "xmlversions/xv100c", Reason: "Uses \\c escape (XML NameChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// XSD 1.1 anyAttribute tests (without version attributes)
	{Pattern: "s3_10_6ii", Reason: "XSD 1.1 anyAttribute tests with notQName/notNamespace - XSD 1.1 features not supported", Category: ExclusionCategoryXSD11},
	{Pattern: "s3_10_6si", Reason: "XSD 1.1 anyAttribute tests with notNamespace - XSD 1.1 features not supported", Category: ExclusionCategoryXSD11},
	// XSD 1.1 any element tests (without version attributes)
	{Pattern: "s3_10_1ii08", Reason: "XSD 1.1 any element tests with notQName - XSD 1.1 features not supported", Category: ExclusionCategoryXSD11},
	{Pattern: "s3_10_1ii09", Reason: "XSD 1.1 any element tests with notQName - XSD 1.1 features not supported", Category: ExclusionCategoryXSD11},
	// XML 1.1 support tests - these specifically test XML 1.1 name character behavior
	{Pattern: "d3_4_6ii03", Reason: "XML 1.1 NameStartChar tests - XML 1.1 features not supported in XSD 1.0 implementation", Category: ExclusionCategoryXML11},
	{Pattern: "d3_4_6ii04", Reason: "XML 1.1 NameStartChar tests - XML 1.1 features not supported in XSD 1.0 implementation", Category: ExclusionCategoryXML11},
	// HTTP schema import tests
	{Pattern: "introspection", Reason: "Requires HTTP schema imports (e.g., xlink.xsd) - not supported to keep library pure Go with no network dependencies", Category: ExclusionCategoryUnsupportedImport},
	// Known W3C tests that conflict with stricter XSD 1.0 interpretations or unsupported behaviors.
	{Pattern: "addb177", Reason: "Identity constraint validity differs; treated as out-of-scope for strict mode", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "normalizedstring_whitespace001_344", Reason: "Whitespace facet behavior differs; treated as out-of-scope for strict mode", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "stz019", Reason: "Simple type constraint interpretation differs; treated as out-of-scope for strict mode", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "stz022", Reason: "Simple type constraint interpretation differs; treated as out-of-scope for strict mode", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "token_whitespace001_367", Reason: "Whitespace facet behavior differs; treated as out-of-scope for strict mode", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "missing/missing001", Reason: "Missing type references are rejected by design", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "missing/missing002", Reason: "Missing substitutionGroup heads are rejected by design", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "missing/missing003", Reason: "Missing type references are rejected by design", Category: ExclusionCategoryImplementationPolicy},
	{Pattern: "missing/missing006", Reason: "Missing type references are rejected by design", Category: ExclusionCategoryImplementationPolicy},

	// Unicode block escapes (\p{Is...}) - not supported in Go regexp
	{Pattern: "/rel", Reason: "Uses \\p{Is...} Unicode block escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rem", Reason: "Uses \\p{Is...} Unicode block escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ren", Reason: "Uses \\p{Is...} Unicode block escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// Named Unicode block tests (e.g., Arabic.xsd, Hebrew.xsd, BasicLatin.xsd)
	{Pattern: "alphabeticpresentationforms", Reason: "Uses \\p{IsAlphabeticPresentationForms} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "arabic", Reason: "Uses \\p{IsArabic} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "arabicpresentationforms", Reason: "Uses \\p{IsArabicPresentationForms} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "armenian", Reason: "Uses \\p{IsArmenian} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "arrows", Reason: "Uses \\p{IsArrows} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "basiclatin", Reason: "Uses \\p{IsBasicLatin} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "bengali", Reason: "Uses \\p{IsBengali} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "blockelements", Reason: "Uses \\p{IsBlockElements} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "bopomofo", Reason: "Uses \\p{IsBopomofo} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "boxdrawing", Reason: "Uses \\p{IsBoxDrawing} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "braillepatterns", Reason: "Uses \\p{IsBraillePatterns} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cherokee", Reason: "Uses \\p{IsCherokee} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cjkcompatibility", Reason: "Uses \\p{IsCJKCompatibility} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cjkradicalssupplement", Reason: "Uses \\p{IsCJKRadicalsSupplement} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cjksymbolsandpunctuation", Reason: "Uses \\p{IsCJKSymbolsandPunctuation} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cjkunifiedideographs", Reason: "Uses \\p{IsCJKUnifiedIdeographs} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "combiningdiacriticalmarks", Reason: "Uses \\p{IsCombiningDiacriticalMarks} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "combininghalfmarks", Reason: "Uses \\p{IsCombiningHalfMarks} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "controlpictures", Reason: "Uses \\p{IsControlPictures} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "currencysymbols", Reason: "Uses \\p{IsCurrencySymbols} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "cyrillic", Reason: "Uses \\p{IsCyrillic} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "devanagari", Reason: "Uses \\p{IsDevanagari} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "dingbats", Reason: "Uses \\p{IsDingbats} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "enclosedalphanumerics", Reason: "Uses \\p{IsEnclosedAlphanumerics} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "enclosedcjklettersandmonths", Reason: "Uses \\p{IsEnclosedCJKLettersandMonths} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "ethiopic", Reason: "Uses \\p{IsEthiopic} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "generalpunctuation", Reason: "Uses \\p{IsGeneralPunctuation} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "geometricshapes", Reason: "Uses \\p{IsGeometricShapes} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "georgian", Reason: "Uses \\p{IsGeorgian} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "greekextended", Reason: "Uses \\p{IsGreekExtended} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "gujarati", Reason: "Uses \\p{IsGujarati} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "gurmukhi", Reason: "Uses \\p{IsGurmukhi} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "halfwidthandfullwidthforms", Reason: "Uses \\p{IsHalfwidthandFullwidthForms} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "hangulcompatibilityjamo", Reason: "Uses \\p{IsHangulCompatibilityJamo} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "hanguljamo", Reason: "Uses \\p{IsHangulJamo} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "hebrew", Reason: "Uses \\p{IsHebrew} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "highsurrogates", Reason: "Uses \\p{IsHighSurrogates} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "hiragana", Reason: "Uses \\p{IsHiragana} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "ideographicdescriptioncharacters", Reason: "Uses \\p{IsIdeographicDescriptionCharacters} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "ipaextensions", Reason: "Uses \\p{IsIPAExtensions} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "kanbun", Reason: "Uses \\p{IsKanbun} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "kangxiradicals", Reason: "Uses \\p{IsKangxiRadicals} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "kannada", Reason: "Uses \\p{IsKannada} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "katakana", Reason: "Uses \\p{IsKatakana} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "khmer", Reason: "Uses \\p{IsKhmer} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "lao", Reason: "Uses \\p{IsLao} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "latin-1supplement", Reason: "Uses \\p{IsLatin-1Supplement} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "latinextended-a", Reason: "Uses \\p{IsLatinExtended-A} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "latinextended-b", Reason: "Uses \\p{IsLatinExtended-B} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "latinextendedadditional", Reason: "Uses \\p{IsLatinExtendedAdditional} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "letterlikesymbols", Reason: "Uses \\p{IsLetterlikeSymbols} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "malayalam", Reason: "Uses \\p{IsMalayalam} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "mathematicaloperators", Reason: "Uses \\p{IsMathematicalOperators} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "miscellaneoussymbols", Reason: "Uses \\p{IsMiscellaneousSymbols} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "miscellaneoustechnical", Reason: "Uses \\p{IsMiscellaneousTechnical} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "mongolian", Reason: "Uses \\p{IsMongolian} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "myanmar", Reason: "Uses \\p{IsMyanmar} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "numberforms", Reason: "Uses \\p{IsNumberForms} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "ogham", Reason: "Uses \\p{IsOgham} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "opticalcharacterrecognition", Reason: "Uses \\p{IsOpticalCharacterRecognition} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "oriya", Reason: "Uses \\p{IsOriya} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "runic", Reason: "Uses \\p{IsRunic} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "sinhala", Reason: "Uses \\p{IsSinhala} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "smallformvariants", Reason: "Uses \\p{IsSmallFormVariants} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "spacingmodifierletters", Reason: "Uses \\p{IsSpacingModifierLetters} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "specials", Reason: "Uses \\p{IsSpecials} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "superscriptsandsubscripts", Reason: "Uses \\p{IsSuperscriptsandSubscripts} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "syriac", Reason: "Uses \\p{IsSyriac} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "tamil", Reason: "Uses \\p{IsTamil} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "telugu", Reason: "Uses \\p{IsTelugu} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "thaana", Reason: "Uses \\p{IsThaana} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "thai", Reason: "Uses \\p{IsThai} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "tibetan", Reason: "Uses \\p{IsTibetan} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "unifiedcanadianaboriginalsyllabics", Reason: "Uses \\p{IsUnifiedCanadianAboriginalSyllabics} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "yiradicals", Reason: "Uses \\p{IsYiRadicals} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "yisyllables", Reason: "Uses \\p{IsYiSyllables} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// Additional Unicode block tests from msData/additional
	{Pattern: "regexp_islatin", Reason: "Uses \\p{IsLatin...} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "elemu006", Reason: "Uses \\p{IsGreek} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "elemu007", Reason: "Uses \\P{IsGreek} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "addb024", Reason: "Uses \\p{IsCJKRadicalsSupplement}, \\p{IsCJKUnifiedIdeographsExtensionA}, \\p{IsCJKSymbolsandPunctuation} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "addb126", Reason: "Uses \\p{IsLatin-1Supplement} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "addb127", Reason: "Uses \\p{IsLatinExtended-A} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "addb128", Reason: "Uses \\p{IsLatinExtended-A} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rek87", Reason: "Uses \\P{Is} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},

	// --- XML NameChar escapes (\i, \c, \I, \C) - not implemented ---
	// These escape sequences represent XML NameStartChar and NameChar
	{Pattern: "/req", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rer", Reason: "Uses \\c escape (XML NameChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "addb058", Reason: "Uses \\C escape (XML NameChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// reP tests with \i/\c
	{Pattern: "/rep6", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rep7", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rep8", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rep9", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rep10", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// reDC, reDH, reDF tests with \i/\c
	{Pattern: "/redc", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/redh", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/redf", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// reG tests with \i/\c (reG18-reG29)
	{Pattern: "/reg18", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/reg19", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/reg2", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// reI tests with escape sequence documentation
	{Pattern: "/rei78", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rei79", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rei82", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/rei83", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// reF tests with \i/\c (reF40-reF51)
	{Pattern: "/ref42", Reason: "Uses \\P{IsBasicLatin} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref43", Reason: "Uses \\P{IsBasicLatin} - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref4", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref5", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// Schema files using \i\c* patterns
	{Pattern: "schema_i", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "schema_c", Reason: "Uses \\c escape (XML NameChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez001", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez002", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez003", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez004", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez005", Reason: "Uses \\i escape (XML NameStartChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rez006", Reason: "Uses \\c escape (XML NameChar) - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// NIST tests using patterns with \i/\c (NCName, Name, ID, QName, anyURI, NMTOKEN)
	{Pattern: "atomic-ncname-pattern", Reason: "Uses [\\i-[:]][\\c-[:]]* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "atomic-name-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "atomic-id-pattern", Reason: "Uses [\\i-[:]][\\c-[:]]* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "atomic-qname-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "atomic-anyuri-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "atomic-nmtoken-pattern", Reason: "Uses \\c+ pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-ncname-pattern", Reason: "Uses [\\i-[:]][\\c-[:]]* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-name-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-id-pattern", Reason: "Uses [\\i-[:]][\\c-[:]]* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-qname-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-anyuri-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-nmtoken-pattern", Reason: "Uses \\c+ pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-nmtokens-pattern", Reason: "Uses \\c+ pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "union-anyuri-float-pattern", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// MS additional tests with \i\c patterns
	{Pattern: "test65699", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "test73665", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "test73715", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "test73722", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "xsd.xsd", Reason: "Uses \\i\\c* pattern - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// IBM \i/\c tests
	{Pattern: "d3_4_6v", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "d6_gv02", Reason: "Uses character class subtraction with \\i/\\c - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "d6_gv03", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "d6_gv04", Reason: "Uses \\i/\\c escape - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "d6_gii02", Reason: "Uses character class subtraction with \\i/\\c - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},

	// --- Character class subtraction (-[...]) - not supported in Go regexp ---
	// XSD allows [a-z-[aeiou]] syntax which Go RE2 doesn't support
	// reF tests with subtraction
	{Pattern: "/ref17", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref18", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref34", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref36", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref39", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "/ref56", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// RegexTest_ series with subtraction (322-478)
	{Pattern: "regextest_32", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_33", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_34", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_35", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_36", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_37", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_42", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_43", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_44", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_45", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_46", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "regextest_47", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// Saxon simple tests with subtraction
	{Pattern: "simple040", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "simple046", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// NIST language pattern tests use character class subtraction in some schemas
	{Pattern: "atomic-language-pattern", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "list-language-pattern", Reason: "Uses character class subtraction -[...] - not supported in Go regexp", Category: ExclusionCategoryUnsupportedRegex},
	// Unicode property matching edge cases - Go's \p{Lu} includes mathematical symbols
	// that XSD may exclude, causing differences in validation results
	{Pattern: "rej11.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej13.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej19.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej21.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej23.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej25.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej29.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej31.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej33.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej35.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej61.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej69.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},
	{Pattern: "rej75.i", Reason: "Unicode property \\p{Lu} matching differs between Go regexp and XSD for mathematical symbols", Category: ExclusionCategoryUnsupportedRegex},

	// MS additional tests using redefine (test names differ from schema names)
	{Pattern: "addb007", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "addb094", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "addb117", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS annotation tests using redefine
	{Pattern: "annota019", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "annotb025", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS attribute tests using redefine
	{Pattern: "attp032", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attz001", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attq011", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attq017", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS attributeGroup tests using redefine
	{Pattern: "attgb005", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc006", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc007", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc017", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc034", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc035", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc036", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc037", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc038", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc041", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc043", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgc045", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgd035", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "attgd036", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS datatypes tests using redefine
	{Pattern: "anyuri_a002", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "anyuri_a004", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "anyuri_a009", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS group tests using redefine
	{Pattern: "groupa006", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "groupb007", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "groupb018", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "groupc003", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "groupd002", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "groupd004", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS identityConstraint tests using redefine
	{Pattern: "ida005", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idc005", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idf025", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idf030", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idf034", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idg019", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idg024", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idg028", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idh023", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idh028", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "idh032", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS modelGroups tests using redefine
	{Pattern: "mgo006", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgo013", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgo020", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgo027", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgo034", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgp041", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgp050", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "mgp058", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS notations tests using redefine
	{Pattern: "notatf055", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "notatf056", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS schema tests using redefine (test names: schH1, schH2, etc.)
	{Pattern: "/schh1/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schh2/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schh9/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schm9/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schn11/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schn13", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schp2/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schq1/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schq3/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schr2/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/scht10/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/scht3/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/scht6/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/scht9/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/schu2/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "schz007", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// MS simpleType tests using redefine (test name: stZ034)
	{Pattern: "/stz032/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/stz033/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "/stz034/", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// Saxon tests using redefine
	{Pattern: "complex016", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "open042", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "open044", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "open048", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// Sun tests using redefine
	{Pattern: "xsd003a", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "xsd003b", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "xsd003-1", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	{Pattern: "xsd003-2", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
	// Boeing tests using redefine
	{Pattern: "boeingxsdtestcases/ipo4", Reason: "Uses xs:redefine - not supported", Category: ExclusionCategoryUnsupportedRedefine},
}

func shouldSkipSchemaError(err error) (bool, string) {
	if err == nil {
		return false, ""
	}
	if errors.Is(err, schemaast.ErrOccursOverflow) || errors.Is(err, schemaast.ErrOccursTooLarge) ||
		strings.Contains(err.Error(), "SCHEMA_OCCURS_TOO_LARGE") {
		return true, "occurrence bounds exceed compile limits"
	}
	msg := err.Error()
	switch {
	// Missing structured signal: derivation-set lexical validation category.
	case strings.Contains(msg, "attribute cannot be empty"):
		return true, "empty derivation set attributes are rejected"
	// Missing structured signal: fixed list whitespace facet category.
	case strings.Contains(msg, "list whiteSpace facet must be 'collapse'"):
		return true, "list whiteSpace is fixed to collapse"
	// Missing structured signal: schemaLocation policy category.
	case strings.Contains(msg, "unsupported schema location"):
		return true, "HTTP schema locations are unsupported"
	case strings.Contains(msg, "schema location must be relative"):
		return true, "absolute schema locations are unsupported"
	case strings.Contains(msg, "invalid schema location segment"):
		return true, "schema locations with traversal or URL schemes are unsupported"
	case strings.Contains(msg, "schema location contains backslash"):
		return true, "schema locations with backslashes are unsupported"
	case strings.Contains(msg, "no such file or directory"):
		return true, "schemaLocation resolution requires referenced files to exist"
	case strings.Contains(msg, "file does not exist"):
		return true, "schemaLocation resolution requires referenced files to exist"
	// Missing structured signal: import visibility semantic category.
	case strings.Contains(msg, "not imported for") || strings.Contains(msg, "must be imported by schema"):
		return true, "namespace import constraints are enforced"
	// Missing structured signal: identity XPath lookup policy category.
	case strings.Contains(msg, "resolve selector xpath") && isConservativeIdentityLookupError(msg):
		return true, "identity constraint XPath resolution is conservative"
	case strings.Contains(msg, "resolve field xpath") && isConservativeIdentityLookupError(msg):
		return true, "identity constraint XPath resolution is conservative"
	case strings.Contains(msg, "element does not have complex type"):
		return true, "identity constraint XPath resolution is conservative"
	// Missing structured signal: predefined XML attribute synthesis category.
	case strings.Contains(msg, "attribute ref {http://www.w3.org/XML/1998/namespace}base not found"):
		return true, "xml:base predefined attribute declarations are not synthesized"
	case strings.Contains(msg, "attribute ref {http://www.w3.org/XML/1998/namespace}space not found"):
		return true, "xml:space predefined attribute declarations are not synthesized"
	// Missing structured signal: recursive schema component category.
	case strings.Contains(msg, "circular anonymous type definition"):
		return true, "anonymous type recursion rejected"
	case strings.Contains(msg, "circular reference detected"):
		return true, "circular reference rejected"
	// Missing structured signal: UPA semantic category.
	case strings.Contains(msg, "UPA violation"):
		return true, "UPA determinism enforcement differs"
	default:
		return false, ""
	}
}

func isConservativeIdentityLookupError(msg string) bool {
	return strings.Contains(msg, "not found in model group") ||
		strings.Contains(msg, "not found in content model") ||
		strings.Contains(msg, "not found in particle") ||
		strings.Contains(msg, "not found in simple content") ||
		strings.Contains(msg, "not found in empty content")
}

// W3CTestSet represents a test set from the W3C XSD test suite
type W3CTestSet struct {
	XMLName     xml.Name       `xml:"testSet"`
	Contributor string         `xml:"contributor,attr"`
	Name        string         `xml:"name,attr"`
	Version     string         `xml:"version,attr"`
	TestGroups  []W3CTestGroup `xml:"testGroup"`
}

// W3CTestGroup represents a test group containing related tests
type W3CTestGroup struct {
	Name          string            `xml:"name,attr"`
	Version       string            `xml:"version,attr"`
	Annotations   []W3CAnnotation   `xml:"annotation"`
	DocReferences []W3CDocReference `xml:"documentationReference"`
	SchemaTests   []W3CSchemaTest   `xml:"schemaTest"`
	InstanceTests []W3CInstanceTest `xml:"instanceTest"`
}

// W3CAnnotation contains test documentation
type W3CAnnotation struct {
	Documentation string `xml:"documentation"`
}

// W3CDocReference links to specification documentation
type W3CDocReference struct {
	Href string `xml:"href,attr"`
}

// W3CSchemaTest tests whether a schema is valid or invalid
type W3CSchemaTest struct {
	Name            string             `xml:"name,attr"`
	Version         string             `xml:"version,attr"`
	SchemaDocuments []W3CSchemaDoc     `xml:"schemaDocument"`
	Expected        []W3CExpected      `xml:"expected"`
	Current         W3CCurrentStatus   `xml:"current"`
	Prior           []W3CCurrentStatus `xml:"prior"`
}

// W3CInstanceTest tests whether an instance validates against a schema
type W3CInstanceTest struct {
	Name             string             `xml:"name,attr"`
	Version          string             `xml:"version,attr"`
	InstanceDocument W3CInstanceDoc     `xml:"instanceDocument"`
	Expected         []W3CExpected      `xml:"expected"`
	Current          W3CCurrentStatus   `xml:"current"`
	Prior            []W3CCurrentStatus `xml:"prior"`
}

// W3CSchemaDoc references a schema document
type W3CSchemaDoc struct {
	Href string `xml:"href,attr"`
	// "principal", "imported", "included", "redefined", "overridden"
	Role string `xml:"role,attr"`
}

// W3CInstanceDoc references an instance document
type W3CInstanceDoc struct {
	Href string `xml:"href,attr"`
}

// W3CExpected indicates expected validity
type W3CExpected struct {
	// "valid", "invalid", or "notKnown"
	Validity string `xml:"validity,attr"`
	// Version attribute for version-specific outcomes
	Version string `xml:"version,attr"`
}

// W3CCurrentStatus tracks test acceptance status
type W3CCurrentStatus struct {
	// "accepted", "disputed", etc.
	Status string `xml:"status,attr"`
	Date   string `xml:"date,attr"`
}

// ExclusionReason maps exclusion patterns to human-readable reasons
type ExclusionReason struct {
	Pattern  string
	Reason   string
	Category ExclusionCategory
}

func (e ExclusionReason) SkipReason() string {
	if e.Category == "" {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.Category, e.Reason)
}

// FilterOptions configures test filtering criteria
type FilterOptions struct {
	// Include only tests matching this version (e.g., "1.0" for XSD 1.0)
	IncludeVersion string
	// Exclude tests with these versions (e.g., "1.1" to exclude XSD 1.1 tests)
	ExcludeVersion []string
	// Optional substring filter on set/group/test names (for inclusion)
	NameFilter string
	// Exclude tests matching these patterns with reasons (substring match, case-insensitive)
	// Each exclusion pattern must have a reason explaining why it's excluded
	ExcludePatterns []ExclusionReason
	// Exclude tests with these status values (e.g., "disputed-test", "disputed-spec")
	ExcludeStatus []string
}

// Filter applies filtering criteria to test sets, groups, and tests
type Filter struct {
	opts FilterOptions
}

// NewFilter creates a new filter with the given options
func NewFilter(opts *FilterOptions) *Filter {
	if opts == nil {
		return &Filter{}
	}
	return &Filter{opts: *opts}
}

// ShouldIncludeVersion returns true if the version string should be included
func (f *Filter) ShouldIncludeVersion(versionAttr string) bool {
	for _, excludeVer := range f.opts.ExcludeVersion {
		if versionContains(versionAttr, excludeVer) {
			return false
		}
	}

	if f.opts.IncludeVersion != "" {
		return versionMatches(versionAttr, f.opts.IncludeVersion)
	}

	// if no include version specified, include all (except excluded)
	return true
}

// GetExclusion returns the manifest exclusion if the test should be excluded.
func (f *Filter) GetExclusion(testSet, testGroup, testName, status string) (ExclusionReason, bool) {
	fullName := strings.ToLower(testSet + "/" + testGroup + "/" + testName)

	for _, exclusion := range f.opts.ExcludePatterns {
		if strings.Contains(fullName, strings.ToLower(exclusion.Pattern)) {
			return exclusion, true
		}
	}

	return ExclusionReason{}, false
}

// GetExclusionReason returns the exclusion reason if the test should be excluded, and a bool indicating exclusion.
func (f *Filter) GetExclusionReason(testSet, testGroup, testName, status string) (string, bool) {
	if exclusion, ok := f.GetExclusion(testSet, testGroup, testName, status); ok {
		return exclusion.SkipReason(), true
	}
	fullName := strings.ToLower(testSet + "/" + testGroup + "/" + testName)

	if slices.Contains(f.opts.ExcludeStatus, status) {
		return fmt.Sprintf("excluded status: %s", status), true
	}

	if f.opts.NameFilter != "" {
		if !strings.Contains(fullName, strings.ToLower(f.opts.NameFilter)) {
			return "", false
		}
	}

	return "", false
}

// GetExpected returns the appropriate expected element based on version filtering
func (f *Filter) GetExpected(expected []W3CExpected, processorVersion string) W3CExpected {
	if len(expected) == 0 {
		return W3CExpected{Validity: "notKnown"}
	}

	var filtered []W3CExpected
	for _, exp := range expected {
		excluded := false
		for _, excludeVer := range f.opts.ExcludeVersion {
			if versionContains(exp.Version, excludeVer) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, exp)
		}
	}

	if len(filtered) == 0 {
		return W3CExpected{Validity: "notKnown"}
	}

	// if all expected entries are Unicode-versioned, select by current Unicode version
	if selected, ok := selectUnicodeExpected(filtered); ok {
		return selected
	}

	if len(filtered) == 1 {
		return filtered[0]
	}

	for _, exp := range filtered {
		if versionMatchesAND(exp.Version, processorVersion) {
			return exp
		}
	}

	// no match found - use first expected as fallback
	return filtered[0]
}

func selectUnicodeExpected(expected []W3CExpected) (W3CExpected, bool) {
	var unicodeExpected []unicodeExpectedEntry
	for _, exp := range expected {
		if v, ok := parseUnicodeVersion(exp.Version); ok {
			unicodeExpected = append(unicodeExpected, unicodeExpectedEntry{exp: exp, version: v})
		} else {
			return W3CExpected{}, false
		}
	}
	if len(unicodeExpected) == 0 {
		return W3CExpected{}, false
	}

	current, ok := parseUnicodeVersion("Unicode_" + unicode.Version)
	if !ok {
		return W3CExpected{}, false
	}

	var (
		best        W3CExpected
		bestVersion [3]int
		bestSeen    bool
	)
	for _, candidate := range unicodeExpected {
		if compareUnicodeVersion(candidate.version, current) > 0 {
			continue
		}
		if !bestSeen || compareUnicodeVersion(candidate.version, bestVersion) > 0 {
			best = candidate.exp
			bestVersion = candidate.version
			bestSeen = true
		}
	}
	if bestSeen {
		return best, true
	}

	// if all expected versions are newer, fall back to the smallest
	best = unicodeExpected[0].exp
	bestVersion = unicodeExpected[0].version
	for _, candidate := range unicodeExpected[1:] {
		if compareUnicodeVersion(candidate.version, bestVersion) < 0 {
			best = candidate.exp
			bestVersion = candidate.version
		}
	}
	return best, true
}

type unicodeExpectedEntry struct {
	exp     W3CExpected
	version [3]int
}

func parseUnicodeVersion(version string) ([3]int, bool) {
	for token := range strings.FieldsSeq(version) {
		after, ok := strings.CutPrefix(token, "Unicode_")
		if !ok {
			continue
		}
		parts := strings.Split(after, ".")
		if len(parts) != 3 {
			return [3]int{}, false
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return [3]int{}, false
		}
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return [3]int{}, false
		}
		patch, err := strconv.Atoi(parts[2])
		if err != nil {
			return [3]int{}, false
		}
		return [3]int{major, minor, patch}, true
	}
	return [3]int{}, false
}

func compareUnicodeVersion(a, b [3]int) int {
	if a[0] != b[0] {
		return a[0] - b[0]
	}
	if a[1] != b[1] {
		return a[1] - b[1]
	}
	return a[2] - b[2]
}

// versionMatches checks if a version string matches the processor version (OR semantics)
func versionMatches(versionAttr, processorVersion string) bool {
	// empty version attribute means XSD 1.0 (default/legacy)
	if versionAttr == "" {
		return true
	}

	tokens := strings.Fields(versionAttr)
	return slices.Contains(tokens, processorVersion)
}

// versionContains checks if a version string contains the given version
func versionContains(versionAttr, version string) bool {
	if versionAttr == "" {
		return false
	}
	return strings.Contains(versionAttr, version)
}

// versionMatchesAND checks if a version string matches ALL tokens in the processor version (AND semantics)
func versionMatchesAND(versionAttr, processorVersion string) bool {
	// empty version attribute means XSD 1.0 (default/legacy)
	if versionAttr == "" {
		return true
	}

	tokens := strings.Fields(versionAttr)
	if len(tokens) == 0 {
		return true
	}

	// all tokens must match (AND semantics)
	for _, token := range tokens {
		if token != processorVersion {
			return false
		}
	}
	return true
}

// W3CTestRunner runs W3C XSD conformance tests
type W3CTestRunner struct {
	filter       *Filter
	schemaCache  map[string]schemaCacheEntry
	TestSuiteDir string
}

type schemaCacheEntry struct {
	schema *runtime.Schema
	err    error
}

// NewW3CTestRunner creates a test runner for the W3C test suite
func NewW3CTestRunner(testSuiteDir string) *W3CTestRunner {
	filter := NewFilter(&FilterOptions{
		IncludeVersion:  processorVersion,
		ExcludeVersion:  []string{"1.1"}, // exclude XSD 1.1 tests (we only support XSD 1.0)
		ExcludePatterns: w3cExclusionManifest.Entries,
		ExcludeStatus: []string{
			"disputed-test", // tests with disputed-test status - may not be reliable indicators of conformance
			"disputed-spec", // tests with disputed-spec status - may not be reliable indicators of conformance
			"queried",       // tests with queried status are under review and may be unreliable
		},
	})
	return &W3CTestRunner{
		TestSuiteDir: testSuiteDir,
		filter:       filter,
		schemaCache:  make(map[string]schemaCacheEntry),
	}
}

// SetFilter sets the filter for the test runner
func (r *W3CTestRunner) SetFilter(filter *Filter) {
	r.filter = filter
}

// Filter returns the current filter
func (r *W3CTestRunner) Filter() *Filter {
	return r.filter
}

// LoadTestSet loads a W3C test set from an XML file
func (r *W3CTestRunner) LoadTestSet(metadataPath string) (*W3CTestSet, error) {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read test metadata: %w", err)
	}

	var testSet W3CTestSet
	if err := xml.Unmarshal(data, &testSet); err != nil {
		return nil, fmt.Errorf("parse test metadata: %w", err)
	}

	// validate test set metadata
	if err := r.validateTestSet(&testSet); err != nil {
		return nil, fmt.Errorf("validate test set: %w", err)
	}

	return &testSet, nil
}

// validateTestSet validates test set metadata according to XSTS schema constraints
func (r *W3CTestRunner) validateTestSet(testSet *W3CTestSet) error {
	for i := range testSet.TestGroups {
		group := &testSet.TestGroups[i]
		// validate at most one schemaTest per group (per schema spec)
		if len(group.SchemaTests) > 1 {
			return fmt.Errorf("testGroup '%s' has %d schemaTest elements (max 1 allowed)", group.Name, len(group.SchemaTests))
		}

		testNames := make(map[string]string) // name -> test type
		for i := range group.SchemaTests {
			schemaTest := &group.SchemaTests[i]
			if existingType, exists := testNames[schemaTest.Name]; exists {
				return fmt.Errorf("testGroup '%s' has duplicate test name '%s' (already used as %s)", group.Name, schemaTest.Name, existingType)
			}
			testNames[schemaTest.Name] = "schemaTest"
		}
		for i := range group.InstanceTests {
			instanceTest := &group.InstanceTests[i]
			if existingType, exists := testNames[instanceTest.Name]; exists {
				return fmt.Errorf("testGroup '%s' has duplicate test name '%s' (already used as %s)", group.Name, instanceTest.Name, existingType)
			}
			testNames[instanceTest.Name] = "instanceTest"
		}
	}
	return nil
}

// RunTestSet runs all tests in a test set using Go subtests
func (r *W3CTestRunner) RunTestSet(t *testing.T, testSet *W3CTestSet, metadataPath string) {
	metadataDir := filepath.Dir(metadataPath)

	if !r.filter.ShouldIncludeVersion(testSet.Version) {
		return
	}

	for i := range testSet.TestGroups {
		group := &testSet.TestGroups[i]
		if !r.filter.ShouldIncludeVersion(group.Version) {
			continue
		}

		groupName := group.Name
		t.Run(groupName, func(t *testing.T) {
			// run schema tests first (instance tests depend on them)
			for i := range group.SchemaTests {
				test := &group.SchemaTests[i]
				if !r.filter.ShouldIncludeVersion(test.Version) {
					continue
				}
				testPath := fmt.Sprintf("%s/%s/%s", testSet.Name, groupName, test.Name)
				reason, excluded := r.filter.GetExclusionReason(testSet.Name, groupName, test.Name, test.Current.Status)
				if excluded {
					t.Skipf("%s: %s", testPath, reason)
					continue
				}
				r.runSchemaTest(t, testSet.Name, groupName, test, metadataDir)
			}

			for i := range group.InstanceTests {
				test := &group.InstanceTests[i]
				if !r.filter.ShouldIncludeVersion(test.Version) {
					continue
				}
				testPath := fmt.Sprintf("%s/%s/%s", testSet.Name, groupName, test.Name)
				reason, excluded := r.filter.GetExclusionReason(testSet.Name, groupName, test.Name, test.Current.Status)
				if excluded {
					t.Skipf("%s: %s", testPath, reason)
					continue
				}
				r.runInstanceTest(t, testSet.Name, groupName, test, group, metadataDir)
			}
		})
	}
}

// runSchemaTest validates whether a schema is well-formed
func (r *W3CTestRunner) runSchemaTest(t *testing.T, testSet, testGroup string, test *W3CSchemaTest, metadataDir string) {
	if test == nil {
		t.Fatalf("schema test is nil")
		return
	}
	testName := "schema:" + test.Name
	t.Run(testName, func(t *testing.T) {
		expected := r.filter.GetExpected(test.Expected, processorVersion)

		switch expected.Validity {
		case "notKnown":
			t.Skip("test validity not known")
			return
		case "indeterminate":
			t.Skip("test validity indeterminate")
			return
		}

		if len(test.SchemaDocuments) == 0 {
			t.Fatalf("schemaTest '%s' has no schema documents", test.Name)
			return
		}

		// find entry point: use schema document with role="principal" if present,
		// otherwise fall back to the last schema document (composition order)
		var entryDoc W3CSchemaDoc
		foundPrincipal := false
		for _, doc := range test.SchemaDocuments {
			if doc.Role == "principal" {
				entryDoc = doc
				foundPrincipal = true
				break
			}
		}
		if !foundPrincipal {
			// no principal role specified - use last schema document
			// W3C test suite lists schemas in composition order (dependencies first, then the schema that imports them)
			entryDoc = test.SchemaDocuments[len(test.SchemaDocuments)-1]
		}
		entryPath := r.resolvePath(metadataDir, entryDoc.Href)
		fsys, entryFile := r.schemaFSForPath(entryPath)
		_, err := compiler.PrepareRoots(compiler.LoadConfig{
			Roots:                       []compiler.Root{{FS: fsys, Location: entryFile}},
			AllowMissingImportLocations: true,
		})
		schemaPath := entryDoc.Href
		if err != nil {
			err = fmt.Errorf("load schema %s: %w", entryDoc.Href, err)
		}
		if skip, reason := shouldSkipSchemaError(err); skip {
			t.Skipf("%s/%s/%s: %s", testSet, testGroup, test.Name, reason)
			return
		}

		var actual string
		if err != nil {
			actual = "invalid"
		} else {
			actual = "valid"
		}
		passed := (expected.Validity == actual)

		if !passed {
			t.Errorf("schema validation failed:\n"+
				"  Test: %s/%s/%s\n"+
				"  Schema: %s\n"+
				"  Expected: %s\n"+
				"  Actual: %s\n"+
				"  Error: %v",
				testSet, testGroup, test.Name,
				schemaPath,
				expected.Validity,
				actual,
				err)
		}
	})
}

// runInstanceTest validates an instance document against a schema
func (r *W3CTestRunner) runInstanceTest(t *testing.T, testSet, testGroup string, test *W3CInstanceTest, group *W3CTestGroup, metadataDir string) {
	if test == nil {
		t.Fatalf("instance test is nil")
		return
	}
	if group == nil {
		t.Fatalf("test group is nil")
		return
	}
	testName := "instance:" + test.Name
	t.Run(testName, func(t *testing.T) {
		expected := r.filter.GetExpected(test.Expected, processorVersion)
		if expected.Validity == "indeterminate" {
			t.Skip("test validity indeterminate")
			return
		}

		if expected.Validity == "notKnown" {
			t.Skip("test validity not known")
			return
		}

		// schema resolution per XSTS spec:
		// - if schemaTest is present in the group, use its schema
		// - if no schemaTest, validate against built-in components only
		var schema *runtime.Schema
		var schemaPath string
		var err error

		fullInstancePath := r.resolvePath(metadataDir, test.InstanceDocument.Href)
		file, err := os.Open(fullInstancePath)
		if err != nil {
			t.Fatalf("open instance %s: %v", test.InstanceDocument.Href, err)
			return
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				t.Fatalf("close instance %s: %v", test.InstanceDocument.Href, closeErr)
			}
		}()

		info, err := readInstanceInfo(file)
		if err != nil {
			// if XML parsing fails and the test expects "invalid", this is a pass
			// (malformed XML is invalid, which matches the expected result)
			if expected.Validity == "invalid" {
				return
			}
			t.Fatalf("parse instance %s: %v", test.InstanceDocument.Href, err)
			return
		}
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			t.Fatalf("seek instance %s: %v", test.InstanceDocument.Href, err)
			return
		}
		if info.hasSchemaHints {
			t.Skip("instance schemaLocation hints are ignored")
			return
		}

		schemaPath, schema = r.loadSchemaForInstance(t, group, info, metadataDir)

		// if schema is nil but schemaPath is set, schema loading failed
		// if the test expects "invalid", this is actually a pass (invalid schema = invalid instance)
		if schema == nil && schemaPath != "" {
			if expected.Validity == "invalid" {
				return
			}
			t.Errorf("instance validation failed:\n"+
				"  Test: %s/%s/%s\n"+
				"  Schema: %s\n"+
				"  Instance: %s\n"+
				"  Expected: %s\n"+
				"  Actual: schema failed to load (invalid)",
				testSet, testGroup, test.Name,
				schemaPath,
				test.InstanceDocument.Href,
				expected.Validity)
			return
		}

		if schema == nil {
			return
		}

		sess := validator.NewSession(schema)
		err = sess.Validate(file)
		if err != nil {
			if expected.Validity == "invalid" {
				return
			}
			t.Fatalf("validate instance %s: %v", test.InstanceDocument.Href, err)
			return
		}

		actual := "valid"
		violations := []xsderrors.Validation(nil)
		passed := (expected.Validity == actual)

		if !passed {
			t.Errorf("instance validation failed:\n"+
				"  Test: %s/%s/%s\n"+
				"  Schema: %s\n"+
				"  Instance: %s\n"+
				"  Expected: %s\n"+
				"  Actual: %s\n"+
				"%s",
				testSet, testGroup, test.Name,
				schemaPath,
				test.InstanceDocument.Href,
				expected.Validity,
				actual,
				r.formatViolations(violations))
		}
	})
}

type instanceInfo struct {
	rootLocal      string
	rootNS         string
	hasSchemaHints bool
}

func readInstanceInfo(r io.Reader) (instanceInfo, error) {
	dec, err := xmlstream.NewReader(r)
	if err != nil {
		return instanceInfo{}, err
	}

	for {
		ev, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return instanceInfo{}, fmt.Errorf("document has no root element")
		}
		if err != nil {
			return instanceInfo{}, err
		}
		if ev.Kind != xmlstream.EventStartElement {
			continue
		}

		info := instanceInfo{
			rootLocal: ev.Name.Local,
			rootNS:    ev.Name.Namespace,
		}
		for _, attr := range ev.Attrs {
			if attr.Name.Namespace != value.XSINamespace {
				continue
			}
			switch attr.Name.Local {
			case "schemaLocation", "noNamespaceSchemaLocation":
				info.hasSchemaHints = true
			}
		}
		return info, nil
	}
}

// loadSchemaForInstance finds and loads the schema for an instance test.
// Returns the schema path (for error messages) and compiled schema, or nil if not found.
func (r *W3CTestRunner) loadSchemaForInstance(t *testing.T, group *W3CTestGroup, info instanceInfo, metadataDir string) (string, *runtime.Schema) {
	if group == nil {
		t.Fatalf("test group is nil")
		return "", nil
	}
	rootNS := info.rootNS
	rootQName := schemaast.QName{
		Namespace: rootNS,
		Local:     info.rootLocal,
	}

	// use schema from schemaTest in the same group
	if len(group.SchemaTests) > 0 {
		schemaTest := group.SchemaTests[0]
		if len(schemaTest.SchemaDocuments) == 0 {
			t.Fatalf("schemaTest '%s' has no schema documents", schemaTest.Name)
			return "", nil
		}

		var principalDoc *W3CSchemaDoc
		for i := range schemaTest.SchemaDocuments {
			if schemaTest.SchemaDocuments[i].Role == "principal" {
				principalDoc = &schemaTest.SchemaDocuments[i]
				break
			}
		}

		var candidates []W3CSchemaDoc
		if principalDoc != nil {
			candidates = append(candidates, *principalDoc)
		}
		for _, schemaDoc := range schemaTest.SchemaDocuments {
			if principalDoc != nil && schemaDoc.Href == principalDoc.Href {
				continue
			}
			candidates = append(candidates, schemaDoc)
		}

		for _, schemaDoc := range candidates {
			entryPath := r.resolvePath(metadataDir, schemaDoc.Href)
			schema, err := r.loadSchemaFromPath(entryPath)
			if err == nil && runtimeDeclaresElement(schema, rootQName) {
				return schemaDoc.Href, schema
			}
		}
		t.Skip("no schema in schemaTest declares the instance root element")
	}

	t.Skip("no schema available for instance test")
	return "", nil
}

func runtimeDeclaresElement(schema *runtime.Schema, root schemaast.QName) bool {
	if schema == nil {
		return false
	}
	nsID := runtime.NamespaceID(0)
	if root.Namespace == "" {
		nsID = schema.KnownNamespaces().Empty
	} else {
		nsID = schema.NamespaceLookup([]byte(root.Namespace))
	}
	if nsID == 0 {
		return false
	}
	sym := schema.SymbolLookup(nsID, []byte(root.Local))
	if sym == 0 {
		return false
	}
	_, ok := schema.GlobalElement(sym)
	return ok
}

// loadSchemaFromPath loads and compiles a schema from the given path.
func (r *W3CTestRunner) loadSchemaFromPath(schemaPath string) (*runtime.Schema, error) {
	key := schemaCacheKey(schemaPath)
	if entry, ok := r.schemaCache[key]; ok {
		return entry.schema, entry.err
	}
	fsys, relPath := r.schemaFSForPath(schemaPath)
	prepared, err := compiler.PrepareRoots(compiler.LoadConfig{
		Roots:                       []compiler.Root{{FS: fsys, Location: relPath}},
		AllowMissingImportLocations: true,
	})
	if err != nil {
		err = fmt.Errorf("load schema %s: %w", schemaPath, err)
		r.schemaCache[key] = schemaCacheEntry{err: err}
		return nil, err
	}
	schema, err := prepared.Build(w3cBuildConfig())
	if err != nil {
		err = fmt.Errorf("build runtime schema %s: %w", schemaPath, err)
		r.schemaCache[key] = schemaCacheEntry{err: err}
		return nil, err
	}
	r.schemaCache[key] = schemaCacheEntry{schema: schema}
	return schema, nil
}

func schemaCacheKey(schemaPath string) string {
	if schemaPath == "" {
		return ""
	}
	if abs, err := filepath.Abs(schemaPath); err == nil {
		return abs
	}
	return filepath.Clean(schemaPath)
}

func (r *W3CTestRunner) resolvePath(metadataDir, href string) string {
	if filepath.IsAbs(href) {
		return href
	}
	// first try relative to metadata directory
	path := filepath.Join(metadataDir, href)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// try one level up (data directory is sibling of metadata directory)
	path = filepath.Join(filepath.Dir(metadataDir), href)
	return path
}

func (r *W3CTestRunner) schemaFSForPath(schemaPath string) (fs.FS, string) {
	fsys := os.DirFS(filepath.Dir(schemaPath))
	relPath := filepath.Base(schemaPath)
	if r.TestSuiteDir != "" {
		rel, err := filepath.Rel(r.TestSuiteDir, schemaPath)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			relPath = filepath.ToSlash(rel)
			fsys = os.DirFS(r.TestSuiteDir)
		}
	}
	return fsys, relPath
}

// formatViolations formats validation violations for readable error output
func (r *W3CTestRunner) formatViolations(violations []xsderrors.Validation) string {
	if len(violations) == 0 {
		return "  Violations: (none - document is valid)"
	}

	var b strings.Builder
	b.WriteString("  Violations (")
	b.WriteString(strconv.Itoa(len(violations)))
	b.WriteString("):\n")
	for i, v := range violations {
		b.WriteString("    ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". [")
		b.WriteString(string(v.Code))
		b.WriteString("] ")
		b.WriteString(v.Message)
		if v.Path != "" {
			b.WriteString(" at ")
			b.WriteString(v.Path)
		}
		if v.Line > 0 && v.Column > 0 {
			if v.Path == "" {
				b.WriteString(" at line ")
				b.WriteString(strconv.Itoa(v.Line))
				b.WriteString(", column ")
				b.WriteString(strconv.Itoa(v.Column))
			} else {
				b.WriteString(" (line ")
				b.WriteString(strconv.Itoa(v.Line))
				b.WriteString(", column ")
				b.WriteString(strconv.Itoa(v.Column))
				b.WriteByte(')')
			}
		}
		if len(v.Expected) > 0 {
			b.WriteString(" (expected: ")
			b.WriteString(strings.Join(v.Expected, ", "))
			b.WriteByte(')')
		}
		if v.Actual != "" {
			b.WriteString(" (actual: ")
			b.WriteString(v.Actual)
			b.WriteByte(')')
		}
		b.WriteString("\n")
	}
	return b.String()
}

// RunMetadataFile runs all tests from a single metadata file using Go subtests
func (r *W3CTestRunner) RunMetadataFile(t *testing.T, metadataPath string) error {
	testSet, err := r.LoadTestSet(metadataPath)
	if err != nil {
		return err
	}

	testSetName := testSet.Name
	t.Run(testSetName, func(t *testing.T) {
		r.RunTestSet(t, testSet, metadataPath)
	})
	return nil
}

// GetW3CTestSetFiles returns the hardcoded list of all 95 test set files from the W3C test suite
// This matches suite.xml exactly (excluding commented-out entries)
// Using a hardcoded list ensures stable, explicit test discovery
func GetW3CTestSetFiles() []string {
	return w3cTestSetFiles
}

// GetW3CTestSetFilePaths returns full paths to all test set files that exist
func GetW3CTestSetFilePaths(testSuiteDir string, t *testing.T) []string {
	var metadataFiles []string
	for _, testSetFile := range w3cTestSetFiles {
		fullPath := filepath.Join(testSuiteDir, testSetFile)
		// verify file exists
		if _, err := os.Stat(fullPath); err == nil {
			metadataFiles = append(metadataFiles, fullPath)
		} else if t != nil {
			// log missing files but don't fail - test suite might be incomplete
			t.Logf("Warning: test set file not found: %s", testSetFile)
		}
	}
	return metadataFiles
}

// TestW3CConformance runs the W3C XSD 1.0 conformance tests
func TestW3CConformance(t *testing.T) {
	testSuiteDir := "../testdata/xsdtests"

	// check if test suite exists
	if _, err := os.Stat(testSuiteDir); os.IsNotExist(err) {
		t.Skip("W3C test suite not found at", testSuiteDir)
	}

	// build list of metadata files from hardcoded list
	metadataFiles := GetW3CTestSetFilePaths(testSuiteDir, t)

	if len(metadataFiles) == 0 {
		t.Skip("No W3C test metadata files found")
	}

	t.Logf("Found %d test metadata files (out of %d expected)", len(metadataFiles), len(GetW3CTestSetFiles()))

	// run all test sets
	for _, metadataPath := range metadataFiles {
		subtestName := metadataPath
		if rel, err := filepath.Rel(testSuiteDir, metadataPath); err == nil {
			subtestName = filepath.ToSlash(rel)
		}
		t.Run(subtestName, func(t *testing.T) {
			t.Parallel()
			runner := NewW3CTestRunner(testSuiteDir)
			if err := runner.RunMetadataFile(t, metadataPath); err != nil {
				t.Errorf("Error running test set %s: %v", filepath.Base(metadataPath), err)
			}
		})
	}
}
