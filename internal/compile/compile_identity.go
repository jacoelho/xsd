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
	for _, document := range c.schemas.documents {
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
		name, hasName := node.attr(vocab.XSDAttrName)
		if err := ValidateIdentityConstraintNameSource(hasName && name != ""); err != nil {
			return nil, withSchemaCompileLocation(node, err)
		}
		q, err := c.names.InternQName(ctx.targetNS, name)
		if err != nil {
			return nil, err
		}
		if duplicateErr := CheckIdentityConstraintNameAvailable(c.rt.GlobalIdentities, q, c.rt.Names.Format(q)); duplicateErr != nil {
			return nil, withSchemaCompileLocation(node, duplicateErr)
		}
		id, err := c.registerGlobalIdentity(q, runtime.NewDeclaredIdentityConstraint(q))
		if err != nil {
			return nil, err
		}
		c.identityDeclared[node] = id
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *compiler) compileDeclaredIdentityConstraints(nodes []*rawNode, ids []runtime.IdentityConstraintID, ctx *schemaContext) error {
	for i, node := range nodes {
		id := ids[i]
		ic, err := c.compileIdentityConstraint(node, ctx, c.rt.Identities[id].Name)
		if err != nil {
			return err
		}
		c.completeIdentity(id, ic)
	}
	return nil
}

func (c *compiler) validateIdentityReferences() error {
	return ValidateIdentityReferences(c.rt.Identities)
}

func (c *compiler) compileIdentityConstraint(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.IdentityConstraint, error) {
	empty := runtime.IdentityConstraint{Refer: runtime.NoIdentityConstraint}
	syntax, err := checkIdentityConstraintChildren(n)
	if err != nil {
		return empty, err
	}
	refer := runtime.NoIdentityConstraint
	if n.Name.Local == vocab.XSDElemKeyref {
		referLexical, hasRefer := n.attr(vocab.XSDAttrRefer)
		if sourceErr := ValidateIdentityConstraintReferSource(n.Name.Local, hasRefer); sourceErr != nil {
			return empty, withSchemaCompileLocation(n, sourceErr)
		}
		q, resolveErr := c.resolveQNameChecked(n, ctx, referLexical)
		if resolveErr != nil {
			return empty, resolveErr
		}
		ref, referErr := ResolveIdentityConstraintRefer(c.rt.GlobalIdentities, q, c.rt.Names.Format(q))
		if referErr != nil {
			return empty, withSchemaCompileLocation(n, referErr)
		}
		refer = ref
	}
	selector := syntax.selector
	xpath, _ := selector.attr(vocab.XSDAttrXPath)
	paths, err := c.identitySelectorPaths(selector, xpath)
	if err != nil {
		return empty, err
	}
	fields := make([]runtime.IdentityField, 0, len(syntax.fields))
	for _, field := range syntax.fields {
		xpath, _ := field.attr(vocab.XSDAttrXPath)
		fieldPaths, fieldErr := c.identityFieldPaths(field, xpath)
		if fieldErr != nil {
			return empty, fieldErr
		}
		fields = append(fields, runtime.IdentityField{Paths: fieldPaths})
	}
	kind, kindErr := IdentityConstraintKindForLocal(n.Name.Local)
	if kindErr != nil {
		return empty, withSchemaCompileLocation(n, kindErr)
	}
	return runtime.NewIdentityConstraint(kind, name, refer, paths, fields), nil
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

func (r identityXPathResolver) ResolveIdentityQName(prefix, local string, prefixed bool) (runtime.QName, error) {
	ns := ""
	if prefixed {
		var ok bool
		ns, ok = r.node.NS[prefix]
		if !ok {
			return runtime.QName{}, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
		}
	}
	return r.compiler.names.InternQName(ns, local)
}

func (r identityXPathResolver) ResolveIdentityWildcardNamespace(prefix string) (runtime.NamespaceID, error) {
	ns, ok := r.node.NS[prefix]
	if !ok {
		return 0, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return r.compiler.names.InternNamespace(ns)
}
