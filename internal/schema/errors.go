package schema

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/xsderrors"
)

type schemaLocated interface {
	schemaLocation() (string, int, int)
}

func schemaCompileAt(n schemaLocated, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.SchemaCompile(code, msg)
	}
	path, line, column := n.schemaLocation()
	return xsderrors.WithLocation(path, line, column, xsderrors.SchemaCompile(code, msg))
}

// wrapUnlessLimit adds context to err, except that a schema.limit diagnostic is
// returned as is: location is attached only to a direct diagnostic, and a
// budget failure must stay locatable by the enclosing component scope.
func wrapUnlessLimit(err error, context string) error {
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic.Code() == xsderrors.CodeSchemaLimit {
		return err
	}
	return fmt.Errorf("%s: %w", context, err)
}

func withSchemaCompileLocation(n schemaLocated, err error) error {
	if n == nil || err == nil {
		return err
	}
	path, line, column := n.schemaLocation()
	return xsderrors.WithLocation(path, line, column, err)
}

func unsupportedAtSchemaNode(n schemaLocated, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.Unsupported(code, msg, nil)
	}
	path, line, column := n.schemaLocation()
	return xsderrors.WithLocation(path, line, column, xsderrors.Unsupported(code, msg, nil))
}

func schemaParseAt(line, col int, code xsderrors.Code, msg string, cause error) error {
	return xsderrors.WithLocation("", line, col, xsderrors.SchemaParse(code, msg, cause))
}
