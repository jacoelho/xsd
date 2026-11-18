package schema

import (
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

// declareAllIdentityConstraints declares every identity constraint of every
// plan document context, in plan order, before any component compiles.
// Identity-constraint names are one schema-wide symbol space (Structures
// §3.11.1), so every name is declared before a keyref refer resolves against
// them, wherever the keyref and the referenced constraint are written. A
// constraint is declared once per document context: a group referenced twice
// shares its constraints, and a chameleon document declares its constraints
// once in each including namespace.
func (c *compiler) declareAllIdentityConstraints() error {
	for i := range c.plan.documents {
		ctx := &c.plan.documents[i]
		if err := c.declareIdentityConstraintsInTree(ctx, ctx.doc.root); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraintsInTree(ctx *schemaSetDocument, n *schemaNode) error {
	for child := range n.xsdChildren() {
		switch child.local {
		case vocab.XSDElemKey, vocab.XSDElemKeyref, vocab.XSDElemUnique:
			if err := c.declareIdentityConstraint(ctx, child); err != nil {
				return err
			}
		default:
			if err := c.declareIdentityConstraintsInTree(ctx, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraint(ctx *schemaSetDocument, node *schemaNode) error {
	source := semanticSource[*schemaIdentitySource](&node.semantic)
	if source == nil {
		return withSchemaCompileLocation(node, xsderrors.InternalInvariant("identity node has no typed identity source"))
	}
	q, err := c.rt.internQName(ctx.effectiveTargetNS, source.Name.Value)
	if err != nil {
		return withSchemaCompileLocation(node, err)
	}
	if duplicateErr := c.checkIdentityConstraintNameAvailable(q); duplicateErr != nil {
		return withSchemaCompileLocation(node, duplicateErr)
	}
	id, err := c.registerGlobalIdentity(q, newDeclaredIdentityConstraint(q), node)
	if err != nil {
		return withSchemaCompileLocation(node, err)
	}
	c.identityDeclared[nodeKey(ctx, node)] = id
	return nil
}

// declaredIdentityConstraints returns the IDs declareAllIdentityConstraints
// gave nodes in ctx.
func (c *compiler) declaredIdentityConstraints(ctx *schemaSetDocument, nodes []*schemaNode) ([]IdentityConstraintID, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	ids := make([]IdentityConstraintID, 0, len(nodes))
	for _, node := range nodes {
		id, ok := c.identityDeclared[nodeKey(ctx, node)]
		if !ok {
			return nil, withSchemaCompileLocation(node, xsderrors.InternalInvariant("identity constraint was not declared before compilation"))
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// identityOwnership states whether an element declaration owns the identity
// constraints being compiled.
type identityOwnership uint8

const (
	// identityOwned constraints belong to the element declaration that
	// declares them.
	identityOwned identityOwnership = iota
	// identityDetached constraints are declared under an xs:element with
	// minOccurs and maxOccurs 0, which maps to no declaration.
	identityDetached
)

func (c *compiler) compileDeclaredIdentityConstraints(ctx *schemaSetDocument, nodes []*schemaNode, ids []IdentityConstraintID, ownership identityOwnership) error {
	for i, node := range nodes {
		id := ids[i]
		ic, err := c.compileIdentityConstraint(ctx, node, c.rt.identityName(id))
		if err != nil {
			return err
		}
		ic.Detached = ownership == identityDetached
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

// compileIdentityConstraint builds the identity constraint n from the paths
// its selector and fields stored at their start tags (admitIdentityXPath).
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
	selector := semanticSource[*schemaIdentitySelectorSource](&syntax.selector.semantic)
	if selector == nil || len(selector.Paths) == 0 {
		return empty, withSchemaCompileLocation(syntax.selector, xsderrors.InternalInvariant("identity selector has no admitted paths"))
	}
	fields, err := compileIdentityFields(syntax.fields)
	if err != nil {
		return empty, err
	}
	kind, kindErr := identityConstraintKindForLocal(n.local)
	if kindErr != nil {
		return empty, withSchemaCompileLocation(n, kindErr)
	}
	return newIdentityConstraint(kind, name, refer, selector.Paths, fields), nil
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

func compileIdentityFields(nodes []*schemaNode) ([]identityField, error) {
	fields := make([]identityField, 0, len(nodes))
	for _, field := range nodes {
		source := semanticSource[*schemaIdentityFieldSource](&field.semantic)
		if source == nil || len(source.Paths) == 0 {
			return nil, withSchemaCompileLocation(field, xsderrors.InternalInvariant("identity field has no admitted paths"))
		}
		fields = append(fields, identityField{Paths: source.Paths})
	}
	return fields, nil
}

type identityConstraintSyntax struct {
	selector *schemaNode
	fields   []*schemaNode
}
