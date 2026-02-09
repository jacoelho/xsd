package validatorcompile

import (
	"regexp"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) addPattern(p *types.Pattern) (runtime.PatternID, error) {
	if p.GoPattern == "" {
		if err := p.ValidateSyntax(); err != nil {
			return 0, err
		}
	}
	re, err := regexp.Compile(p.GoPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(p.GoPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *compiler) addPatternSet(set *types.PatternSet) (runtime.PatternID, error) {
	if set == nil || len(set.Patterns) == 0 {
		return 0, nil
	}
	if len(set.Patterns) == 1 {
		return c.addPattern(set.Patterns[0])
	}
	bodies := make([]string, 0, len(set.Patterns))
	for _, pat := range set.Patterns {
		if pat.GoPattern == "" {
			if err := pat.ValidateSyntax(); err != nil {
				return 0, err
			}
		}
		body := stripAnchors(pat.GoPattern)
		bodies = append(bodies, body)
	}
	goPattern := "^(?:" + strings.Join(bodies, "|") + ")$"
	re, err := regexp.Compile(goPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(goPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func stripAnchors(goPattern string) string {
	const prefix = "^(?:"
	const suffix = ")$"
	if strings.HasPrefix(goPattern, prefix) && strings.HasSuffix(goPattern, suffix) {
		return goPattern[len(prefix) : len(goPattern)-len(suffix)]
	}
	return goPattern
}
