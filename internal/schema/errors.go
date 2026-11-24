package schema

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaLocated interface {
	schemaLocation() (string, int, int)
}

func schemaCompileAt(n schemaLocated, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.SchemaCompile(code, msg, nil)
	}
	path, line, column := n.schemaLocation()
	return xsderrors.WithLocation(path, line, column, xsderrors.SchemaCompile(code, msg, nil))
}

// wrapUnlessLimit adds context to err, except that a schema.limit diagnostic is
// returned as is: location is attached only to a direct diagnostic, and a
// budget failure must stay locatable by the enclosing component scope.
func wrapUnlessLimit(err error, context string) error {
	if isSchemaLimit(err) {
		return err
	}
	return fmt.Errorf("%s: %w", context, err)
}

func isSchemaLimit(err error) bool {
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	return ok && diagnostic.Code() == xsderrors.CodeSchemaLimit
}

// buildDefectError reports a state that only a corrupt build reaches, such as an
// out-of-range ID or an unknown kind. A schema violation found by the same
// check is a plain error.
type buildDefectError struct {
	message string
}

func (e *buildDefectError) Error() string { return e.message }

func newBuildDefect(message string) error {
	return &buildDefectError{message: message}
}

// isBuildDefect reports a build defect of this package or corrupt value
// metadata, which value.ErrMetadata identifies.
func isBuildDefect(err error) bool {
	if _, ok := errors.AsType[*buildDefectError](err); ok {
		return true
	}
	return errors.Is(err, value.ErrMetadata)
}

// compileRuleError translates the failure of a shared rule check into a
// compile diagnostic: a build defect becomes an internal invariant, an
// existing diagnostic passes through, and any other failure is a violation of
// the schema reported under code, after subject when subject is not empty.
func compileRuleError(code xsderrors.Code, subject string, err error) error {
	if err == nil {
		return nil
	}
	if isBuildDefect(err) {
		return xsderrors.InternalInvariant(subject, err)
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
		return err
	}
	if subject == "" {
		return xsderrors.SchemaCompile(code, err.Error(), nil)
	}
	return xsderrors.SchemaCompile(code, subject+": "+err.Error(), nil)
}

// untypedFailure makes an error that reached the compile or publication entry
// with no diagnostic in its chain an internal invariant with the error as its
// cause. Every failure compilation reports to a schema author or caller is a
// diagnostic, so an untyped one is a defect of this package, whether a build
// defect that only propagating callers carried up or a plain error.
func untypedFailure(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*xsderrors.Error](err); ok {
		return err
	}
	return xsderrors.InternalInvariant("compilation failed without a diagnostic", err)
}

// auditError translates a failure of the publication audit. The audit
// re-checks state that compilation already admitted, so any failure other than
// a schema.limit diagnostic or an internal invariant, including a schema
// diagnostic, is a defect of this package.
func auditError(subject string, err error) error {
	if err == nil {
		return nil
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
		if code := diagnostic.Code(); code == xsderrors.CodeSchemaLimit || code == xsderrors.CodeInternalInvariant {
			return err
		}
	}
	return xsderrors.InternalInvariant(subject, err)
}

func withSchemaCompileLocation(n schemaLocated, err error) error {
	if n == nil || err == nil {
		return err
	}
	path, line, column := n.schemaLocation()
	return xsderrors.WithLocation(path, line, column, err)
}

// admittedValueInvariant converts err, the failure of a lexical parser on an
// attribute value whose start tag was admitted, into an internal invariant:
// checkSchemaAttributeValueSyntax owns the diagnostic of such a value.
func admittedValueInvariant(err error) error {
	if err == nil {
		return nil
	}
	return xsderrors.InternalInvariant("start tag admitted an invalid attribute value", err)
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
