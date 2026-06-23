package compile

import "github.com/jacoelho/xsd/xsderrors"

func schemaCompileAt(n *rawNode, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.SchemaCompile(code, msg)
	}
	return schemaCompileAtPosition(n.Line, n.Column, code, msg)
}

func schemaCompileAtPosition(line, col int, code xsderrors.Code, msg string) error {
	if line == 0 && col == 0 {
		return xsderrors.SchemaCompile(code, msg)
	}
	return xsderrors.SchemaCompileAt(line, col, code, msg)
}

func withSchemaCompileLocation(n *rawNode, err error) error {
	if n == nil || err == nil {
		return err
	}
	return xsderrors.WithSchemaCompileLocation(n.Line, n.Column, err)
}
