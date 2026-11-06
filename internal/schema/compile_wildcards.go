package schema

import "github.com/jacoelho/xsd/xsderrors"

func (c *compiler) compileWildcardParticle(n *schemaNode, ctx *schemaSetDocument) (contentParticle, error) {
	if err := checkAnyParticleChildren(n); err != nil {
		return contentParticle{}, err
	}
	id, err := c.compileWildcard(n, ctx)
	if err != nil {
		return contentParticle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return contentParticle{}, err
	}
	return wildcardParticle(id, occurs), nil
}

func (c *compiler) compileAttributeWildcard(n *schemaNode, ctx *schemaSetDocument) (WildcardID, error) {
	if err := checkAnyAttributeChildren(n); err != nil {
		return NoWildcard, err
	}
	return c.compileWildcard(n, ctx)
}

func (c *compiler) compileWildcard(n *schemaNode, ctx *schemaSetDocument) (WildcardID, error) {
	source := semanticSource[*schemaWildcardSource](&n.semantic)
	if source == nil {
		return NoWildcard, withSchemaCompileLocation(n, xsderrors.InternalInvariant("wildcard node has no typed wildcard source"))
	}
	ns, hasNS := source.Namespace.Value, source.Namespace.Present
	process, hasProcess := source.ProcessContents.Value, source.ProcessContents.Present
	w, err := parseWildcard(newNameInterner(&c.rt.Names), wildcardAttrs{
		Namespace:          ns,
		ProcessContents:    process,
		TargetNamespace:    ctx.effectiveTargetNS,
		HasNamespace:       hasNS,
		HasProcessContents: hasProcess,
	})
	if err != nil {
		return NoWildcard, withSchemaCompileLocation(n, err)
	}
	return c.appendWildcard(w)
}

// wildcardRuntime returns compiler-owned wildcard metadata for internal compile
// helpers.
func (c *compiler) Wildcard(id WildcardID) (wildcardRuntime, bool) {
	return wildcardByID(c.rt.Wildcards, id)
}

// AddWildcard stores wildcard metadata produced by internal compile helpers.
func (c *compiler) AddWildcard(w wildcardRuntime) (WildcardID, error) {
	return c.appendWildcard(w)
}
