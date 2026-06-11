package xsd

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *compiler) addModel(m contentModel) (contentModelID, error) {
	id, err := nextContentModelID(len(c.rt.Models))
	if err != nil {
		return noContentModel, err
	}
	c.rt.Models = append(c.rt.Models, m)
	return id, nil
}

func (c *compiler) extendSequence(baseID, extID contentModelID) (contentModelID, error) {
	if baseID == noContentModel {
		return extID, nil
	}
	base := c.rt.Models[baseID]
	ext := c.rt.Models[extID]
	mixed := base.Mixed || ext.Mixed
	if c.modelHasNoParticles(baseID) {
		return c.modelWithMixed(extID, mixed)
	}
	if c.modelHasNoParticles(extID) {
		return c.modelWithMixed(baseID, mixed)
	}
	m := contentModel{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}, Mixed: mixed}
	if base.Kind == modelSequence && base.Occurs.isExactlyOne() {
		m.Particles = append(m.Particles, base.Particles...)
	} else if err := c.appendModelParticle(&m, baseID); err != nil {
		return noContentModel, err
	}
	if ext.Kind == modelSequence && ext.Occurs.isExactlyOne() {
		m.Particles = append(m.Particles, ext.Particles...)
	} else if err := c.appendModelParticle(&m, extID); err != nil {
		return noContentModel, err
	}
	return c.addModel(m)
}

func (c *compiler) modelWithMixed(id contentModelID, mixed bool) (contentModelID, error) {
	if id == noContentModel {
		return id, nil
	}
	model := c.rt.Models[id]
	if model.Mixed == mixed {
		return id, nil
	}
	model.Mixed = mixed
	return c.addModel(model)
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

func (c *compiler) appendModelParticle(m *contentModel, id contentModelID) error {
	p, ok, err := c.modelParticle(id)
	if err != nil || !ok {
		return err
	}
	m.Particles = append(m.Particles, p)
	return nil
}

func (c *compiler) modelParticle(id contentModelID) (particle, bool, error) {
	model := c.rt.Models[id]
	occurs := model.Occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return particle{}, false, nil
	}
	modelID := id
	if !occurs.isExactlyOne() {
		normalized := model
		normalized.Occurs = occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = c.addModel(normalized)
		if err != nil {
			return particle{}, false, err
		}
	}
	return modelParticle(modelID, occurs), true, nil
}

func (c *compiler) compileModel(n *rawNode, ctx *schemaContext) (contentModelID, error) {
	if n.Name.Local == xsdElemGroup {
		if ref, ok := n.attr(xsdAttrRef); ok {
			return c.compileModelGroupRef(n, ctx, ref)
		}
	}
	if id, ok := c.modelDone[n]; ok {
		if c.compilingModel[n] {
			if c.elementDepth > c.modelDepth[n] {
				return id, nil
			}
			return noContentModel, schemaCompileAt(n, ErrSchemaReference, "recursive model group")
		}
		return id, nil
	}
	id, err := c.addModel(contentModel{})
	if err != nil {
		return noContentModel, err
	}
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
		return noContentModel, schemaCompileAt(n, ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	m := contentModel{Kind: kind, Occurs: occurs}
	if err := c.compileModelChildren(n, ctx, &m); err != nil {
		return noContentModel, err
	}
	if err := c.checkElementDeclarationsConsistent(m); err != nil {
		return noContentModel, withSchemaCompileLocation(n, err)
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
		return noContentModel, schemaCompileAt(n, ErrSchemaReference, "unknown model group "+c.rt.Names.Format(q))
	}
	modelNode := firstModelChild(raw.node)
	if modelNode == nil {
		return noContentModel, schemaCompileAt(raw.node, ErrSchemaContentModel, "model group has no content "+c.rt.Names.Format(q))
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
		return noContentModel, schemaCompileAt(n, ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	model.Occurs = occurs
	return c.addModel(model)
}

func (c *compiler) recursiveModelGroupRef(q qName, id contentModelID, occurs occurrence, modelNode *rawNode) (contentModelID, error) {
	if c.elementDepth <= c.modelDepth[modelNode] {
		return noContentModel, schemaCompileAt(modelNode, ErrSchemaReference, "recursive model group "+c.rt.Names.Format(q))
	}
	ref := contentModel{
		Kind:      modelSequence,
		Occurs:    occurs,
		Particles: []particle{modelParticle(id, occurrence{Min: 1, Max: 1})},
	}
	return c.addModel(ref)
}

func modelKindForNode(n *rawNode) (modelKind, error) {
	switch n.Name.Local {
	case xsdElemSequence:
		return modelSequence, nil
	case xsdElemChoice:
		return modelChoice, nil
	case xsdElemAll:
		return modelAll, nil
	default:
		return 0, schemaCompileAt(n, ErrSchemaContentModel, "unsupported model "+n.Name.Local)
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
	case xsdElemElement:
		p, err := c.compileElementParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(m, p))
	case xsdElemAny:
		p, err := c.compileWildcardParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(m, p))
	case xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup:
		return c.appendNestedModelChild(m, child, ctx)
	default:
		return nil
	}
}

func (c *compiler) appendNestedModelChild(m *contentModel, child *rawNode, ctx *schemaContext) error {
	if child.Name.Local == xsdElemAll {
		return schemaCompileAt(child, ErrSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	childModelID, err := c.compileModel(child, ctx)
	if err != nil {
		return err
	}
	childModel := c.rt.Models[childModelID]
	if child.Name.Local == xsdElemGroup && childModel.Kind == modelAll {
		return schemaCompileAt(child, ErrSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	if appendFlattenedModelChild(m, childModel) {
		return nil
	}
	p, ok, err := c.modelParticle(childModelID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return withSchemaCompileLocation(child, appendParticle(m, p))
}

func appendFlattenedModelChild(m *contentModel, child contentModel) bool {
	if m.Kind == modelChoice && child.Kind == modelChoice && child.Occurs.isExactlyOne() {
		m.Particles = append(m.Particles, child.Particles...)
		return true
	}
	if m.Kind != modelSequence {
		return false
	}
	if (child.Kind == modelSequence || child.Kind == modelChoice) && len(child.Particles) == 1 {
		p := child.Particles[0]
		if canFlattenSingleParticleModel(child.Occurs, p.Occurs) {
			p.Occurs = multiplyOccurs(p.Occurs, child.Occurs)
			m.Particles = append(m.Particles, p)
			return true
		}
	}
	if child.Kind == modelSequence && len(child.Particles) > 1 && child.Occurs.isExactlyOne() {
		m.Particles = append(m.Particles, child.Particles...)
		return true
	}
	return false
}

// canFlattenSingleParticleModel names the non-obvious model flattening invariant.
func canFlattenSingleParticleModel(modelOccurs, particleOccurs occurrence) bool {
	return modelOccurs.isExactlyOne() ||
		particleOccurs.Min == 0 ||
		particleOccurs.isExactlyOne() ||
		(particleOccurs.Unbounded && (modelOccurs.Min > 0 || particleOccurs.Min == 1)) ||
		(!modelOccurs.Unbounded && modelOccurs.Min == modelOccurs.Max)
}

func appendParticle(m *contentModel, p particle) error {
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil
	}
	if m.Kind == modelAll && (p.Occurs.Unbounded || p.Occurs.Max > 1) {
		return schemaCompile(ErrSchemaOccurrence, "xs:all particles cannot repeat")
	}
	m.Particles = append(m.Particles, p)
	return nil
}

func validateModelOccurrence(n *rawNode, limits compileLimits) error {
	if n.Name.Local == xsdElemGroup {
		if err := validateKnownAttributes(n, n.Name.Local, isGroupOccurrenceAttribute); err != nil {
			return err
		}
	}
	occurs, err := parseOccurs(n, limits)
	if err != nil {
		return err
	}
	if n.Name.Local == xsdElemAll && (occurs.Unbounded || occurs.Max != 1 || occurs.Min > 1) {
		return schemaCompileAt(n, ErrSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	return validateModelGroupSyntax(n, limits)
}

func isGroupOccurrenceAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrMinOccurs, xsdAttrMaxOccurs, xsdAttrRef:
		return true
	default:
		return false
	}
}

func (c *compiler) checkCompiledModelsUPA() error {
	seen := make([]bool, len(c.rt.Models))
	for i := range c.rt.Models {
		model := c.rt.Models[i]
		clear(seen)
		if c.modelNeedsRuntimeSplitSeen(contentModelID(i), model, seen) || c.sequenceHasWildcardEquivalentOverlap(model) {
			continue
		}
		if err := c.checkDirectUPA(model); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) modelNeedsRuntimeSplitSeen(id contentModelID, model contentModel, seen []bool) bool {
	if validUint32Index(uint32(id), len(seen)) {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	if c.choiceNeedsRuntimeSplit(model, model.Occurs) {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind != particleModel {
			continue
		}
		child := c.rt.Models[p.Model]
		if c.choiceNeedsRuntimeSplit(child, p.Occurs) || c.modelNeedsRuntimeSplitSeen(p.Model, child, seen) {
			return true
		}
	}
	return false
}

func (c *compiler) choiceNeedsRuntimeSplit(model contentModel, occurs occurrence) bool {
	if model.Kind != modelChoice || occurs.isExactlyOne() {
		return false
	}
	for _, p := range model.Particles {
		if !p.Occurs.isExactlyOne() {
			return true
		}
	}
	return false
}

func (c *compiler) checkDirectUPA(m contentModel) error {
	switch m.Kind {
	case modelChoice, modelAll:
		msg := "UPA violation: overlapping particles in choice"
		if m.Kind == modelAll {
			msg = "UPA violation: overlapping particles in all"
		}
		return c.checkPairwiseUPA(m.Particles, msg)
	case modelSequence:
		return c.checkSequenceUPA(m)
	default:
	}
	return nil
}

func (c *compiler) checkPairwiseUPA(particles []particle, msg string) error {
	for i, p := range particles {
		for j := i + 1; j < len(particles); j++ {
			name, ok := c.particlesOverlap(p, particles[j])
			if ok {
				return c.upaError(msg, name)
			}
		}
	}
	return nil
}

func (c *compiler) checkSequenceUPA(m contentModel) error {
	for i, p := range m.Particles {
		for _, candidate := range c.particleContinuationParticles(p) {
			if err := c.checkSequenceContinuationUPA(m, candidate, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) checkSequenceContinuationUPA(m contentModel, candidate particle, start int) error {
	for j := start; j < len(m.Particles); j++ {
		next := m.Particles[j]
		name, ok := c.particlesOverlap(candidate, next)
		if !ok {
			if next.Occurs.Min > 0 {
				break
			}
			continue
		}
		if !m.Occurs.isExactlyOne() && c.wildcardEquivalentOverlap(candidate, next) {
			continue
		}
		return c.upaError("UPA violation: duplicate element in sequence", name)
	}
	return nil
}

func (c *compiler) upaError(msg string, name qName) error {
	if name.Local != 0 || name.Namespace != 0 {
		msg += " " + c.rt.Names.Format(name)
	}
	return schemaCompile(ErrSchemaContentModel, msg)
}

func (c *compiler) particleContinuationParticles(p particle) []particle {
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
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
		if p.Occurs.Unbounded || p.Occurs.Max > p.Occurs.Min {
			out = append(out, c.modelStartParticles(model)...)
		}
		out = append(out, c.modelContinuationParticles(model)...)
		return out
	}
	return nil
}

func (c *compiler) modelContinuationParticles(model contentModel) []particle {
	var out []particle
	if model.Occurs.Unbounded || model.Occurs.Max > model.Occurs.Min {
		out = append(out, c.modelStartParticles(model)...)
	}
	switch model.Kind {
	case modelSequence, modelChoice:
		for _, p := range model.Particles {
			out = append(out, c.particleContinuationParticles(p)...)
		}
	default:
	}
	return out
}

func (c *compiler) particleCanOverlapFollowing(p particle) bool {
	r := c.particleCountRange(p)
	if r.Unbounded {
		return true
	}
	return r.Max > r.Min
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
				if m.Particles[j].Occurs.Min > 0 {
					break
				}
			}
		}
	}
	return false
}

func (c *compiler) wildcardEquivalentOverlap(a, b particle) bool {
	if a.Kind != particleWildcard || b.Kind != particleWildcard {
		return false
	}
	wa := c.rt.Wildcards[a.Wildcard]
	wb := c.rt.Wildcards[b.Wildcard]
	return wildcardNamespaceEqual(wa, wb)
}

func (c *compiler) particlesOverlap(a, b particle) (qName, bool) {
	if a.Kind == particleModel {
		return c.modelStartOverlap(c.rt.Models[a.Model], b)
	}
	if b.Kind == particleModel {
		return c.modelStartOverlap(c.rt.Models[b.Model], a)
	}
	if a.Kind == particleWildcard && b.Kind == particleWildcard {
		return qName{}, wildcardsOverlap(c.rt.Wildcards[a.Wildcard], c.rt.Wildcards[b.Wildcard])
	}
	if name, ok := c.firstParticleElementNameMatchedBy(a, b); ok {
		return name, true
	}
	if name, ok := c.firstParticleElementNameMatchedBy(b, a); ok {
		return name, true
	}
	return qName{}, false
}

func (c *compiler) modelStartOverlap(model contentModel, p particle) (qName, bool) {
	switch model.Kind {
	case modelAll, modelChoice, modelSequence:
	default:
		return qName{}, false
	}
	for _, child := range model.Particles {
		if name, ok := c.particlesOverlap(child, p); ok {
			return name, true
		}
		if model.Kind == modelSequence && !c.particleEmptiable(child) {
			break
		}
	}
	return qName{}, false
}

func (c *compiler) particleMatchesName(p particle, name qName) bool {
	switch p.Kind {
	case particleElement:
		return c.elementParticleMatchesName(p.Element, name)
	case particleWildcard:
		w := c.rt.Wildcards[p.Wildcard]
		return wildcardAllowsNamespace(w, name.Namespace)
	case particleModel:
		return c.modelStartMatchesName(c.rt.Models[p.Model], name)
	}
	return false
}

func (c *compiler) firstParticleElementNameMatchedBy(src, dst particle) (qName, bool) {
	if src.Kind != particleElement {
		return qName{}, false
	}
	decl := c.rt.Elements[src.Element]
	if c.particleMatchesName(dst, decl.Name) {
		return decl.Name, true
	}
	allowed := c.rt.SubstitutionLookup[src.Element]
	if allowed == nil {
		return qName{}, false
	}
	for _, member := range c.rt.Substitutions[src.Element] {
		name := c.rt.Elements[member].Name
		if allowed[name] == member && c.particleMatchesName(dst, name) {
			return name, true
		}
	}
	return qName{}, false
}

func (c *compiler) elementParticleMatchesName(id elementID, name qName) bool {
	if c.rt.Elements[id].Name == name {
		return true
	}
	allowed := c.rt.SubstitutionLookup[id]
	if allowed == nil {
		return false
	}
	for _, member := range c.rt.Substitutions[id] {
		if c.rt.Elements[member].Name == name && allowed[name] == member {
			return true
		}
	}
	return false
}

func (c *compiler) modelStartMatchesName(model contentModel, name qName) bool {
	switch model.Kind {
	case modelAll, modelChoice:
		for _, p := range model.Particles {
			if c.particleMatchesName(p, name) {
				return true
			}
		}
	case modelSequence:
		for _, p := range model.Particles {
			if c.particleMatchesName(p, name) {
				return true
			}
			if !c.particleEmptiable(p) {
				break
			}
		}
	default:
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
	case particleWildcard:
	}
	return nil
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
	default:
	}
	return out
}

func parseOccurs(n *rawNode, limits compileLimits) (occurrence, error) {
	minOccurs := uint32(1)
	minDigits := "1"
	if v, ok := n.attr(xsdAttrMinOccurs); ok {
		digits, err := parseOccurrenceDigits(v)
		if err != nil {
			return occurrence{}, schemaCompileAt(n, ErrSchemaOccurrence, "invalid minOccurs "+v)
		}
		minDigits = digits
		if occurrenceUint32LimitExceeded(digits) {
			return occurrence{}, schemaCompileAt(n, ErrSchemaLimit, "minOccurs exceeds uint32 limit")
		}
		minOccurs = occurrenceUint32(digits)
	}
	maxOccurs := uint32(1)
	maxDigits := "1"
	if v, ok := n.attr(xsdAttrMaxOccurs); ok {
		if trimXMLWhitespace(v) == "unbounded" {
			return occurrence{Min: minOccurs, Unbounded: true}, nil
		}
		digits, err := parseOccurrenceDigits(v)
		if err != nil {
			return occurrence{}, schemaCompileAt(n, ErrSchemaOccurrence, "invalid maxOccurs "+v)
		}
		if maxOccursLimitExceeded(digits, limits.maxFiniteOccurs) {
			return occurrence{}, schemaCompileAt(n, ErrSchemaLimit, maxOccursLimitMessage(limits.maxFiniteOccurs))
		}
		maxDigits = digits
		maxOccurs = occurrenceUint32(digits)
	}
	if compareUnsignedDecimalText(maxDigits, minDigits) < 0 {
		return occurrence{}, schemaCompileAt(n, ErrSchemaOccurrence, "maxOccurs is less than minOccurs")
	}
	return occurrence{Min: minOccurs, Max: maxOccurs}, nil
}

func maxOccursLimitExceeded(digits string, limit uint64) bool {
	limitCap := uint64(maxUint32Value)
	if limit != 0 && limit < limitCap {
		limitCap = limit
	}
	return compareUnsignedDecimalText(digits, strconv.FormatUint(limitCap, 10)) > 0
}

func maxOccursLimitMessage(limit uint64) string {
	if limit != 0 && limit < uint64(maxUint32Value) {
		return "maxOccurs exceeds configured limit"
	}
	return "maxOccurs exceeds uint32 limit"
}

// occurrenceUint32LimitExceeded compares textually so huge values cannot overflow.
func occurrenceUint32LimitExceeded(digits string) bool {
	return compareUnsignedDecimalText(digits, maxUint32Text) > 0
}

func parseOccurrenceDigits(v string) (string, error) {
	v = trimXMLWhitespace(v)
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
	if compareUnsignedDecimalText(digits, maxUint32Text) > 0 {
		return maxUint32Value
	}
	v, err := strconv.ParseUint(digits, 10, 32)
	if err != nil {
		return maxUint32Value
	}
	return uint32(v)
}
