package identitypath

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ParseSelector parses a selector expression under selector attribute policy.
func ParseSelector(expr string, nsContext map[string]string) (xpath.Expression, error) {
	return xpath.Parse(expr, nsContext, xpath.AttributesDisallowed)
}

// ParseField parses a field expression under field attribute policy.
func ParseField(expr string, nsContext map[string]string) (xpath.Expression, error) {
	return xpath.Parse(expr, nsContext, xpath.AttributesAllowed)
}

// CompileSelector compiles a parsed selector expression into runtime programs.
func CompileSelector(expr xpath.Expression, schema *runtime.Schema) ([]runtime.PathProgram, error) {
	return xpath.CompileExpression(expr, schema)
}

// CompileField compiles a parsed field expression into runtime programs.
func CompileField(expr xpath.Expression, schema *runtime.Schema) ([]runtime.PathProgram, error) {
	return xpath.CompileExpression(expr, schema)
}
