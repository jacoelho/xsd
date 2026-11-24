package schema

func (c *compiler) compileWildcardParticle(ctx *schemaSetDocument, n *schemaNode) (contentParticle, error) {
	id, err := c.compileWildcard(ctx, n)
	if err != nil {
		return contentParticle{}, err
	}
	occurs, err := particleOccurrence(n)
	if err != nil {
		return contentParticle{}, err
	}
	return wildcardParticle(id, occurs), nil
}

func (c *compiler) compileAttributeWildcard(ctx *schemaSetDocument, n *schemaNode) (WildcardID, error) {
	return c.compileWildcard(ctx, n)
}

func (c *compiler) compileWildcard(ctx *schemaSetDocument, n *schemaNode) (WildcardID, error) {
	source, err := requiredSemanticSource[*schemaWildcardSource](n)
	if err != nil {
		return NoWildcard, err
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

// SpendWildcardWork charges wildcard comparison work to
// MaxSchemaDependencySteps.
func (c *compiler) SpendWildcardWork(steps int) error {
	return c.dependencyWork.spend(steps)
}
