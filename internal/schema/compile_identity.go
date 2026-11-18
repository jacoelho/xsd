package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func identityConstraintNodes(n *schemaNode) []*schemaNode {
	var nodes []*schemaNode
	for child := range n.xsdChildren() {
		switch child.local {
		case vocab.XSDElemKey, vocab.XSDElemKeyref, vocab.XSDElemUnique:
			nodes = append(nodes, child)
		}
	}
	return nodes
}

func (c *compiler) declareAllIdentityConstraints() error {
	for i := range c.plan.documents {
		ctx := &c.plan.documents[i]
		doc := ctx.doc
		if err := c.declareIdentityConstraintsInTree(ctx, doc.root); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraintsInTree(ctx *schemaSetDocument, n *schemaNode) error {
	if n.local == vocab.XSDElemElement {
		if _, err := c.declareIdentityConstraints(ctx, identityConstraintNodes(n)); err != nil {
			return err
		}
	}
	for child := range n.xsdChildren() {
		if err := c.declareIdentityConstraintsInTree(ctx, child); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraints(ctx *schemaSetDocument, nodes []*schemaNode) ([]IdentityConstraintID, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	ids := make([]IdentityConstraintID, 0, len(nodes))
	for _, node := range nodes {
		if id, ok := c.identityDeclared[nodeKey(ctx, node)]; ok {
			ids = append(ids, id)
			continue
		}
		id, err := c.declareIdentityConstraint(ctx, node)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *compiler) declareIdentityConstraint(ctx *schemaSetDocument, node *schemaNode) (IdentityConstraintID, error) {
	source := semanticSource[*schemaIdentitySource](&node.semantic)
	if source == nil {
		return NoIdentityConstraint, withSchemaCompileLocation(node, xsderrors.InternalInvariant("identity node has no typed identity source"))
	}
	q, err := c.rt.internQName(ctx.effectiveTargetNS, source.Name.Value)
	if err != nil {
		return NoIdentityConstraint, err
	}
	if duplicateErr := c.checkIdentityConstraintNameAvailable(q); duplicateErr != nil {
		return NoIdentityConstraint, withSchemaCompileLocation(node, duplicateErr)
	}
	id, err := c.registerGlobalIdentity(q, newDeclaredIdentityConstraint(q), node)
	if err != nil {
		return NoIdentityConstraint, err
	}
	c.identityDeclared[nodeKey(ctx, node)] = id
	return id, nil
}

func (c *compiler) compileDeclaredIdentityConstraints(ctx *schemaSetDocument, nodes []*schemaNode, ids []IdentityConstraintID) error {
	for i, node := range nodes {
		id := ids[i]
		ic, err := c.compileIdentityConstraint(ctx, node, c.rt.identityName(id))
		if err != nil {
			return err
		}
		c.completeIdentity(id, ic)
	}
	return nil
}

func (c *compiler) validateIdentityReferences() error {
	for id := range c.rt.Identities {
		if err := c.validateIdentityReference(IdentityConstraintID(id)); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) validateIdentityReference(id IdentityConstraintID) (err error) {
	scope, err := c.enterComponent(c.identitySources[id])
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	return validateKeyrefReference(c.rt.Identities, c.rt.Identities[id])
}

// compileIdentityConstraint parses the selector and field XPaths of n again
// to intern their name tests into the compiler's name table. Their syntax and
// prefixes were checked on each start tag (admitIdentityXPath), so a parse
// failure here is an invariant.
func (c *compiler) compileIdentityConstraint(ctx *schemaSetDocument, n *schemaNode, name QName) (identityConstraintSpec, error) {
	empty := identityConstraintSpec{Refer: NoIdentityConstraint}
	syntax, err := identityConstraintChildren(n)
	if err != nil {
		return empty, err
	}
	refer, err := c.compileIdentityRefer(ctx, n)
	if err != nil {
		return empty, err
	}
	selector := syntax.selector
	selectorSource := semanticSource[*schemaIdentityXPathSource](&selector.semantic)
	if selectorSource == nil {
		return empty, withSchemaCompileLocation(selector, xsderrors.InternalInvariant("identity selector has no typed XPath source"))
	}
	xpath := selectorSource.XPath.Value
	paths, err := c.identitySelectorPaths(selector, xpath)
	if err != nil {
		return empty, err
	}
	fields, err := c.compileIdentityFields(syntax.fields)
	if err != nil {
		return empty, err
	}
	kind, kindErr := identityConstraintKindForLocal(n.local)
	if kindErr != nil {
		return empty, withSchemaCompileLocation(n, kindErr)
	}
	return newIdentityConstraint(kind, name, refer, paths, fields), nil
}

func (c *compiler) compileIdentityRefer(ctx *schemaSetDocument, n *schemaNode) (IdentityConstraintID, error) {
	if n.local != vocab.XSDElemKeyref {
		return NoIdentityConstraint, nil
	}
	typed := semanticSource[*schemaIdentitySource](&n.semantic)
	if typed == nil {
		return NoIdentityConstraint, withSchemaCompileLocation(n, xsderrors.InternalInvariant("identity node has no typed identity source"))
	}
	q, err := c.referenceQName(ctx, n, typed.Refer.Name)
	if err != nil {
		return NoIdentityConstraint, err
	}
	refer, err := c.resolveIdentityConstraintRefer(q)
	if err != nil {
		return NoIdentityConstraint, withSchemaCompileLocation(n, err)
	}
	return refer, nil
}

func (c *compiler) compileIdentityFields(nodes []*schemaNode) ([]identityField, error) {
	fields := make([]identityField, 0, len(nodes))
	for _, field := range nodes {
		source := semanticSource[*schemaIdentityXPathSource](&field.semantic)
		if source == nil {
			return nil, withSchemaCompileLocation(field, xsderrors.InternalInvariant("identity field has no typed XPath source"))
		}
		xpath := source.XPath.Value
		paths, err := c.identityFieldPaths(field, xpath)
		if err != nil {
			return nil, err
		}
		fields = append(fields, identityField{Paths: paths})
	}
	return fields, nil
}

type identityConstraintSyntax struct {
	selector *schemaNode
	fields   []*schemaNode
}

func (c *compiler) identitySelectorPaths(n *schemaNode, xpath string) ([]identityPath, error) {
	resolver := &identityXPathResolver{compiler: c, node: n}
	paths, err := parseIdentityPaths(xpath, resolver)
	return paths, resolver.compileError(err)
}

func (c *compiler) identityFieldPaths(n *schemaNode, xpath string) ([]identityFieldPath, error) {
	resolver := &identityXPathResolver{compiler: c, node: n}
	paths, err := parseIdentityFieldPaths(xpath, resolver)
	return paths, resolver.compileError(err)
}

// identityXPathResolver resolves the name tests of an admitted XPath and
// interns them. internErr keeps an interning failure, which is a compile
// diagnostic, apart from a parse failure, which is an invariant.
type identityXPathResolver struct {
	compiler  *compiler
	node      *schemaNode
	internErr error
}

func (r *identityXPathResolver) compileError(err error) error {
	if err == nil {
		return nil
	}
	if r.internErr != nil {
		return withSchemaCompileLocation(r.node, r.internErr)
	}
	return withSchemaCompileLocation(r.node, admittedValueInvariant(err))
}

func (r *identityXPathResolver) ResolveIdentityQName(parts qNameParts) (QName, error) {
	ns := ""
	if parts.Prefixed {
		var ok bool
		ns, ok = r.node.namespace.Lookup(parts.Prefix)
		if !ok {
			return QName{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(parts.Prefix))
		}
	}
	q, err := r.compiler.rt.internQName(ns, parts.Local)
	r.internErr = err
	return q, err
}

func (r *identityXPathResolver) ResolveIdentityWildcardNamespace(prefix string) (namespaceID, error) {
	ns, ok := r.node.namespace.Lookup(prefix)
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	id, err := newNameInterner(&r.compiler.rt.Names).InternNamespace(ns)
	r.internErr = err
	return id, err
}
