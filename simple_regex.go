package xsd

import "regexp"

func (c *compiler) compilePattern(n *rawNode, source string) (pattern, error) {
	p, err := compilePatternWithCompiler(source, c)
	if err != nil {
		return pattern{}, withSchemaCompileLocation(n, err)
	}
	return p, nil
}

func compilePatternWithCompiler(source string, c *compiler) (pattern, error) {
	goUnsupported, err := checkXSDRegexSyntaxWithCompiler(source, c)
	if err != nil {
		return pattern{}, err
	}
	if fast := compileSimplePattern(source); fast != nil {
		return pattern{XSDSource: source, Fast: fast}, nil
	}
	if goUnsupported {
		return pattern{}, unsupported(ErrUnsupportedRegex, "XSD regex is not representable by Go regexp: "+source)
	}
	goPattern := translateXSDRegexToGo(source)
	goSource := "^(?:" + goPattern + ")$"
	re, err := regexp.Compile(goSource)
	if err != nil {
		return pattern{}, unsupported(ErrUnsupportedRegex, "invalid or unsupported regex "+source)
	}
	return pattern{XSDSource: source, GoSource: goSource, RE: re}, nil
}

func (p pattern) matches(s string) bool {
	if p.Fast != nil {
		return p.Fast.match(s)
	}
	return p.RE.MatchString(s)
}

func (p pattern) matchesBytes(s []byte) bool {
	if p.Fast != nil {
		return p.Fast.matchBytes(s)
	}
	return p.RE.Match(s)
}
