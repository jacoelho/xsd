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
	return xsderrors.SchemaCompileAt(path, n.Line, n.Column, code, msg)
}

func withSchemaCompileLocation(n *rawNode, err error) error {
	if n == nil || err == nil {
		return err
	}
	path := ""
	if n.doc != nil {
		path = n.doc.name
	}
	return xsderrors.WithSchemaCompileLocation(path, n.Line, n.Column, err)
}

func unsupportedAtSchemaNode(n *rawNode, code xsderrors.Code, msg string) error {
	if n == nil {
		return xsderrors.Unsupported(code, msg)
	}
	path := ""
	if n.doc != nil {
		path = n.doc.name
	}
	return xsderrors.UnsupportedAt(code, n.Line, n.Column, path, msg, nil)
}
