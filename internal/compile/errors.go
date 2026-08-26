package compile

import "github.com/jacoelho/xsd/xsderrors"

func schemaCompileAt(n *rawNode, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.SchemaCompile(code, msg)
	}
	path := ""
	if n.doc != nil {
		path = n.doc.name
	}
	return xsderrors.WithLocation(path, n.Line, n.Column, xsderrors.SchemaCompile(code, msg))
}

func withSchemaCompileLocation(n *rawNode, err error) error {
	if n == nil || err == nil {
		return err
	}
	path := ""
	if n.doc != nil {
		path = n.doc.name
	}
	return xsderrors.WithLocation(path, n.Line, n.Column, err)
}

func unsupportedAtSchemaNode(n *rawNode, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.Unsupported(code, msg, nil)
	}
	path := ""
	if n.doc != nil {
		path = n.doc.name
	}
	return xsderrors.WithLocation(path, n.Line, n.Column, xsderrors.Unsupported(code, msg, nil))
}

func schemaParseAt(line, col int, code xsderrors.Code, msg string, cause error) error {
	return xsderrors.WithLocation("", line, col, xsderrors.SchemaParse(code, msg, cause))
}
