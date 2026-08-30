package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func identityConstraintNodes(n *rawNode) []*rawNode {
	var nodes []*rawNode
	for child := range n.xsdChildren() {
		switch child.Name.Local {
		case vocab.XSDElemKey, vocab.XSDElemKeyref, vocab.XSDElemUnique:
			nodes = append(nodes, child)
		}
	}
	return nodes
}

func (c *compiler) declareAllIdentityConstraints() error {
	for _, document := range c.plan.documents {
		if !document.indexDeclarations {
			continue
		}
		doc := document.doc
		ctx := c.contexts[doc]
		if err := c.declareIdentityConstraintsInTree(doc.root, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraintsInTree(n *rawNode, ctx *schemaContext) error {
	if n.Name.Space == vocab.XSDNamespaceURI && n.Name.Local == vocab.XSDElemElement {
		if _, err := c.declareIdentityConstraints(identityConstraintNodes(n), ctx); err != nil {
			return err
		}
	}
	for _, child := range n.Children {
		if err := c.declareIdentityConstraintsInTree(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraints(nodes []*rawNode, ctx *schemaContext) ([]runtime.IdentityConstraintID, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	ids := make([]runtime.IdentityConstraintID, 0, len(nodes))
	for _, node := range nodes {
		if id, ok := c.identityDeclared[node]; ok {
			ids = append(ids, id)
			continue
		}
		id, err := c.declareIdentityConstraint(node, ctx)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *compiler) declareIdentityConstraint(node *rawNode, ctx *schemaContext) (runtime.IdentityConstraintID, error) {
	name := rawLexicalAttribute(node, vocab.XSDAttrName)
	if err := ValidateIdentityConstraintNameSource(name); err != nil {
		return runtime.NoIdentityConstraint, withSchemaCompileLocation(node, err)
	}
	q, err := c.rt.internQName(ctx.targetNS, name.Value)
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	if duplicateErr := c.checkIdentityConstraintNameAvailable(q); duplicateErr != nil {
		return runtime.NoIdentityConstraint, withSchemaCompileLocation(node, duplicateErr)
	}
	id, err := c.registerGlobalIdentity(q, runtime.NewDeclaredIdentityConstraint(q))
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	c.identityDeclared[node] = id
	return id, nil
}

func (c *compiler) compileDeclaredIdentityConstraints(nodes []*rawNode, ids []runtime.IdentityConstraintID, ctx *schemaContext) error {
	for i, node := range nodes {
		id := ids[i]
		ic, err := c.compileIdentityConstraint(node, ctx, c.rt.identityName(id))
		if err != nil {
			return err
		}
		c.completeIdentity(id, ic)
	}
	return nil
}

func (c *compiler) validateIdentityReferences() error {
	return c.validateIdentityReferencesBuild()
}

func (c *compiler) compileIdentityConstraint(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.IdentityConstraint, error) {
	empty := runtime.IdentityConstraint{Refer: runtime.NoIdentityConstraint}
	syntax, err := checkIdentityConstraintChildren(n)
	if err != nil {
		return empty, err
	}
	refer, err := c.compileIdentityRefer(n, ctx)
	if err != nil {
		return empty, err
	}
	selector := syntax.selector
	xpath, _ := selector.attr(vocab.XSDAttrXPath)
	paths, err := c.identitySelectorPaths(selector, xpath)
	if err != nil {
		return empty, err
	}
	fields, err := c.compileIdentityFields(syntax.fields)
	if err != nil {
		return empty, err
	}
	kind, kindErr := IdentityConstraintKindForLocal(n.Name.Local)
	if kindErr != nil {
		return empty, withSchemaCompileLocation(n, kindErr)
	}
	return runtime.NewIdentityConstraint(kind, name, refer, paths, fields), nil
}

func (c *compiler) compileIdentityRefer(n *rawNode, ctx *schemaContext) (runtime.IdentityConstraintID, error) {
	if n.Name.Local != vocab.XSDElemKeyref {
		return runtime.NoIdentityConstraint, nil
	}
	source := IdentityConstraintReferSource{Local: n.Name.Local, Refer: rawLexicalAttribute(n, vocab.XSDAttrRefer)}
	if err := ValidateIdentityConstraintReferSource(source); err != nil {
		return runtime.NoIdentityConstraint, withSchemaCompileLocation(n, err)
	}
	q, err := c.resolveQNameChecked(n, ctx, source.Refer.Value)
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	refer, err := c.resolveIdentityConstraintRefer(q)
	if err != nil {
		return runtime.NoIdentityConstraint, withSchemaCompileLocation(n, err)
	}
	return refer, nil
}

func (c *compiler) compileIdentityFields(nodes []*rawNode) ([]runtime.IdentityField, error) {
	fields := make([]runtime.IdentityField, 0, len(nodes))
	for _, field := range nodes {
		xpath, _ := field.attr(vocab.XSDAttrXPath)
		paths, err := c.identityFieldPaths(field, xpath)
		if err != nil {
			return nil, err
		}
		fields = append(fields, runtime.IdentityField{Paths: paths})
	}
	return fields, nil
}

type identityConstraintSyntax struct {
	selector *rawNode
	fields   []*rawNode
}

func (c *compiler) identitySelectorPaths(n *rawNode, xpath string) ([]runtime.IdentityPath, error) {
	paths, err := ParseIdentityPaths(xpath, identityXPathResolver{compiler: c, node: n})
	if err != nil {
		return nil, withSchemaCompileLocation(n, err)
	}
	return paths, nil
}

func (c *compiler) identityFieldPaths(n *rawNode, xpath string) ([]runtime.IdentityFieldPath, error) {
	paths, err := ParseIdentityFieldPaths(xpath, identityXPathResolver{compiler: c, node: n})
	if err != nil {
		return nil, withSchemaCompileLocation(n, err)
	}
	return paths, nil
}

type identityXPathResolver struct {
	compiler *compiler
	node     *rawNode
}

func (r identityXPathResolver) ResolveIdentityQName(parts QNameParts) (runtime.QName, error) {
	ns := ""
	if parts.Prefixed {
		var ok bool
		ns, ok = r.node.NS.Lookup(parts.Prefix)
		if !ok {
			return runtime.QName{}, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+parts.Prefix)
		}
	}
	return r.compiler.rt.internQName(ns, parts.Local)
}

func (r identityXPathResolver) ResolveIdentityWildcardNamespace(prefix string) (runtime.NamespaceID, error) {
	ns, ok := r.node.NS.Lookup(prefix)
	if !ok {
		return 0, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return r.compiler.rt.InternNamespace(ns)
}
