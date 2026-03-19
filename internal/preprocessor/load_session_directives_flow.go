package preprocessor

import (
	"github.com/jacoelho/xsd/internal/parser"
)

func (s *loadSession) processDirectives(schema *parser.Schema, directiveList []parser.Directive) error {
	return Process(
		directiveList,
		func(include parser.IncludeInfo) error {
			return s.processInclude(schema, include)
		},
		func(imp parser.ImportInfo) error {
			return s.processImport(schema, imp)
		},
	)
}
