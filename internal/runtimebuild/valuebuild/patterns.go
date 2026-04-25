package valuebuild

import (
	"regexp"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *artifactCompiler) addPattern(source string) (runtime.PatternID, error) {
	re, err := regexp.Compile(source)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(source), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *artifactCompiler) addPatternSet(values []string) (runtime.PatternID, error) {
	if len(values) == 0 {
		return 0, nil
	}
	if len(values) == 1 {
		return c.addPattern(values[0])
	}
	bodies := make([]string, 0, len(values))
	for _, pattern := range values {
		bodies = append(bodies, stripAnchors(pattern))
	}
	source := "^(?:" + strings.Join(bodies, "|") + ")$"
	return c.addPattern(source)
}

func stripAnchors(pattern string) string {
	const prefix = "^(?:"
	const suffix = ")$"
	if strings.HasPrefix(pattern, prefix) && strings.HasSuffix(pattern, suffix) {
		return pattern[len(prefix) : len(pattern)-len(suffix)]
	}
	return pattern
}
