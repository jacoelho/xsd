package xsd

import "slices"

func (c *compiler) modelEmptiable(modelID contentModelID) bool {
	if modelID == noContentModel {
		return true
	}
	model := c.rt.Models[modelID]
	if model.occurs.Min == 0 {
		return true
	}
	switch model.Kind {
	case modelEmpty, modelAny:
		return true
	case modelSequence, modelAll:
		for _, p := range model.Particles {
			if !c.particleEmptiable(p) {
				return false
			}
		}
		return true
	case modelChoice:
		if slices.ContainsFunc(model.Particles, c.particleEmptiable) {
			return true
		}
	}
	return false
}

func (c *compiler) validateContentRestriction(baseID, derivedID contentModelID) error {
	if baseID == noContentModel || derivedID == noContentModel {
		return nil
	}
	if c.modelEmptiable(derivedID) && !c.modelEmptiable(baseID) {
		return schemaCompile(ErrSchemaContentModel, "content restriction is not subset of base")
	}
	base := c.rt.Models[baseID]
	derived := c.rt.Models[derivedID]
	if !occursRangeSubset(c.modelCountRange(derivedID), c.modelCountRange(baseID)) {
		return schemaCompile(ErrSchemaContentModel, "content restriction is not subset of base")
	}
	if len(base.Particles) == 1 && base.Particles[0].Kind == particleWildcard {
		for _, p := range derived.Particles {
			if err := c.validateParticleRestrictsWildcard(base.Particles[0], p); err != nil {
				return err
			}
		}
		return nil
	}
	if base.Kind == modelChoice && derived.Kind == modelChoice {
		return c.validateChoiceRestriction(base, derived)
	}
	if base.Kind == modelSequence && derived.Kind == modelSequence {
		return c.validateOrderedGroupRestriction(base, derived, "sequence restriction is not subset of base")
	}
	if base.Kind == modelAll && derived.Kind == modelAll {
		return c.validateOrderedGroupRestriction(base, derived, "all restriction is not subset of base")
	}
	if base.Kind == modelAll && derived.Kind == modelSequence {
		return c.validateSequenceRestrictsAll(base, derived)
	}
	if base.Kind == modelChoice && derived.Kind == modelSequence {
		return c.validateSequenceRestrictsChoice(base, derived)
	}
	if base.Kind == modelSequence && derived.Kind == modelAll {
		if len(base.Particles) == 1 && len(derived.Particles) == 1 {
			return c.validateParticleRestriction(base.Particles[0], derived.Particles[0])
		}
		return schemaCompile(ErrSchemaContentModel, "all restriction is not subset of sequence")
	}
	if c.modelContainsWildcard(derived) && !c.modelContainsWildcard(base) {
		return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of base")
	}
	if base.Kind != derived.Kind || len(base.Particles) != len(derived.Particles) {
		return nil
	}
	for i := range base.Particles {
		if err := c.validateParticleRestriction(base.Particles[i], derived.Particles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) restrictionCountLimits(baseID, derivedID contentModelID) []restrictionCountLimit {
	if baseID == noContentModel || derivedID == noContentModel {
		return nil
	}
	base := c.rt.Models[baseID]
	derived := c.rt.Models[derivedID]
	if base.Kind != modelSequence || derived.Kind != modelSequence {
		return nil
	}
	var limits []restrictionCountLimit
	baseIndex := 0
	for derivedIndex, derivedParticle := range derived.Particles {
		for baseIndex < len(base.Particles) {
			baseParticle := base.Particles[baseIndex]
			if c.validateParticleRestriction(baseParticle, derivedParticle) != nil {
				baseIndex++
				continue
			}
			if limit, ok := c.restrictionCountLimit(baseParticle, derivedParticle, derivedIndex); ok {
				limits = append(limits, limit)
			}
			baseIndex++
			break
		}
	}
	return limits
}

func (c *compiler) restrictionCountLimit(baseParticle, derivedParticle particle, derivedIndex int) (restrictionCountLimit, bool) {
	if baseParticle.Kind != particleModel {
		return restrictionCountLimit{}, false
	}
	model := c.rt.Models[baseParticle.Model]
	if model.Kind != modelChoice || baseParticle.occurs.isExactlyOne() {
		return restrictionCountLimit{}, false
	}
	if derivedParticle.Kind == particleModel {
		derivedModel := c.rt.Models[derivedParticle.Model]
		if derivedModel.Kind == modelChoice {
			return restrictionCountLimit{}, false
		}
	}
	if derivedParticle.Kind != particleElement {
		return restrictionCountLimit{}, false
	}
	r := c.particleCountRange(derivedParticle)
	if !r.Unbounded && r.Max <= 1 {
		return restrictionCountLimit{}, false
	}
	return restrictionCountLimit{particle: uint32(derivedIndex), Max: 1}, true
}

func (c *compiler) choiceRestrictionBranchAllowed(base []particle, derived particle) bool {
	for _, b := range base {
		if c.choiceBranchRestricts(b, derived) {
			return true
		}
	}
	return false
}

func (c *compiler) validateChoiceRestriction(base, derived contentModel) error {
	if !occursRangeSubset(derived.occurs, base.occurs) {
		return schemaCompile(ErrSchemaContentModel, "choice restriction occurrence is not subset of base")
	}
	if c.choiceRestrictionRequiresXSD11(base, derived) {
		return schemaCompile(ErrSchemaContentModel, "choice restriction requires XSD 1.1 intensional rules")
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		for baseIndex < len(base.Particles) {
			if c.choiceBranchRestricts(base.Particles[baseIndex], derivedParticle) {
				baseIndex++
				matched = true
				break
			}
			baseIndex++
		}
		if !matched {
			return schemaCompile(ErrSchemaContentModel, "choice restriction branch is not subset of base")
		}
	}
	return nil
}

func (c *compiler) choiceRestrictionRequiresXSD11(base, derived contentModel) bool {
	if base.occurs.isExactlyOne() && derived.occurs.Min < base.occurs.Min {
		return true
	}
	if base.occurs.isExactlyOne() && derived.occurs.isExactlyOne() && len(derived.Particles) < len(base.Particles) {
		for _, p := range derived.Particles {
			if p.Kind == particleModel && c.particleCountRange(p).Unbounded {
				return true
			}
		}
	}
	return slices.ContainsFunc(derived.Particles, c.particleContainsNestedChoice)
}

func (c *compiler) particleContainsNestedChoice(p particle) bool {
	if p.Kind != particleModel {
		return false
	}
	return c.modelContainsNestedChoice(c.rt.Models[p.Model], false)
}

func (c *compiler) modelContainsNestedChoice(model contentModel, nested bool) bool {
	if nested && model.Kind == modelChoice {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind == particleModel && c.modelContainsNestedChoice(c.rt.Models[p.Model], true) {
			return true
		}
	}
	return false
}

func (c *compiler) choiceBranchRestricts(base, derived particle) bool {
	candidate := derived
	if base.Kind != particleModel && base.occurs.isExactlyOne() && c.particleNeedsChoiceBranchNormalization(derived) {
		candidate = c.choiceBranchParticle(derived)
	}
	return c.validateParticleRestriction(base, candidate) == nil
}

func (c *compiler) particleNeedsChoiceBranchNormalization(p particle) bool {
	if c.particleEffectiveMin(p) > 0 {
		return true
	}
	r := c.particleCountRange(p)
	return r.Unbounded || r.Max > 1
}

func (c *compiler) validateOrderedGroupRestriction(base, derived contentModel, msg string) error {
	if !occursRangeSubset(derived.occurs, base.occurs) {
		return schemaCompile(ErrSchemaContentModel, msg)
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		for baseIndex < len(base.Particles) {
			err := c.validateParticleRestriction(base.Particles[baseIndex], derivedParticle)
			if err == nil {
				baseIndex++
				matched = true
				break
			}
			if IsUnsupported(err) {
				return err
			}
			if !c.particleEmptiable(base.Particles[baseIndex]) {
				return schemaCompile(ErrSchemaContentModel, msg)
			}
			baseIndex++
		}
		if !matched {
			return schemaCompile(ErrSchemaContentModel, msg)
		}
	}
	for ; baseIndex < len(base.Particles); baseIndex++ {
		if !c.particleEmptiable(base.Particles[baseIndex]) {
			return schemaCompile(ErrSchemaContentModel, msg)
		}
	}
	return nil
}

func (c *compiler) validateSequenceRestrictsAll(base, derived contentModel) error {
	if !occursRangeSubset(derived.occurs, base.occurs) {
		return schemaCompile(ErrSchemaContentModel, "sequence restriction occurrence is not subset of all")
	}
	mapped := make([]bool, len(base.Particles))
	for _, derivedParticle := range derived.Particles {
		match := -1
		for i, baseParticle := range base.Particles {
			if mapped[i] {
				continue
			}
			if c.validateParticleRestriction(baseParticle, derivedParticle) == nil {
				match = i
				break
			}
		}
		if match < 0 {
			return schemaCompile(ErrSchemaContentModel, "sequence restriction particle is not subset of all")
		}
		mapped[match] = true
	}
	for i, baseParticle := range base.Particles {
		if !mapped[i] && !c.particleEmptiable(baseParticle) {
			return schemaCompile(ErrSchemaContentModel, "sequence restriction omits required all particle")
		}
	}
	return nil
}

func (c *compiler) validateSequenceRestrictsChoice(base, derived contentModel) error {
	if !occursRangeSubset(sequenceChoiceRange(derived), base.occurs) {
		return schemaCompile(ErrSchemaContentModel, "sequence restriction occurrence is not subset of choice")
	}
	for _, derivedParticle := range derived.Particles {
		if !c.choiceRestrictionBranchAllowed(base.Particles, derivedParticle) {
			return schemaCompile(ErrSchemaContentModel, "sequence restriction particle is not subset of choice")
		}
	}
	return nil
}

func sequenceChoiceRange(model contentModel) occurrence {
	particleCount := uint32(len(model.Particles))
	if model.occurs.Unbounded {
		return occurrence{Min: saturatingMul(model.occurs.Min, particleCount), Unbounded: true}
	}
	return occurrence{Min: saturatingMul(model.occurs.Min, particleCount), Max: saturatingMul(model.occurs.Max, particleCount)}
}

func (c *compiler) modelContainsWildcard(model contentModel) bool {
	return slices.ContainsFunc(model.Particles, c.particleContainsWildcard)
}

func (c *compiler) particleContainsWildcard(p particle) bool {
	switch p.Kind {
	case particleWildcard:
		return true
	case particleModel:
		return c.modelContainsWildcard(c.rt.Models[p.Model])
	default:
		return false
	}
}

func (c *compiler) validateParticleRestriction(base, derived particle) error {
	if !occursRangeSubset(c.particleCountRange(derived), c.particleCountRange(base)) {
		return schemaCompile(ErrSchemaContentModel, "particle restriction occurrence is not subset of base")
	}
	if base.Kind == particleWildcard && derived.Kind == particleWildcard {
		if !c.wildcardSubset(derived.wildcard, base.wildcard) {
			return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of base")
		}
		return nil
	}
	if base.Kind == particleElement && derived.Kind == particleWildcard {
		return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of element")
	}
	if base.Kind == particleElement && derived.Kind == particleModel {
		model := c.rt.Models[derived.Model]
		if model.Kind == modelChoice {
			for _, p := range model.Particles {
				if !c.choiceBranchRestricts(base, p) {
					return schemaCompile(ErrSchemaContentModel, "choice restriction branch is not subset of element")
				}
			}
			return nil
		}
		return schemaCompile(ErrSchemaContentModel, "model group restriction is not subset of element")
	}
	if base.Kind == particleWildcard {
		return c.validateParticleRestrictsWildcard(base, derived)
	}
	if base.Kind == particleElement && derived.Kind == particleElement {
		baseDecl := c.rt.Elements[base.Element]
		derivedDecl := c.rt.Elements[derived.Element]
		if baseDecl.Name != derivedDecl.Name && !c.elementRestrictionNameAllowed(base.Element, derived.Element) {
			return schemaCompile(ErrSchemaContentModel, "element restriction name is not subset of base")
		}
		if !c.typeDerivesByRestriction(derivedDecl.Type, baseDecl.Type) {
			return schemaCompile(ErrSchemaContentModel, "element restriction type is not derived from base")
		}
		if derivedDecl.Nillable && !baseDecl.Nillable {
			return schemaCompile(ErrSchemaContentModel, "element restriction nillable is not subset of base")
		}
		if derivedDecl.Block&baseDecl.Block != baseDecl.Block {
			return schemaCompile(ErrSchemaContentModel, "element restriction block is not subset of base")
		}
		if baseDecl.HasFixed {
			if !derivedDecl.HasFixed || !c.elementFixedValuesEqual(baseDecl, derivedDecl) {
				return schemaCompile(ErrSchemaContentModel, "element restriction fixed value is not subset of base")
			}
		}
		return nil
	}
	if base.Kind == particleModel {
		model := c.rt.Models[base.Model]
		if model.Kind == modelChoice {
			if derived.Kind == particleModel {
				derivedModel := c.rt.Models[derived.Model]
				if derivedModel.Kind == modelChoice && derived.occurs.Min < base.occurs.Min {
					return schemaCompile(ErrSchemaContentModel, "choice restriction occurrence is not subset of base")
				}
				switch derivedModel.Kind {
				case modelChoice:
					return c.validateChoiceRestriction(model, derivedModel)
				case modelSequence:
					return c.validateSequenceRestrictsChoice(model, derivedModel)
				}
			}
			if !c.choiceRestrictionBranchAllowed(model.Particles, derived) {
				return schemaCompile(ErrSchemaContentModel, "choice restriction branch is not subset of base")
			}
			return nil
		}
		if len(model.Particles) == 1 {
			return c.validateParticleRestriction(model.Particles[0], derived)
		}
		if derived.Kind == particleWildcard {
			return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of model group")
		}
		if model.Kind == modelSequence && derived.Kind == particleElement {
			return c.validateElementParticleRestrictsSequenceModel(model, derived)
		}
	}
	if base.Kind == particleModel && derived.Kind == particleModel {
		baseModel := c.rt.Models[base.Model]
		derivedModel := c.rt.Models[derived.Model]
		if baseModel.Kind == modelChoice && derivedModel.Kind == modelChoice && derived.occurs.Min < base.occurs.Min {
			return schemaCompile(ErrSchemaContentModel, "choice restriction occurrence is not subset of base")
		}
		return c.validateContentRestriction(base.Model, derived.Model)
	}
	return nil
}

func (c *compiler) validateElementParticleRestrictsSequenceModel(base contentModel, derived particle) error {
	for i, baseParticle := range base.Particles {
		if err := c.validateParticleRestriction(baseParticle, derived); err == nil {
			for j := range i {
				if !c.particleEmptiable(base.Particles[j]) {
					return schemaCompile(ErrSchemaContentModel, "sequence restriction omits required base particle")
				}
			}
			for j := i + 1; j < len(base.Particles); j++ {
				if !c.particleEmptiable(base.Particles[j]) {
					return schemaCompile(ErrSchemaContentModel, "sequence restriction omits required base particle")
				}
			}
			return nil
		}
	}
	return schemaCompile(ErrSchemaContentModel, "sequence restriction particle is not subset of base")
}

func (c *compiler) choiceBranchParticle(p particle) particle {
	p.occurs = occurrence{Min: 1, Max: 1}
	return p
}

func (c *compiler) elementRestrictionNameAllowed(baseID, derivedID elementID) bool {
	for _, member := range c.rt.Substitutions[baseID] {
		if member == derivedID && c.substitutionAllowed(baseID, derivedID) {
			return true
		}
	}
	baseName := c.rt.Elements[baseID].Name
	derivedName := c.rt.Elements[derivedID].Name
	return c.rawSubstitutionMemberOf(derivedName, baseName, make(map[qName]bool)) && c.substitutionAllowed(baseID, derivedID)
}

func (c *compiler) rawSubstitutionMemberOf(member, head qName, seen map[qName]bool) bool {
	if seen[member] {
		return false
	}
	seen[member] = true
	raw, ok := c.elementRaw[member]
	if !ok {
		return false
	}
	headLex, ok := raw.node.attr("substitutionGroup")
	if !ok {
		return false
	}
	q, err := c.resolveQNameChecked(raw.node, raw.ctx, headLex)
	if err != nil {
		return false
	}
	if q == head {
		return true
	}
	return c.rawSubstitutionMemberOf(q, head, seen)
}

func (c *compiler) validateParticleRestrictsWildcard(base, derived particle) error {
	if base.Kind != particleWildcard {
		return nil
	}
	switch derived.Kind {
	case particleElement:
		if !c.wildcardAllowsQName(base.wildcard, c.rt.Elements[derived.Element].Name) {
			return schemaCompile(ErrSchemaContentModel, "element restriction is not allowed by wildcard")
		}
	case particleWildcard:
		if !c.wildcardSubset(derived.wildcard, base.wildcard) {
			return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of base")
		}
	case particleModel:
		model := c.rt.Models[derived.Model]
		for _, child := range model.Particles {
			if err := c.validateParticleRestrictsWildcard(base, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) particleEffectiveMin(p particle) uint32 {
	if p.Kind == particleModel && c.modelEmptiable(p.Model) {
		return 0
	}
	return p.occurs.Min
}

func (c *compiler) particleEmptiable(p particle) bool {
	if p.occurs.Min == 0 {
		return true
	}
	if p.Kind == particleModel {
		return c.modelEmptiable(p.Model)
	}
	return false
}

func (c *compiler) elementFixedValuesEqual(base, derived elementDecl) bool {
	typeID := c.elementValueSimpleType(base)
	if typeID == noSimpleType {
		return base.Fixed == derived.Fixed
	}
	baseFixed, baseErr := validateSimpleValue(&c.rt, typeID, base.Fixed, nil)
	derivedFixed, derivedErr := validateSimpleValue(&c.rt, typeID, derived.Fixed, nil)
	return baseErr == nil && derivedErr == nil && baseFixed == derivedFixed
}

func (c *compiler) elementValueSimpleType(decl elementDecl) simpleTypeID {
	if decl.Type.Kind == typeSimple {
		return simpleTypeID(decl.Type.ID)
	}
	if decl.Type.Kind == typeComplex {
		ct := c.rt.ComplexTypes[decl.Type.ID]
		if ct.SimpleValue {
			return ct.TextType
		}
	}
	return noSimpleType
}

func (c *compiler) modelCountRange(modelID contentModelID) occurrence {
	if modelID == noContentModel {
		return occurrence{}
	}
	model := c.rt.Models[modelID]
	var term occurrence
	switch model.Kind {
	case modelEmpty:
		term = occurrence{}
	case modelAny:
		term = occurrence{Min: 0, Unbounded: true}
	case modelSequence, modelAll:
		for _, p := range model.Particles {
			term = addOccursRanges(term, c.particleCountRange(p))
		}
	case modelChoice:
		if len(model.Particles) == 0 {
			term = occurrence{}
			break
		}
		term = c.particleCountRange(model.Particles[0])
		for _, p := range model.Particles[1:] {
			term = unionOccursRanges(term, c.particleCountRange(p))
		}
	}
	return multiplyOccursRange(term, model.occurs)
}

func (c *compiler) particleCountRange(p particle) occurrence {
	var term occurrence
	switch p.Kind {
	case particleElement, particleWildcard:
		term = occurrence{Min: 1, Max: 1}
	case particleModel:
		term = c.modelCountRange(p.Model)
	default:
		term = occurrence{}
	}
	return multiplyOccursRange(term, p.occurs)
}

func addOccursRanges(a, b occurrence) occurrence {
	return occurrence{Min: saturatingAdd(a.Min, b.Min), Max: saturatingAdd(a.Max, b.Max), Unbounded: a.Unbounded || b.Unbounded}
}

func unionOccursRanges(a, b occurrence) occurrence {
	minOccurs := min(b.Min, a.Min)
	if a.Unbounded || b.Unbounded {
		return occurrence{Min: minOccurs, Unbounded: true}
	}
	maxOccurs := max(b.Max, a.Max)
	return occurrence{Min: minOccurs, Max: maxOccurs}
}

func multiplyOccursRange(term, occurs occurrence) occurrence {
	minOccurs := saturatingMul(term.Min, occurs.Min)
	if term.Unbounded || occurs.Unbounded {
		return occurrence{Min: minOccurs, Unbounded: true}
	}
	return occurrence{Min: minOccurs, Max: saturatingMul(term.Max, occurs.Max)}
}

func multiplyOccurs(a, b occurrence) occurrence {
	minOccurs := saturatingMul(a.Min, b.Min)
	if a.Unbounded || b.Unbounded {
		return occurrence{Min: minOccurs, Unbounded: true}
	}
	return occurrence{Min: minOccurs, Max: saturatingMul(a.Max, b.Max)}
}

func occursRangeSubset(derived, base occurrence) bool {
	if derived.Min < base.Min {
		return false
	}
	if base.Unbounded {
		return true
	}
	if derived.Unbounded {
		return false
	}
	return derived.Max <= base.Max
}

func saturatingAdd(a, b uint32) uint32 {
	if ^uint32(0)-a < b {
		return ^uint32(0)
	}
	return a + b
}

func saturatingMul(a, b uint32) uint32 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > ^uint32(0)/b {
		return ^uint32(0)
	}
	return a * b
}
