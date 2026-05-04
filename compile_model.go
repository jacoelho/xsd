package xsd

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func modelChildren(n *rawNode) []*rawNode {
	var out []*rawNode
	for _, c := range n.xsContentChildren() {
		switch c.Name.Local {
		case "sequence", "choice", "all", "group":
			out = append(out, c)
		}
	}
	return out
}

func (c *compiler) addModel(m contentModel) contentModelID {
	m.Replay = c.modelNeedsReplay(m)
	id := contentModelID(len(c.rt.Models))
	c.rt.Models = append(c.rt.Models, m)
	return id
}

func (c *compiler) modelNeedsReplay(m contentModel) bool {
	if c.choiceNeedsReplay(m, m.occurs) {
		return true
	}
	for _, p := range m.Particles {
		if p.Kind != particleModel {
			continue
		}
		child := c.rt.Models[p.Model]
		if c.choiceNeedsReplay(child, p.occurs) || child.Replay {
			return true
		}
	}
	return false
}

func (c *compiler) choiceNeedsReplay(m contentModel, occurs occurrence) bool {
	if m.Kind != modelChoice || occurs.isExactlyOne() {
		return false
	}
	for _, p := range m.Particles {
		if !p.occurs.isExactlyOne() {
			return true
		}
	}
	return false
}

func (c *compiler) extendSequence(baseID, extID contentModelID) contentModelID {
	if baseID == noContentModel {
		return extID
	}
	base := c.rt.Models[baseID]
	ext := c.rt.Models[extID]
	if base.Kind == modelEmpty {
		return extID
	}
	if ext.Kind == modelEmpty {
		return baseID
	}
	if base.Kind == modelAll && len(base.Particles) == 0 {
		return extID
	}
	m := contentModel{Kind: modelSequence, occurs: occurrence{Min: 1, Max: 1}, Mixed: base.Mixed || ext.Mixed}
	if base.Kind == modelSequence && base.occurs.isExactlyOne() {
		m.Particles = append(m.Particles, base.Particles...)
	} else {
		c.appendModelParticle(&m, baseID)
	}
	if ext.Kind == modelSequence && ext.occurs.isExactlyOne() {
		m.Particles = append(m.Particles, ext.Particles...)
	} else {
		c.appendModelParticle(&m, extID)
	}
	return c.addModel(m)
}

func (c *compiler) modelHasNoParticles(modelID contentModelID) bool {
	if modelID == noContentModel {
		return true
	}
	model := c.rt.Models[modelID]
	switch model.Kind {
	case modelEmpty:
		return true
	case modelSequence, modelChoice, modelAll:
		return len(model.Particles) == 0
	default:
		return false
	}
}

func (c *compiler) appendModelParticle(m *contentModel, id contentModelID) {
	p, ok := c.modelParticle(id)
	if !ok {
		return
	}
	m.Particles = append(m.Particles, p)
}

func (c *compiler) modelParticle(id contentModelID) (particle, bool) {
	model := c.rt.Models[id]
	occurs := model.occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return particle{}, false
	}
	modelID := id
	if !occurs.isExactlyOne() {
		normalized := model
		normalized.occurs = occurrence{Min: 1, Max: 1}
		normalized.SkipUPA = model.SkipUPA || c.sequenceHasWildcardEquivalentOverlap(normalized)
		modelID = c.addModel(normalized)
		c.rt.Models[modelID].Replay = model.Replay || c.choiceNeedsReplay(normalized, occurs)
		c.rt.Models[modelID].SkipUPA = normalized.SkipUPA
	}
	return particle{Kind: particleModel, occurs: occurs, Model: modelID}, true
}

func (c *compiler) compileModel(n *rawNode, ctx *schemaContext) (contentModelID, error) {
	if n.Name.Local == "group" {
		if ref, ok := n.attr("ref"); ok {
			return c.compileModelGroupRef(n, ctx, ref)
		}
	}
	if id, ok := c.modelDone[n]; ok {
		if c.compilingModel[n] {
			if c.elementDepth > c.modelDepth[n] {
				return id, nil
			}
			return noContentModel, schemaCompile(ErrSchemaReference, "recursive model group")
		}
		return id, nil
	}
	if c.compilingModel[n] {
		return noContentModel, schemaCompile(ErrSchemaReference, "recursive model group")
	}
	id := c.addModel(contentModel{})
	c.modelDone[n] = id
	c.modelDepth[n] = c.elementDepth
	c.compilingModel[n] = true
	defer delete(c.compilingModel, n)
	kind, err := modelKindForNode(n)
	if err != nil {
		return noContentModel, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return noContentModel, err
	}
	if kind == modelAll && (occurs.Unbounded || occurs.Max > 1 || occurs.Min > 1) {
		return noContentModel, schemaCompile(ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	m := contentModel{Kind: kind, occurs: occurs}
	if err := c.compileModelChildren(n, ctx, &m); err != nil {
		return noContentModel, err
	}
	m.Replay = c.modelNeedsReplay(m)
	if !m.Replay {
		if err := c.checkDirectUPA(m); err != nil {
			return noContentModel, err
		}
	}
	if err := c.checkElementDeclarationsConsistent(m); err != nil {
		return noContentModel, err
	}
	c.rt.Models[id] = m
	return id, nil
}

func (c *compiler) compileModelGroupRef(n *rawNode, ctx *schemaContext, ref string) (contentModelID, error) {
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return noContentModel, err
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return noContentModel, err
	}
	raw, ok := c.groupRaw[q]
	if !ok {
		return noContentModel, schemaCompile(ErrSchemaReference, "unknown model group "+c.rt.Names.Format(q))
	}
	modelNode := firstModelChild(raw.node)
	if modelNode == nil {
		return noContentModel, schemaCompile(ErrSchemaContentModel, "model group has no content "+c.rt.Names.Format(q))
	}
	if id, ok := c.modelDone[modelNode]; ok && c.compilingModel[modelNode] {
		return c.recursiveModelGroupRef(q, id, occurs, modelNode)
	}
	id, err := c.compileModel(modelNode, raw.ctx)
	if err != nil {
		return noContentModel, err
	}
	if occurs.isExactlyOne() {
		return id, nil
	}
	model := c.rt.Models[id]
	if model.Kind == modelAll && (occurs.Unbounded || occurs.Max > 1 || occurs.Min > 1) {
		return noContentModel, schemaCompile(ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	model.occurs = occurs
	return c.addModel(model), nil
}

func (c *compiler) recursiveModelGroupRef(q qName, id contentModelID, occurs occurrence, modelNode *rawNode) (contentModelID, error) {
	if c.elementDepth <= c.modelDepth[modelNode] {
		return noContentModel, schemaCompile(ErrSchemaReference, "recursive model group "+c.rt.Names.Format(q))
	}
	ref := contentModel{
		Kind:   modelSequence,
		occurs: occurs,
		Particles: []particle{{
			Kind:   particleModel,
			occurs: occurrence{Min: 1, Max: 1},
			Model:  id,
		}},
	}
	return c.addModel(ref), nil
}

func modelKindForNode(n *rawNode) (modelKind, error) {
	switch n.Name.Local {
	case "sequence":
		return modelSequence, nil
	case "choice":
		return modelChoice, nil
	case "all":
		return modelAll, nil
	default:
		return 0, schemaCompile(ErrSchemaContentModel, "unsupported model "+n.Name.Local)
	}
}

func (c *compiler) compileModelChildren(n *rawNode, ctx *schemaContext, m *contentModel) error {
	for _, child := range n.xsContentChildren() {
		if err := c.appendModelChild(m, child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) appendModelChild(m *contentModel, child *rawNode, ctx *schemaContext) error {
	switch child.Name.Local {
	case "element":
		p, err := c.compileElementParticle(child, ctx)
		return appendParticle(m, p, err)
	case "any":
		p, err := c.compileWildcardParticle(child, ctx)
		return appendParticle(m, p, err)
	case "sequence", "choice", "all", "group":
		return c.appendNestedModelChild(m, child, ctx)
	default:
		return nil
	}
}

func (c *compiler) appendNestedModelChild(m *contentModel, child *rawNode, ctx *schemaContext) error {
	if child.Name.Local == "all" {
		return schemaCompile(ErrSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	childModelID, err := c.compileModel(child, ctx)
	if err != nil {
		return err
	}
	childModel := c.rt.Models[childModelID]
	if child.Name.Local == "group" && childModel.Kind == modelAll {
		return schemaCompile(ErrSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	if appendFlattenedModelChild(m, childModel) {
		return nil
	}
	p, ok := c.modelParticle(childModelID)
	if !ok {
		return nil
	}
	return appendParticle(m, p, nil)
}

func appendFlattenedModelChild(m *contentModel, child contentModel) bool {
	if m.Kind == modelChoice && child.Kind == modelChoice && child.occurs.isExactlyOne() {
		m.Particles = append(m.Particles, child.Particles...)
		return true
	}
	if m.Kind != modelSequence {
		return false
	}
	if (child.Kind == modelSequence || child.Kind == modelChoice) && len(child.Particles) == 1 {
		p := child.Particles[0]
		p.occurs = multiplyOccurs(p.occurs, child.occurs)
		m.Particles = append(m.Particles, p)
		return true
	}
	if child.Kind == modelSequence && len(child.Particles) > 1 && child.occurs.isExactlyOne() {
		m.Particles = append(m.Particles, child.Particles...)
		return true
	}
	return false
}

func appendParticle(m *contentModel, p particle, err error) error {
	if err != nil {
		return err
	}
	if p.occurs.Max == 0 && !p.occurs.Unbounded {
		return nil
	}
	if m.Kind == modelAll && (p.occurs.Unbounded || p.occurs.Max > 1) {
		return schemaCompile(ErrSchemaOccurrence, "xs:all particles cannot repeat")
	}
	m.Particles = append(m.Particles, p)
	return nil
}

func validateModelOccurrence(n *rawNode, limits compileLimits) error {
	allowed := map[string]bool{
		"id": true, "minOccurs": true, "maxOccurs": true,
	}
	if n.Name.Local == "group" {
		allowed["ref"] = true
	}
	if err := validateKnownAttributes(n, n.Name.Local, allowed); err != nil {
		return err
	}
	occurs, err := parseOccurs(n, limits)
	if err != nil {
		return err
	}
	if n.Name.Local == "all" && (occurs.Unbounded || occurs.Max != 1 || occurs.Min > 1) {
		return schemaCompile(ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	return validateModelGroupSyntax(n, limits)
}

func (c *compiler) checkDirectUPA(m contentModel) error {
	switch m.Kind {
	case modelChoice, modelAll:
		for i, p := range m.Particles {
			for j := i + 1; j < len(m.Particles); j++ {
				if name, ok := c.particlesOverlap(p, m.Particles[j]); ok {
					msg := "UPA violation: overlapping particles in choice"
					if name.Local != 0 || name.Namespace != 0 {
						msg += " " + c.rt.Names.Format(name)
					}
					return schemaCompile(ErrSchemaContentModel, msg)
				}
			}
		}
	case modelSequence:
		for i, p := range m.Particles {
			for _, candidate := range c.particleContinuationParticles(p) {
				for j := i + 1; j < len(m.Particles); j++ {
					if name, ok := c.particlesOverlap(candidate, m.Particles[j]); ok {
						if !m.occurs.isExactlyOne() && c.wildcardEquivalentOverlap(candidate, m.Particles[j]) {
							continue
						}
						return schemaCompile(ErrSchemaContentModel, "UPA violation: duplicate element in sequence "+c.rt.Names.Format(name))
					}
					if m.Particles[j].occurs.Min > 0 {
						break
					}
				}
			}
		}
	}
	return nil
}

func (c *compiler) wildcardEquivalentOverlap(a, b particle) bool {
	if a.Kind != particleWildcard || b.Kind != particleWildcard {
		return false
	}
	wa := c.rt.Wildcards[a.wildcard]
	wb := c.rt.Wildcards[b.wildcard]
	return wildcardNamespaceEqual(wa, wb)
}

func wildcardNamespaceEqual(a, b wildcard) bool {
	if a.Mode != b.Mode || a.OtherThan != b.OtherThan || len(a.Namespaces) != len(b.Namespaces) {
		return false
	}
	for i := range a.Namespaces {
		if a.Namespaces[i] != b.Namespaces[i] {
			return false
		}
	}
	return true
}

func (c *compiler) particleCanOverlapFollowing(p particle) bool {
	r := c.particleCountRange(p)
	if r.Unbounded {
		return true
	}
	return r.Max > r.Min
}

func (c *compiler) particleContinuationParticles(p particle) []particle {
	if p.occurs.Max == 0 && !p.occurs.Unbounded {
		return nil
	}
	switch p.Kind {
	case particleElement, particleWildcard:
		if c.particleCanOverlapFollowing(p) {
			return []particle{p}
		}
	case particleModel:
		model := c.rt.Models[p.Model]
		var out []particle
		if p.occurs.Unbounded || p.occurs.Max > p.occurs.Min {
			out = append(out, c.modelStartParticles(model)...)
		}
		out = append(out, c.modelContinuationParticles(model)...)
		return out
	}
	return nil
}

func (c *compiler) modelContinuationParticles(model contentModel) []particle {
	var out []particle
	if model.occurs.Unbounded || model.occurs.Max > model.occurs.Min {
		out = append(out, c.modelStartParticles(model)...)
	}
	switch model.Kind {
	case modelSequence:
		for _, p := range model.Particles {
			out = append(out, c.particleContinuationParticles(p)...)
		}
	case modelChoice:
		for _, p := range model.Particles {
			out = append(out, c.particleContinuationParticles(p)...)
		}
	}
	return out
}

func (c *compiler) checkCompiledModelsUPA() error {
	for _, model := range c.rt.Models {
		if model.Replay || model.SkipUPA {
			continue
		}
		if err := c.checkDirectUPA(model); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) sequenceHasWildcardEquivalentOverlap(m contentModel) bool {
	if m.Kind != modelSequence {
		return false
	}
	for i, p := range m.Particles {
		for _, candidate := range c.particleContinuationParticles(p) {
			for j := i + 1; j < len(m.Particles); j++ {
				if c.wildcardEquivalentOverlap(candidate, m.Particles[j]) {
					return true
				}
				if m.Particles[j].occurs.Min > 0 {
					break
				}
			}
		}
	}
	return false
}

func (c *compiler) particlesOverlap(a, b particle) (qName, bool) {
	if a.Kind == particleModel {
		for _, p := range c.modelStartParticles(c.rt.Models[a.Model]) {
			if name, ok := c.particlesOverlap(p, b); ok {
				return name, true
			}
		}
		return qName{}, false
	}
	if b.Kind == particleModel {
		for _, p := range c.modelStartParticles(c.rt.Models[b.Model]) {
			if name, ok := c.particlesOverlap(a, p); ok {
				return name, true
			}
		}
		return qName{}, false
	}
	if a.Kind == particleWildcard && b.Kind == particleWildcard {
		return qName{}, wildcardsOverlap(c.rt.Wildcards[a.wildcard], c.rt.Wildcards[b.wildcard])
	}
	for _, name := range c.particleElementNames(a) {
		if c.particleMatchesName(b, name) {
			return name, true
		}
	}
	for _, name := range c.particleElementNames(b) {
		if c.particleMatchesName(a, name) {
			return name, true
		}
	}
	return qName{}, false
}

func wildcardsOverlap(a, b wildcard) bool {
	if a.Mode == wildAny || b.Mode == wildAny {
		return true
	}
	if a.Mode == wildOther && b.Mode == wildOther {
		return true
	}
	if a.Mode == wildOther {
		return wildcardHasNamespaceOtherThan(b, a.OtherThan)
	}
	if b.Mode == wildOther {
		return wildcardHasNamespaceOtherThan(a, b.OtherThan)
	}
	if a.Mode == wildLocal && b.Mode == wildLocal {
		return true
	}
	if a.Mode == wildLocal {
		return wildcardAllowsNamespace(b, namespaceID(0))
	}
	if b.Mode == wildLocal {
		return wildcardAllowsNamespace(a, namespaceID(0))
	}
	for _, ns := range wildcardNamespaces(a) {
		if wildcardAllowsNamespace(b, ns) {
			return true
		}
	}
	return false
}

func wildcardHasNamespaceOtherThan(w wildcard, excluded namespaceID) bool {
	switch w.Mode {
	case wildAny:
		return true
	case wildOther:
		return true
	case wildLocal:
		return false
	case wildTargetNamespace, wildList:
		for _, ns := range wildcardNamespaces(w) {
			if ns != excluded {
				return true
			}
		}
	}
	return false
}

func wildcardAllowsNamespace(w wildcard, ns namespaceID) bool {
	switch w.Mode {
	case wildAny:
		return true
	case wildOther:
		return ns != namespaceID(0) && ns != w.OtherThan
	case wildLocal:
		return ns == namespaceID(0)
	case wildTargetNamespace, wildList:
		if slices.Contains(wildcardNamespaces(w), ns) {
			return true
		}
	}
	return false
}

func wildcardNamespaces(w wildcard) []namespaceID {
	if w.Mode == wildTargetNamespace || w.Mode == wildList {
		return w.Namespaces
	}
	return nil
}

func (c *compiler) particleMatchesName(p particle, name qName) bool {
	switch p.Kind {
	case particleElement:
		if slices.Contains(c.particleElementNames(p), name) {
			return true
		}
	case particleWildcard:
		w := c.rt.Wildcards[p.wildcard]
		return wildcardMatches(&c.rt, w, runtimeName{
			Name:  name,
			Known: true,
			NS:    c.rt.Names.Namespace(name.Namespace),
			Local: c.rt.Names.Local(name.Local),
		})
	case particleModel:
		model := c.rt.Models[p.Model]
		if slices.Contains(c.modelStartElementNames(model), name) {
			return true
		}
	}
	return false
}

func (c *compiler) checkElementDeclarationsConsistent(m contentModel) error {
	types := make(map[qName]typeID)
	for _, p := range m.Particles {
		if err := c.collectElementDeclarationType(types, p); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) collectElementDeclarationType(types map[qName]typeID, p particle) error {
	switch p.Kind {
	case particleElement:
		decl := c.rt.Elements[p.Element]
		if typ, ok := types[decl.Name]; ok && typ != decl.Type {
			return schemaCompile(ErrSchemaContentModel, "element declarations with the same name must have the same type")
		}
		types[decl.Name] = decl.Type
	case particleModel:
		model := c.rt.Models[p.Model]
		for _, child := range model.Particles {
			if err := c.collectElementDeclarationType(types, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) particleElementNames(p particle) []qName {
	switch p.Kind {
	case particleElement:
		names := []qName{c.rt.Elements[p.Element].Name}
		for _, member := range c.rt.Substitutions[p.Element] {
			if c.substitutionAllowed(p.Element, member) {
				names = append(names, c.rt.Elements[member].Name)
			}
		}
		return names
	case particleModel:
		model := c.rt.Models[p.Model]
		return c.modelStartElementNames(model)
	default:
		return nil
	}
}

func (c *compiler) modelStartElementNames(model contentModel) []qName {
	var names []qName
	switch model.Kind {
	case modelAll, modelChoice:
		for _, p := range model.Particles {
			names = append(names, c.particleElementNames(p)...)
		}
	case modelSequence:
		for _, p := range model.Particles {
			names = append(names, c.particleElementNames(p)...)
			if !c.particleEmptiable(p) {
				break
			}
		}
	}
	return names
}

func (c *compiler) modelStartParticles(model contentModel) []particle {
	var out []particle
	switch model.Kind {
	case modelAll, modelChoice:
		out = append(out, model.Particles...)
	case modelSequence:
		for _, p := range model.Particles {
			out = append(out, p)
			if !c.particleEmptiable(p) {
				break
			}
		}
	}
	return out
}

func (c *compiler) substitutionAllowed(headID, memberID elementID) bool {
	head := c.rt.Elements[headID]
	member := c.rt.Elements[memberID]
	if head.Block&blockSubstitution != 0 {
		return false
	}
	return c.rt.substitutionDerivationAllowed(member.Type, head.Type, head.Block)
}

func parseOccurs(n *rawNode, limits compileLimits) (occurrence, error) {
	minOccurs := uint32(1)
	minDigits := "1"
	if v, ok := n.attr("minOccurs"); ok {
		digits, err := parseOccurrenceDigits(v)
		if err != nil {
			return occurrence{}, schemaCompile(ErrSchemaOccurrence, "invalid minOccurs "+v)
		}
		minDigits = digits
		minOccurs = occurrenceUint32(digits)
	}
	maxOccurs := uint32(1)
	maxDigits := "1"
	if v, ok := n.attr("maxOccurs"); ok {
		if strings.TrimSpace(v) == "unbounded" {
			return occurrence{Min: minOccurs, Unbounded: true}, nil
		}
		digits, err := parseOccurrenceDigits(v)
		if err != nil {
			return occurrence{}, schemaCompile(ErrSchemaOccurrence, "invalid maxOccurs "+v)
		}
		if maxOccursLimitExceeded(digits, limits.maxFiniteOccurs) {
			return occurrence{}, schemaCompile(ErrSchemaLimit, "maxOccurs exceeds configured limit")
		}
		maxDigits = digits
		maxOccurs = occurrenceUint32(digits)
	}
	if compareDecimalDigits(maxDigits, minDigits) < 0 {
		return occurrence{}, schemaCompile(ErrSchemaOccurrence, "maxOccurs is less than minOccurs")
	}
	return occurrence{Min: minOccurs, Max: maxOccurs}, nil
}

func maxOccursLimitExceeded(digits string, limit uint64) bool {
	if limit == 0 {
		return false
	}
	return compareDecimalDigits(digits, strconv.FormatUint(limit, 10)) > 0
}

func parseOccurrenceDigits(v string) (string, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "+")
	if v == "" {
		return "", fmt.Errorf("empty occurrence")
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("invalid occurrence")
		}
	}
	v = strings.TrimLeft(v, "0")
	if v == "" {
		return "0", nil
	}
	return v, nil
}

func occurrenceUint32(digits string) uint32 {
	const maxUint32 = "4294967295"
	if compareDecimalDigits(digits, maxUint32) > 0 {
		return ^uint32(0)
	}
	v, _ := strconv.ParseUint(digits, 10, 32)
	return uint32(v)
}

func compareDecimalDigits(a, b string) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}
	return cmp.Compare(a, b)
}
