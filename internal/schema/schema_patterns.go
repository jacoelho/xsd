package schema

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xsdregex"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaPatterns admits the xs:pattern values of one schema set. It compiles
// each distinct pattern text once, at the start tag of the first facet that
// names it in document load order, and charges the compiled pattern's retained
// estimate against MaxSchemaPatternRetainedBytes before it keeps the pattern;
// a later facet with the same text shares that *xsdregex.Pattern and is not
// charged again. Sharing is sound because Compile takes no other input and a
// Pattern is immutable. The compiler owns it and lends it to start-tag
// admission while the set loads; MaxSchemaInstantiatedNodes bounds its entries.
type schemaPatterns struct {
	byText   map[string]*xsdregex.Pattern
	limit    int64
	retained int64
}

// newSchemaPatterns returns an empty pattern admission charged against
// limits.MaxSchemaPatternRetainedBytes.
func newSchemaPatterns(limits Limits) schemaPatterns {
	return schemaPatterns{limit: limits.MaxSchemaPatternRetainedBytes}
}

// admit returns the compiled pattern of text. A text that is no regular
// expression or exceeds a fixed xsdregex compile limit is reported as
// schemaRegexError maps it; a pattern whose retained estimate does not fit the
// remaining MaxSchemaPatternRetainedBytes is a schema.limit violation.
func (p *schemaPatterns) admit(text string) (*xsdregex.Pattern, error) {
	if p == nil {
		return nil, xsderrors.InternalInvariant("pattern admission has no schema pattern set")
	}
	if pattern, ok := p.byText[text]; ok {
		return pattern, nil
	}
	pattern, err := xsdregex.Compile(text)
	if err != nil {
		return nil, schemaRegexError(err)
	}
	n := pattern.RetainedBytes()
	// The charged total never exceeds the normalized, non-negative limit.
	remaining := uint64(p.limit - p.retained) //nolint:gosec // Normalization rejects negative limits and charges stay within them.
	if n > remaining {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit,
			fmt.Sprintf("MaxSchemaPatternRetainedBytes=%d exceeded", p.limit))
	}
	p.retained += int64(n) //nolint:gosec // n <= remaining <= MaxInt64.
	if p.byText == nil {
		p.byText = make(map[string]*xsdregex.Pattern)
	}
	p.byText[text] = pattern
	return pattern, nil
}
