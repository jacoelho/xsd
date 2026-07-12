package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) compileWildcardParticle(n *rawNode, ctx *schemaContext) (runtime.Particle, error) {
	if err := checkAnyParticleChildren(n); err != nil {
		return runtime.Particle{}, err
	}
	id, err := c.compileWildcard(n, ctx)
	if err != nil {
		return runtime.Particle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.Particle{}, err
	}
	return runtime.WildcardParticle(id, occurs), nil
}

func (c *compiler) compileAttributeWildcard(n *rawNode, ctx *schemaContext) (runtime.WildcardID, error) {
	if err := checkAnyAttributeChildren(n); err != nil {
		return runtime.NoWildcard, err
	}
	return c.compileWildcard(n, ctx)
}

func (c *compiler) compileWildcard(n *rawNode, ctx *schemaContext) (runtime.WildcardID, error) {
	ns, hasNS := n.attr(vocab.XSDAttrNamespace)
	process, hasProcess := n.attr(vocab.XSDAttrProcessContents)
	w, err := ParseWildcard(c.names, WildcardAttrs{
		Namespace:          ns,
		ProcessContents:    process,
		TargetNamespace:    ctx.targetNS,
		HasNamespace:       hasNS,
		HasProcessContents: hasProcess,
	})
	if err != nil {
		return runtime.NoWildcard, withSchemaCompileLocation(n, err)
	}
	return c.addWildcard(w)
}

// Wildcard returns compiler-owned wildcard metadata for internal compile
// helpers.
func (c *compiler) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	if !runtime.ValidWildcardID(id, len(c.rt.Wildcards)) {
		return runtime.Wildcard{}, false
	}
	return c.rt.Wildcards[id], true
}

// AddWildcard stores wildcard metadata produced by internal compile helpers.
func (c *compiler) AddWildcard(w runtime.Wildcard) (runtime.WildcardID, error) {
	return c.addWildcard(w)
}
