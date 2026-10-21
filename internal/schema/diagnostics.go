package schema

import (
	"slices"

	"github.com/jacoelho/xsd/internal/diagnostictext"
)

// diagnosticNames retains names needed only when assessment fails. These
// columns do not enlarge the component reads copied by successful validation.
type diagnosticNames struct {
	locals       []string
	identities   []QName
	simpleTypes  []QName
	complexTypes []QName
}

func newDiagnosticNames(build *schemaBuild) diagnosticNames {
	names := diagnosticNames{locals: slices.Clone(build.Names.locals), identities: make([]QName, len(build.Identities)), simpleTypes: make([]QName, len(build.SimpleTypes)), complexTypes: make([]QName, len(build.ComplexTypes))}
	for i, identity := range build.Identities {
		names.identities[i] = identity.Name
	}
	for i, typ := range build.SimpleTypes {
		names.simpleTypes[i] = NoQName()
		if typ.Scope == DeclarationScopeGlobal {
			names.simpleTypes[i] = typ.Name
		}
	}
	for i, typ := range build.ComplexTypes {
		names.complexTypes[i] = NoQName()
		if typ.Scope == DeclarationScopeGlobal {
			names.complexTypes[i] = typ.Name
		}
	}
	return names
}

func (rt *Schema) diagnosticName(name QName) (ExpandedName, bool) {
	if !validRuntimeID(uint32(name.Local), len(rt.program.DiagnosticNames.locals)) || !validRuntimeID(uint32(name.Namespace), len(rt.program.Names.namespaces)) {
		return ExpandedName{}, false
	}
	return ExpandedName{Namespace: rt.program.Names.Namespace(name.Namespace), Local: rt.program.DiagnosticNames.locals[name.Local]}, true
}

// DiagnosticElementName returns the expanded declaration name on failure.
func (rt *Schema) DiagnosticElementName(id ElementID) (ExpandedName, bool) {
	if rt == nil {
		return ExpandedName{}, false
	}
	name, ok := rt.program.Elements.name(id)
	if !ok {
		return ExpandedName{}, false
	}
	return rt.diagnosticName(name)
}

// DiagnosticIdentityName returns the expanded declaration name on failure.
func (rt *Schema) DiagnosticIdentityName(id IdentityConstraintID) (ExpandedName, bool) {
	if rt == nil || !ValidIdentityConstraintID(id, len(rt.program.DiagnosticNames.identities)) {
		return ExpandedName{}, false
	}
	return rt.diagnosticName(rt.program.DiagnosticNames.identities[id])
}

// DiagnosticTypeName returns a named type's expanded name. Anonymous types have
// no diagnostic name; generated compiler names are not instance-facing facts.
func (rt *Schema) DiagnosticTypeName(id TypeID) (ExpandedName, bool) {
	if rt == nil {
		return ExpandedName{}, false
	}
	if simple, ok := id.Simple(); ok {
		if !ValidSimpleTypeID(simple, len(rt.program.DiagnosticNames.simpleTypes)) {
			return ExpandedName{}, false
		}
		return rt.diagnosticName(rt.program.DiagnosticNames.simpleTypes[simple])
	}
	if complexID, ok := id.Complex(); ok {
		if !ValidComplexTypeID(complexID, len(rt.program.DiagnosticNames.complexTypes)) {
			return ExpandedName{}, false
		}
		return rt.diagnosticName(rt.program.DiagnosticNames.complexTypes[complexID])
	}
	return ExpandedName{}, false
}

// ContentExpectation is one currently admissible particle's diagnostic facts.
// Namespace slices are bounded, owned copies and may be changed by the caller.
type ContentExpectation struct {
	Name              ExpandedName
	ExcludedNamespace string
	Namespaces        []string
	WildcardMode      WildcardMode
	IsWildcard        bool
	MoreNamespaces    bool
}

// ContentExpectations is a bounded error-only projection of the authoritative
// content state. More means alternatives or traversal work were truncated.
type ContentExpectations struct {
	values [diagnostictext.MaxAlternatives]ContentExpectation
	count  int
	More   bool
}

// Len returns the number of displayed alternatives.
func (e ContentExpectations) Len() int { return e.count }

// At returns a displayed alternative by index.
func (e ContentExpectations) At(i int) (ContentExpectation, bool) {
	if i < 0 || i >= e.count {
		return ContentExpectation{}, false
	}
	return e.values[i], true
}

type contentExpectationProjection struct {
	schema    *Schema
	result    ContentExpectations
	remaining int
}

// ContentExpectationPurpose selects the failure being explained.
type ContentExpectationPurpose uint8

const (
	// ExpectedNextChild explains an unexpected child at the current state.
	ExpectedNextChild ContentExpectationPurpose = iota
	// MissingRequiredContent explains why the current content cannot end.
	MissingRequiredContent
)

// ExpectedContent returns next-child alternatives without changing state or
// scratch. MissingRequiredContent omits optional, unseen all-group terms when explaining
// incomplete content. DFA alternatives retain their existing occurrence guards.
// At most 256 edges, all terms, and raw substitution entries are inspected.
// valid distinguishes invalid metadata from a valid state with no alternatives.
func (rt *Schema) ExpectedContent(state ContentState, scratch *ContentScratch, purpose ContentExpectationPurpose) (result ContentExpectations, valid bool) {
	if !state.HasModel() || !ValidContentModelID(state.model, len(rt.program.CompiledModels)) {
		return result, false
	}
	if purpose != ExpectedNextChild && purpose != MissingRequiredContent {
		return result, false
	}
	projection := contentExpectationProjection{schema: rt, remaining: 256}
	model := &rt.program.CompiledModels[state.model]
	switch model.Kind {
	case CompiledModelEmpty:
		return result, true
	case CompiledModelAny:
		projection.add(ContentExpectation{IsWildcard: true, WildcardMode: WildcardAny})
		return projection.result, true
	case CompiledModelAll:
		valid = projection.allContent(model, scratch, purpose)
	case CompiledModelDFA:
		valid = projection.dfaContent(state, model)
	default:
		return result, false
	}
	if !valid {
		return result, false
	}
	return projection.result, true
}

func (p *contentExpectationProjection) allContent(model *compiledModelRead, scratch *ContentScratch, purpose ContentExpectationPurpose) bool {
	for index, term := range model.All {
		if p.result.More || !p.spend() {
			return true
		}
		if !p.allTerm(term, index, scratch, purpose) {
			return false
		}
	}
	return true
}

func (p *contentExpectationProjection) allTerm(term compiledAllTermRead, index int, scratch *ContentScratch, purpose ContentExpectationPurpose) bool {
	seen, valid := scratch.AllSeen(index)
	if !valid {
		return false
	}
	if seen || purpose == MissingRequiredContent && !term.Required {
		return true
	}
	return p.particle(term.Particle)
}

func (p *contentExpectationProjection) dfaContent(state ContentState, model *compiledModelRead) bool {
	if !ValidUint32Index(state.state, len(model.Rows)) {
		return false
	}
	for _, edge := range model.Rows[state.state].Edges {
		if p.result.More || !p.spend() {
			return true
		}
		if !p.dfaEdge(state, model, edge) {
			return false
		}
	}
	return true
}

func (p *contentExpectationProjection) dfaEdge(state ContentState, model *compiledModelRead, edge compiledModelEdgeRead) bool {
	if !ValidUint32Index(edge.To, len(model.Rows)) {
		return false
	}
	if _, valid := nextPublishedDFAState(state, model, edge); !valid {
		return true
	}
	return p.particle(edge.Particle)
}

func (p *contentExpectationProjection) spend() bool {
	if p.remaining == 0 {
		p.result.More = true
		return false
	}
	p.remaining--
	return true
}
func (p *contentExpectationProjection) add(item ContentExpectation) {
	for i := range p.result.count {
		if !item.IsWildcard && !p.result.values[i].IsWildcard && item.Name == p.result.values[i].Name {
			return
		}
	}
	if p.result.count == len(p.result.values) {
		p.result.More = true
		return
	}
	p.result.values[p.result.count] = item
	p.result.count++
}
func (p *contentExpectationProjection) particle(particle compiledParticleRead) bool {
	switch particle.Kind {
	case ParticleElement:
		return p.element(particle.Element)
	case ParticleWildcard:
		if !ValidWildcardID(particle.Wildcard, len(p.schema.program.Wildcards)) {
			return false
		}
		wildcard := p.schema.program.Wildcards[particle.Wildcard]
		if !wildcard.valid {
			return false
		}
		count := min(len(wildcard.namespaces), diagnostictext.MaxAlternatives)
		p.add(ContentExpectation{IsWildcard: true, WildcardMode: wildcard.mode, ExcludedNamespace: wildcard.otherThan, Namespaces: slices.Clone(wildcard.namespaces[:count]), MoreNamespaces: count < len(wildcard.namespaces)})
		return true
	case ParticleModel:
		return false
	}
	return false
}
func (p *contentExpectationProjection) element(id ElementID) bool {
	name, valid := p.schema.program.Elements.name(id)
	if !valid {
		return false
	}
	info, valid := p.schema.program.Elements.start(id)
	if !valid {
		return false
	}
	if !info.Abstract && !p.namedElement(name) {
		return false
	}
	return p.substitutionMembers(id)
}

func (p *contentExpectationProjection) namedElement(name QName) bool {
	expanded, valid := p.schema.diagnosticName(name)
	if !valid {
		return false
	}
	p.add(ContentExpectation{Name: expanded})
	return true
}

func (p *contentExpectationProjection) substitutionMembers(id ElementID) bool {
	table := p.schema.program.Substitutions
	span, valid := table.span(id)
	if !valid {
		return true
	}
	for _, entry := range table.entries[span.start : span.start+span.count] {
		if p.result.More || !p.spend() {
			return true
		}
		if !entry.effective {
			continue
		}
		if !p.namedElement(entry.name) {
			return false
		}
	}
	return true
}
