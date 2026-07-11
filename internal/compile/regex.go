package compile

import (
	"regexp"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// CompilePatternFacet validates and compiles one XSD pattern facet into the
// runtime matcher representation.
func CompilePatternFacet(source string, categories RegexCategoryCache) (runtime.StringPattern, error) {
	goUnsupported, err := CheckXSDRegexSyntax(source, categories)
	if err != nil {
		return runtime.StringPattern{}, err
	}
	if fast := runtime.CompileSimpleStringPattern(source); fast != nil {
		return runtime.NewFastStringPattern(fast), nil
	}
	if goUnsupported {
		return runtime.StringPattern{}, xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "XSD regex is not representable by Go regexp: "+source)
	}
	goPattern := TranslateXSDRegexToGo(source)
	goSource := "^(?:" + goPattern + ")$"
	re, err := regexp.Compile(goSource)
	if err != nil {
		return runtime.StringPattern{}, xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "invalid or unsupported regex "+source)
	}
	return runtime.NewRegexpStringPattern(re), nil
}
