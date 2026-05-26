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
	if base.Kind == modelAny {
		return nil
	}
	if c.modelHasNoParticles(derivedID) {
		return nil
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
	if base.Kind == modelSequence && derived.Kind == modelChoice {
		return c.validateChoiceRestrictsSequence(base, derived)
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
		return schemaCompile(ErrSchemaContentModel, "content restriction is not subset of base")
	}
	for i := range base.Particles {
		if err := c.validateParticleRestriction(base.Particles[i], derived.Particles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) restrictionRepeatedChoiceParticles(baseID, derivedID contentModelID) []uint32 {
	if baseID == noContentModel || derivedID == noContentModel {
		return nil
	}
	base := c.rt.Models[baseID]
	derived := c.rt.Models[derivedID]
	if base.Kind != modelSequence || derived.Kind != modelSequence {
		return nil
	}
	var out []uint32
	baseIndex := 0
	for derivedIndex, derivedParticle := range derived.Particles {
		for baseIndex < len(base.Particles) {
			baseParticle := base.Particles[baseIndex]
			if c.validateParticleRestriction(baseParticle, derivedParticle) != nil {
				baseIndex++
				continue
			}
			if c.restrictionRepeatedChoiceParticle(baseParticle, derivedParticle) {
				out = append(out, uint32(derivedIndex))
			}
			baseIndex++
			break
		}
	}
	return out
}

func (c *compiler) restrictionRepeatedChoiceParticle(baseParticle, derivedParticle particle) bool {
	if baseParticle.Kind != particleModel || baseParticle.occurs.isExactlyOne() {
		return false
	}
	model := c.rt.Models[baseParticle.Model]
	if model.Kind != modelChoice || derivedParticle.Kind != particleElement {
		return false
	}
	return derivedParticle.occurs.Min <= 1 && derivedParticle.occurs.Unbounded
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
	return c.modelContainsChoiceBelow(c.rt.Models[p.Model], 0)
}

func (c *compiler) modelContainsChoiceBelow(model contentModel, depth int) bool {
	if depth > 0 && model.Kind == modelChoice {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind == particleModel && c.modelContainsChoiceBelow(c.rt.Models[p.Model], depth+1) {
			return true
		}
	}
	return false
}

func (c *compiler) choiceBranchRestricts(base, derived particle) bool {
	candidate := derived
	if base.Kind != particleModel && base.occurs.isExactlyOne() && c.particleNeedsChoiceBranchNormalization(derived) {
		candidate = singleParticle(derived)
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
	return c.validateMappedGroupRestriction(base, derived, "sequence restriction particle is not subset of all", "sequence restriction omits required all particle")
}

func (c *compiler) validateMappedGroupRestriction(base, derived contentModel, particleMsg, omittedMsg string) error {
	mapped := make([]bool, len(base.Particles))
	for _, derivedParticle := range derived.Particles {
		match := -1
		var unsupportedErr error
		for i, baseParticle := range base.Particles {
			if mapped[i] {
				continue
			}
			err := c.validateParticleRestriction(baseParticle, derivedParticle)
			if err == nil {
				match = i
				break
			}
			if IsUnsupported(err) {
				unsupportedErr = err
			}
		}
		if match < 0 {
			if unsupportedErr != nil {
				return unsupportedErr
			}
			return schemaCompile(ErrSchemaContentModel, particleMsg)
		}
		mapped[match] = true
	}
	for i, baseParticle := range base.Particles {
		if !mapped[i] && !c.particleEmptiable(baseParticle) {
			return schemaCompile(ErrSchemaContentModel, omittedMsg)
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

func (c *compiler) validateChoiceRestrictsSequence(base, derived contentModel) error {
	if derived.occurs.Max == 0 && !derived.occurs.Unbounded {
		return nil
	}
	if derived.occurs.Unbounded || derived.occurs.Max > 1 {
		return schemaCompile(ErrSchemaContentModel, "choice restriction occurrence is not subset of sequence")
	}
	for _, derivedParticle := range derived.Particles {
		if err := c.validateChoiceBranchRestrictsSequence(base, derivedParticle); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) validateChoiceBranchRestrictsSequence(base contentModel, derived particle) error {
	var unsupportedErr error
	for i, baseParticle := range base.Particles {
		err := c.validateParticleRestriction(baseParticle, derived)
		if err != nil {
			if IsUnsupported(err) {
				unsupportedErr = err
			}
			continue
		}
		if c.sequenceRemainderEmptiable(base.Particles, i) {
			return nil
		}
	}
	if unsupportedErr != nil {
		return unsupportedErr
	}
	return schemaCompile(ErrSchemaContentModel, "choice restriction branch is not subset of sequence")
}

func (c *compiler) sequenceRemainderEmptiable(particles []particle, selected int) bool {
	for i, p := range particles {
		if i != selected && !c.particleEmptiable(p) {
			return false
		}
	}
	return true
}

func sequenceChoiceRange(model contentModel) occurrence {
	particleCount := saturatingUint32(len(model.Particles))
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
	switch base.Kind {
	case particleWildcard:
		return c.validateParticleRestrictsWildcard(base, derived)
	case particleElement:
		return c.validateParticleRestrictsElement(base, derived)
	case particleModel:
		return c.validateParticleRestrictsModel(base, derived)
	default:
		return nil
	}
}

func (c *compiler) validateParticleRestrictsElement(base, derived particle) error {
	if derived.Kind == particleWildcard {
		return schemaCompile(ErrSchemaContentModel, "wildcard restriction is not subset of element")
	}
	if derived.Kind == particleModel {
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
	if derived.Kind != particleElement {
		return nil
	}
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
	if baseDecl.HasFixed && (!derivedDecl.HasFixed || !c.elementFixedValuesEqual(baseDecl, derivedDecl)) {
		return schemaCompile(ErrSchemaContentModel, "element restriction fixed value is not subset of base")
	}
	return nil
}

func (c *compiler) validateParticleRestrictsModel(base, derived particle) error {
	model := c.rt.Models[base.Model]
	if model.Kind == modelChoice {
		return c.validateParticleRestrictsChoiceModel(base, derived, model)
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
	if derived.Kind != particleModel {
		return nil
	}
	return c.validateContentRestriction(base.Model, derived.Model)
}

func (c *compiler) validateParticleRestrictsChoiceModel(base, derived particle, model contentModel) error {
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
		default:
		}
	}
	if !c.choiceRestrictionBranchAllowed(model.Particles, derived) {
		return schemaCompile(ErrSchemaContentModel, "choice restriction branch is not subset of base")
	}
	return nil
}

func (c *compiler) validateElementParticleRestrictsSequenceModel(base contentModel, derived particle) error {
	for i, baseParticle := range base.Particles {
		if err := c.validateParticleRestriction(baseParticle, derived); err == nil {
			if !c.sequenceRemainderEmptiable(base.Particles, i) {
				return schemaCompile(ErrSchemaContentModel, "sequence restriction omits required base particle")
			}
			return nil
		}
	}
	return schemaCompile(ErrSchemaContentModel, "sequence restriction particle is not subset of base")
}

func (c *compiler) elementRestrictionNameAllowed(baseID, derivedID elementID) bool {
	derivedName := c.rt.Elements[derivedID].Name
	_, ok := c.rt.SubstitutionLookup[baseID][derivedName]
	return ok && c.substitutionAllowed(baseID, derivedID)
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
	return base.FixedCanonical == derived.FixedCanonical
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
	return multiplyOccurs(term, model.occurs)
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
	return multiplyOccurs(term, p.occurs)
}

// addOccursRanges saturates sequence ranges before applying occurrence limits.
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
	if maxUint32Value-a < b {
		return maxUint32Value
	}
	return a + b
}

func saturatingMul(a, b uint32) uint32 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > maxUint32Value/b {
		return maxUint32Value
	}
	return a * b
}
