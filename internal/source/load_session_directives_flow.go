package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func (s *loadSession) processDirectives(schema *parser.Schema, directives []parser.Directive) error {
	for _, directive := range directives {
		switch directive.Kind {
		case parser.DirectiveInclude:
			if err := s.processInclude(schema, directive.Include); err != nil {
				return err
			}
		case parser.DirectiveImport:
			if err := s.processImport(schema, directive.Import); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected directive kind: %d", directive.Kind)
		}
	}
	return nil
}
