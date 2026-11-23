package schema

import (
	"cmp"
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
	wildcards    []ContentExpectation
}

func newDiagnosticNames(build *schemaBuild) diagnosticNames {
	names := diagnosticNames{locals: slices.Clone(build.Names.locals), identities: make([]QName, len(build.Identities)), simpleTypes: make([]QName, len(build.SimpleTypes)), complexTypes: make([]QName, len(build.ComplexTypes)), wildcards: make([]ContentExpectation, len(build.Wildcards))}
	for i, wildcard := range build.Wildcards {
		names.wildcards[i] = wildcardExpectation(&build.Names, wildcard)
	}
	for i, identity := range build.Identities {
		names.identities[i] = identity.Name
	}
	for i, typ := range build.SimpleTypes {
		names.simpleTypes[i] = noQName()
		if typ.Scope == declarationScopeGlobal {
			names.simpleTypes[i] = typ.Name
		}
	}
	for i, typ := range build.ComplexTypes {
		names.complexTypes[i] = noQName()
		if typ.Scope == declarationScopeGlobal {
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
	if rt == nil || !validIdentityConstraintID(id, len(rt.program.DiagnosticNames.identities)) {
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
		if !validSimpleTypeID(simple, len(rt.program.DiagnosticNames.simpleTypes)) {
			return ExpandedName{}, false
		}
		return rt.diagnosticName(rt.program.DiagnosticNames.simpleTypes[simple])
	}
	if complexID, ok := id.Complex(); ok {
		if !validComplexTypeID(complexID, len(rt.program.DiagnosticNames.complexTypes)) {
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
	WildcardMode      wildcardMode
	IsWildcard        bool
	MoreNamespaces    bool
}

// ContentExpectations is a bounded error-only projection of the authoritative
// content state. It holds the first alternatives in diagnostic order:
// wildcards, then element names by namespace URI, then local name, each listed
// once however many particles render it. Wildcards take at most half of the
// displayed alternatives while element names remain to show, and at least
// that half while wildcards remain, so neither kind hides the other. More
// means alternatives or traversal work were truncated.
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
	wildcards contentExpectationRanking
	elements  contentExpectationRanking
	remaining int
	more      bool
}

// contentExpectationRanking keeps the first distinct alternatives of one kind
// in diagnostic order, enough to fill every displayed slot. Their namespace
// slices borrow published wildcard state until result copies them.
type contentExpectationRanking struct {
	values  [diagnostictext.MaxAlternatives]ContentExpectation
	count   int
	dropped bool
}

// insert keeps item unless an alternative that renders the same is kept or
// the ranking is full of alternatives that precede it. The ranking therefore
// does not depend on traversal order once every alternative is inspected.
func (r *contentExpectationRanking) insert(item ContentExpectation) {
	at := r.count
	for ; at > 0; at-- {
		order := compareContentExpectation(item, r.values[at-1])
		if order == 0 {
			return
		}
		if order > 0 {
			break
		}
	}
	if r.count == len(r.values) {
		r.dropped = true
		if at == r.count {
			return
		}
		r.count--
	}
	copy(r.values[at+1:r.count+1], r.values[at:r.count])
	r.values[at] = item
	r.count++
}

// sortsAfterFull reports whether the ranking is full and item sorts after
// every alternative it keeps, so insert would drop it.
func (r *contentExpectationRanking) sortsAfterFull(item ContentExpectation) bool {
	return r.count == len(r.values) && compareContentExpectation(item, r.values[r.count-1]) > 0
}

// result applies the display rule of ContentExpectations to the rankings.
func (p *contentExpectationProjection) result() ContentExpectations {
	const slots = diagnostictext.MaxAlternatives
	wildcards := min(p.wildcards.count, max(slots/2, slots-p.elements.count))
	elements := min(p.elements.count, slots-wildcards)
	out := ContentExpectations{
		More: p.more || p.wildcards.dropped || p.elements.dropped || wildcards < p.wildcards.count || elements < p.elements.count,
	}
	for _, item := range p.wildcards.values[:wildcards] {
		item.Namespaces = slices.Clone(item.Namespaces)
		out.values[out.count] = item
		out.count++
	}
	for _, item := range p.elements.values[:elements] {
		out.values[out.count] = item
		out.count++
	}
	return out
}

// contentExpectationPurpose selects the failure being explained.
type contentExpectationPurpose uint8

const (
	// ExpectedNextChild explains an unexpected child at the current state.
	ExpectedNextChild contentExpectationPurpose = iota
	// MissingRequiredContent explains why the current content cannot end.
	MissingRequiredContent
)

// ExpectedContent returns next-child alternatives without changing state or
// scratch. MissingRequiredContent omits optional, unseen all-group terms when explaining
// incomplete content. DFA alternatives retain their existing occurrence guards.
// At most 256 edges, all terms, and raw substitution entries are inspected.
// valid distinguishes invalid metadata from a valid state with no alternatives.
func (rt *Schema) ExpectedContent(state ContentState, scratch *ContentScratch, purpose contentExpectationPurpose) (result ContentExpectations, valid bool) {
	if !state.HasModel() || !validContentModelID(state.model, len(rt.program.CompiledModels)) {
		return result, false
	}
	if purpose != ExpectedNextChild && purpose != MissingRequiredContent {
		return result, false
	}
	projection := contentExpectationProjection{schema: rt, remaining: 256}
	model := &rt.program.CompiledModels[state.model]
	switch model.Kind {
	case compiledModelEmpty:
		return result, true
	case compiledModelAny:
		projection.add(ContentExpectation{IsWildcard: true, WildcardMode: WildcardAny})
		return projection.result(), true
	case compiledModelAll:
		valid = projection.allContent(model, scratch, purpose)
	case compiledModelDFA:
		valid = projection.dfaContent(state, model)
	default:
		return result, false
	}
	if !valid {
		return result, false
	}
	return projection.result(), true
}

func (p *contentExpectationProjection) allContent(model *compiledModelRead, scratch *ContentScratch, purpose contentExpectationPurpose) bool {
	for index, term := range model.All {
		if !p.spend() {
			return true
		}
		if !p.allTerm(term, index, scratch, purpose) {
			return false
		}
	}
	return true
}

func (p *contentExpectationProjection) allTerm(term compiledAllTermRead, index int, scratch *ContentScratch, purpose contentExpectationPurpose) bool {
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
	if !validUint32Index(state.state, len(model.Rows)) {
		return false
	}
	for _, edge := range model.Rows[state.state].Edges {
		if !p.spend() {
			return true
		}
		if !p.dfaEdge(state, model, edge) {
			return false
		}
	}
	return true
}

func (p *contentExpectationProjection) dfaEdge(state ContentState, model *compiledModelRead, edge compiledModelEdgeRead) bool {
	if !validUint32Index(edge.To, len(model.Rows)) {
		return false
	}
	if _, valid := nextPublishedDFAState(state, model, edge); !valid {
		return true
	}
	return p.particle(edge.Particle)
}

func (p *contentExpectationProjection) spend() bool {
	if p.remaining == 0 {
		p.more = true
		return false
	}
	p.remaining--
	return true
}

// add ranks item among the alternatives of its kind. item.Namespaces may
// borrow published wildcard state: only displayed alternatives are copied.
func (p *contentExpectationProjection) add(item ContentExpectation) {
	if item.IsWildcard {
		p.wildcards.insert(item)
		return
	}
	p.elements.insert(item)
}

// compareContentExpectation orders alternatives of one kind, the kind each
// contentExpectationRanking holds; result lists wildcards before element names.
func compareContentExpectation(a, b ContentExpectation) int {
	return cmp.Or(
		compareExpandedName(a.Name, b.Name),
		cmp.Compare(wildcardRendering(a.WildcardMode), wildcardRendering(b.WildcardMode)),
		compareNamespaceURI(a.ExcludedNamespace, b.ExcludedNamespace),
		slices.CompareFunc(a.Namespaces, b.Namespaces, compareNamespaceURI),
		compareFalseFirst(a.MoreNamespaces, b.MoreNamespaces),
	)
}

// wildcardRendering maps a wildcard mode to the modes rendered alike: a
// target-namespace wildcard is listed as the namespace list it holds.
func wildcardRendering(mode wildcardMode) wildcardMode {
	if mode == WildcardTargetNamespace {
		return WildcardList
	}
	return mode
}

func compareFalseFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

func (p *contentExpectationProjection) particle(particle compiledParticleRead) bool {
	switch particle.Kind {
	case particleElement:
		return p.element(particle.Element)
	case particleWildcard:
		if !validWildcardID(particle.Wildcard, len(p.schema.program.Wildcards)) || !validWildcardID(particle.Wildcard, len(p.schema.program.DiagnosticNames.wildcards)) {
			return false
		}
		if !p.schema.program.Wildcards[particle.Wildcard].valid {
			return false
		}
		p.add(p.schema.program.DiagnosticNames.wildcards[particle.Wildcard])
		return true
	case particleModel:
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

// substitutionMembers adds the effective members of head id in expanded-name
// order. Once the element ranking is full and a member sorts after every kept
// element, so does each member after it: the ranking is marked as having
// dropped alternatives and the rest of the head is not inspected.
func (p *contentExpectationProjection) substitutionMembers(id ElementID) bool {
	valid := true
	p.schema.program.Substitutions.forEachEntryByName(id, func(entry substitutionEntry) bool {
		if !p.spend() {
			return false
		}
		if !entry.effective {
			return true
		}
		name, ok := p.schema.diagnosticName(entry.name)
		if !ok {
			valid = false
			return false
		}
		item := ContentExpectation{Name: name}
		if p.elements.sortsAfterFull(item) {
			p.elements.dropped = true
			return false
		}
		p.add(item)
		return true
	})
	return valid
}
