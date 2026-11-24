package xsd_test

import (
	"errors"
	"flag"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

var contentModelOracleModels = flag.Int("content-model-oracle.models", 2_000, "generated content models TestContentModelValidationMatchesReferenceLanguage checks")

const (
	oracleTargetNamespace = "urn:t"
	oracleOtherNamespace  = "urn:u"
	oracleUnbounded       = -1
	oracleMaxWord         = 24
	oracleInstanceWord    = 10
	oracleShortWords      = 300
	oracleWalks           = 30
	oracleAmbiguityBudget = 20_000
	oracleMaxLeaves       = 63
)

type oracleName struct {
	namespace, local string
}

func (n oracleName) xml() string {
	switch n.namespace {
	case oracleTargetNamespace:
		return "<t:" + n.local + "/>"
	case oracleOtherNamespace:
		return "<u:" + n.local + "/>"
	default:
		return "<" + n.local + "/>"
	}
}

func (n oracleName) String() string {
	return "{" + n.namespace + "}" + n.local
}

var (
	oracleHead       = oracleName{oracleTargetNamespace, "h"}
	oracleMember     = oracleName{oracleTargetNamespace, "m"}
	oracleUndeclared = oracleName{oracleTargetNamespace, "z"}
	oracleOther      = oracleName{oracleOtherNamespace, "x"}
	oracleAbsent     = oracleName{"", "x"}
)

type oracleTermKind int

const (
	oracleElement oracleTermKind = iota
	oracleHeadRef
	oracleWildcard
	oracleSequence
	oracleChoice
	oracleAll
)

// oracleParticle is a content model kept in the shape of the XSD 1.0
// component model. leaf names a leaf particle in the masks the matcher
// reports; node indexes the matcher's memo.
type oracleParticle struct {
	kind       oracleTermKind
	local      string
	namespace  string
	constraint []string
	min, max   int
	children   []*oracleParticle
	group      int
	named      bool
	leaf       int
	node       int
}

func (p *oracleParticle) accepts(n oracleName) bool {
	switch p.kind {
	case oracleElement:
		return n == oracleName{p.namespace, p.local}
	case oracleHeadRef:
		return n == oracleHead || n == oracleMember
	case oracleWildcard:
		for _, token := range p.constraint {
			switch token {
			case "##any":
				return true
			case "##other":
				return n.namespace != "" && n.namespace != oracleTargetNamespace
			case "##local":
				token = ""
			case "##targetNamespace":
				token = oracleTargetNamespace
			}
			if n.namespace == token {
				return true
			}
		}
	case oracleSequence, oracleChoice, oracleAll:
	}
	return false
}

// matchesNothing reports whether no sequence is valid against p, as for a
// required choice with no members (I-042 of docs/spec/INTERPRETATIONS.md).
func (p *oracleParticle) matchesNothing() bool {
	if p.max == 0 || p.min == 0 {
		return false
	}
	switch p.kind {
	case oracleSequence:
		for _, child := range p.children {
			if child.matchesNothing() {
				return true
			}
		}
	case oracleChoice:
		for _, child := range p.children {
			if child.max != 0 && !child.matchesNothing() {
				return false
			}
		}
		return true
	case oracleElement, oracleHeadRef, oracleWildcard, oracleAll:
	}
	return false
}

func (p *oracleParticle) holdsParticleMatchingNothing() bool {
	if p.max == 0 {
		return false
	}
	if p.matchesNothing() {
		return true
	}
	for _, child := range p.children {
		if child.holdsParticleMatchingNothing() {
			return true
		}
	}
	return false
}

// oracleSpans maps each end position of a match to the leaves that consumed
// the probe item on the way there; oracleProbeUnseen marks a match that did
// not cover the probe.
type oracleSpans [oracleMaxWord + 1]uint64

const oracleProbeUnseen = uint64(1) << 63

// oracleMatcher matches a content model against a word. A particle with
// maxOccurs 0 corresponds to no component (XSD 1.0 §3.3.2, §3.7.2, §3.8.2), so
// it is no branch of a choice and matches the empty sequence elsewhere. In
// prefix mode a term that starts after the last item matches, standing for
// the items still to come.
type oracleMatcher struct {
	model     *oracleModel
	word      []oracleName
	probe     int
	prefix    bool
	particles []oracleSpans
	terms     []oracleSpans
	done      []uint8
}

const (
	oracleParticleDone uint8 = 1 << iota
	oracleTermDone
)

func newOracleMatcher(model *oracleModel) *oracleMatcher {
	size := model.nodes * (oracleMaxWord + 1)
	return &oracleMatcher{
		model:     model,
		particles: make([]oracleSpans, size),
		terms:     make([]oracleSpans, size),
		done:      make([]uint8, size),
	}
}

// match returns the leaves that consume the probe item in matches of the
// whole word, with oracleProbeUnseen set when the probe is past the word, or
// zero when no match exists.
func (m *oracleMatcher) match(word []oracleName, probe int, prefix bool) uint64 {
	m.word, m.probe, m.prefix = word, probe, prefix
	clear(m.done)
	spans := m.particle(m.model.root, 0)
	return spans[len(word)]
}

func (m *oracleMatcher) accepts(word []oracleName) bool {
	return m.match(word, len(word), false) != 0
}

// prefixLeaves returns the leaves that can consume the last item of a word
// that some valid sequence starts with, or zero when none starts with it.
func (m *oracleMatcher) prefixLeaves(word []oracleName) uint64 {
	return m.match(word, len(word)-1, true) &^ oracleProbeUnseen
}

func (m *oracleMatcher) particle(p *oracleParticle, start int) oracleSpans {
	key := p.node*(oracleMaxWord+1) + start
	if m.done[key]&oracleParticleDone != 0 {
		return m.particles[key]
	}
	var out oracleSpans
	if p.min == 0 || p.max == 0 {
		out[start] = oracleProbeUnseen
	}
	limit := p.max
	if limit == oracleUnbounded {
		limit = p.min + len(m.word)
	}
	var from oracleSpans
	from[start] = oracleProbeUnseen
	for n := 1; n <= limit; n++ {
		from = m.then(from, func(at int) oracleSpans { return m.term(p, at) })
		if n >= p.min {
			for end := range out {
				out[end] |= from[end]
			}
		}
	}
	m.particles[key] = out
	m.done[key] |= oracleParticleDone
	return out
}

func (m *oracleMatcher) then(from oracleSpans, next func(int) oracleSpans) oracleSpans {
	var out oracleSpans
	for mid := range len(m.word) + 1 {
		before := from[mid]
		if before == 0 {
			continue
		}
		after := next(mid)
		for end := mid; end <= len(m.word); end++ {
			switch {
			case after[end] == 0:
			case mid > m.probe:
				out[end] |= before
			default:
				out[end] |= after[end]
			}
		}
	}
	return out
}

func (m *oracleMatcher) term(p *oracleParticle, start int) oracleSpans {
	key := p.node*(oracleMaxWord+1) + start
	if m.done[key]&oracleTermDone != 0 {
		return m.terms[key]
	}
	var out oracleSpans
	switch {
	case m.prefix && start == len(m.word):
		out[start] = oracleProbeUnseen
	case p.kind == oracleSequence:
		out[start] = oracleProbeUnseen
		for _, child := range p.children {
			out = m.then(out, func(at int) oracleSpans { return m.particle(child, at) })
		}
	case p.kind == oracleChoice:
		for _, child := range p.children {
			if child.max == 0 {
				continue
			}
			spans := m.particle(child, start)
			for end := start; end <= len(m.word); end++ {
				out[end] |= spans[end]
			}
		}
	case p.kind == oracleAll:
		m.all(p.children, 0, start, oracleProbeUnseen, &out)
	case start < len(m.word) && p.accepts(m.word[start]):
		out[start+1] = oracleProbeUnseen
		if start == m.probe {
			out[start+1] = 1 << p.leaf
		}
	}
	m.terms[key] = out
	m.done[key] |= oracleTermDone
	return out
}

func (m *oracleMatcher) all(children []*oracleParticle, used uint64, at int, leaves uint64, out *oracleSpans) {
	complete := true
	for i, child := range children {
		if used&(1<<i) == 0 && child.min > 0 && child.max > 0 {
			complete = false
		}
	}
	if complete || m.prefix && at == len(m.word) {
		out[at] |= leaves
	}
	if at == len(m.word) {
		return
	}
	for i, child := range children {
		if used&(1<<i) != 0 || child.max == 0 || !child.accepts(m.word[at]) {
			continue
		}
		next := leaves
		if at == m.probe {
			next = 1 << child.leaf
		}
		m.all(children, used|1<<i, at+1, next, out)
	}
}

// oracleModel is the content type of the root element: the particle of a
// complex type, or a base type's particle followed by an extension's.
type oracleModel struct {
	base, extension *oracleParticle
	root            *oracleParticle
	nodes           int
}

func newOracleModel(base, extension *oracleParticle) *oracleModel {
	model := &oracleModel{base: base, extension: extension, root: base}
	if extension != nil {
		model.root = &oracleParticle{kind: oracleSequence, min: 1, max: 1, children: []*oracleParticle{base, extension}}
	}
	var number func(*oracleParticle)
	number = func(p *oracleParticle) {
		p.node = model.nodes
		model.nodes++
		for _, child := range p.children {
			number(child)
		}
	}
	number(model.root)
	return model
}

func (model *oracleModel) alphabet() []oracleName {
	seen := map[oracleName]bool{oracleUndeclared: true}
	alphabet := []oracleName{oracleUndeclared}
	add := func(n oracleName) {
		if !seen[n] {
			seen[n] = true
			alphabet = append(alphabet, n)
		}
	}
	var walk func(*oracleParticle)
	walk = func(p *oracleParticle) {
		switch p.kind {
		case oracleElement:
			add(oracleName{p.namespace, p.local})
		case oracleHeadRef:
			add(oracleHead)
			add(oracleMember)
		case oracleWildcard:
			add(oracleOther)
			add(oracleAbsent)
		case oracleSequence, oracleChoice, oracleAll:
		}
		for _, child := range p.children {
			walk(child)
		}
	}
	walk(model.root)
	return alphabet
}

func oracleExtend(word []oracleName, n oracleName) []oracleName {
	return append(append(make([]oracleName, 0, len(word)+1), word...), n)
}

// ambiguity searches the words that some valid sequence starts with, shortest
// first, for one whose last item two leaves can consume, which Unique Particle
// Attribution forbids, and returns the first it finds. complete reports
// whether the search covered every such word, which it does only when none
// is longer than oracleMaxWord items and there are at most budget of them.
func (m *oracleMatcher) ambiguity(alphabet []oracleName, budget int) (witness []oracleName, complete bool) {
	frontier := [][]oracleName{{}}
	for len(frontier) != 0 {
		var next [][]oracleName
		for _, w := range frontier {
			for _, n := range alphabet {
				if budget == 0 {
					return nil, false
				}
				budget--
				word := oracleExtend(w, n)
				leaves := m.prefixLeaves(word)
				if bits.OnesCount64(leaves) > 1 {
					return word, true
				}
				if leaves == 0 {
					continue
				}
				if len(word) == oracleMaxWord {
					return nil, false
				}
				next = append(next, word)
			}
		}
		frontier = next
	}
	return nil, true
}

// instanceWords returns about oracleShortWords words, every word up to some
// length, followed by random walks along words that some valid sequence starts
// with, each step adding the walk's word and one arbitrary extension of it.
// It also returns a word whose last item two leaves can consume, if it met
// one.
func (m *oracleMatcher) instanceWords(alphabet []oracleName, rng *rand.Rand) (words [][]oracleName, ambiguous []oracleName) {
	viable := func(word []oracleName) bool {
		leaves := m.prefixLeaves(word)
		if bits.OnesCount64(leaves) > 1 && ambiguous == nil {
			ambiguous = word
		}
		return leaves != 0
	}
	words = [][]oracleName{{}}
	for frontier := words; len(frontier) != 0 && len(words) < oracleShortWords; {
		var next [][]oracleName
		for _, w := range frontier {
			for _, n := range alphabet {
				word := oracleExtend(w, n)
				words = append(words, word)
				if viable(word) && len(word) < oracleInstanceWord {
					next = append(next, word)
				}
			}
		}
		frontier = next
	}
	for range oracleWalks {
		var walk []oracleName
		for len(walk) < oracleInstanceWord-1 {
			var options []oracleName
			for _, n := range alphabet {
				if viable(oracleExtend(walk, n)) {
					options = append(options, n)
				}
			}
			if len(options) == 0 {
				break
			}
			walk = oracleExtend(walk, options[rng.IntN(len(options))])
			words = append(words, walk, oracleExtend(walk, alphabet[rng.IntN(len(alphabet))]))
		}
	}
	return words, ambiguous
}

func (model *oracleModel) schema() string {
	var groups, doc strings.Builder
	var render func(*strings.Builder, *oracleParticle)
	render = func(b *strings.Builder, p *oracleParticle) {
		occurs := ""
		if p.min != 1 {
			occurs += fmt.Sprintf(` minOccurs="%d"`, p.min)
		}
		switch p.max {
		case 1:
		case oracleUnbounded:
			occurs += ` maxOccurs="unbounded"`
		default:
			occurs += fmt.Sprintf(` maxOccurs="%d"`, p.max)
		}
		switch p.kind {
		case oracleElement:
			if p.namespace == "" {
				occurs += ` form="unqualified"`
			}
			fmt.Fprintf(b, `<xs:element name="%s"%s/>`, p.local, occurs)
		case oracleHeadRef:
			fmt.Fprintf(b, `<xs:element ref="t:h"%s/>`, occurs)
		case oracleWildcard:
			fmt.Fprintf(b, `<xs:any namespace="%s" processContents="skip"%s/>`, strings.Join(p.constraint, " "), occurs)
		case oracleSequence, oracleChoice, oracleAll:
			tag := map[oracleTermKind]string{oracleSequence: "sequence", oracleChoice: "choice", oracleAll: "all"}[p.kind]
			var body strings.Builder
			for _, child := range p.children {
				render(&body, child)
			}
			if p.named {
				name := fmt.Sprintf("g%d", p.group)
				fmt.Fprintf(&groups, `<xs:group name="%s"><xs:%s>%s</xs:%s></xs:group>`, name, tag, body.String(), tag)
				fmt.Fprintf(b, `<xs:group ref="t:%s"%s/>`, name, occurs)
				return
			}
			fmt.Fprintf(b, `<xs:%s%s>%s</xs:%s>`, tag, occurs, body.String(), tag)
		}
	}
	var base, extension strings.Builder
	render(&base, model.base)
	if model.extension == nil {
		fmt.Fprintf(&doc, `<xs:element name="root"><xs:complexType>%s</xs:complexType></xs:element>`, base.String())
	} else {
		render(&extension, model.extension)
		fmt.Fprintf(&doc, `<xs:complexType name="base">%s</xs:complexType>`, base.String())
		fmt.Fprintf(&doc, `<xs:element name="root"><xs:complexType><xs:complexContent><xs:extension base="t:base">%s</xs:extension></xs:complexContent></xs:complexType></xs:element>`, extension.String())
	}
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">` +
		`<xs:element name="h"/><xs:element name="m" substitutionGroup="t:h"/>` + groups.String() + doc.String() + `</xs:schema>`
}

func oracleInstance(word []oracleName) string {
	var b strings.Builder
	b.WriteString(`<t:root xmlns:t="urn:t" xmlns:u="urn:u">`)
	for _, n := range word {
		b.WriteString(n.xml())
	}
	b.WriteString(`</t:root>`)
	return b.String()
}

type oracleGenerator struct {
	rng    *rand.Rand
	leaves int
	groups int
}

var (
	oracleOccurrences = [][2]int{{1, 1}, {1, 1}, {0, 1}, {0, oracleUnbounded}, {1, oracleUnbounded}, {2, 3}, {0, 2}, {1, 3}, {2, 2}, {0, 0}, {3, oracleUnbounded}}
	oracleConstraints = [][]string{{"##any"}, {"##other"}, {"##local"}, {"##targetNamespace"}, {"urn:u"}, {"##local", "urn:u"}, {"##targetNamespace", "urn:u"}}
	oracleLocalNames  = []string{"a", "b", "c"}
)

func (g *oracleGenerator) nextLeaf() int {
	g.leaves++
	return g.leaves - 1
}

func (g *oracleGenerator) occurs(p *oracleParticle) *oracleParticle {
	o := oracleOccurrences[g.rng.IntN(len(oracleOccurrences))]
	p.min, p.max = o[0], o[1]
	return p
}

func (g *oracleGenerator) element() *oracleParticle {
	switch n := g.rng.IntN(10); {
	case n < 2:
		return g.occurs(&oracleParticle{kind: oracleWildcard, constraint: oracleConstraints[g.rng.IntN(len(oracleConstraints))], leaf: g.nextLeaf()})
	case n < 3:
		return g.occurs(&oracleParticle{kind: oracleHeadRef, leaf: g.nextLeaf()})
	case n < 4:
		return g.occurs(&oracleParticle{kind: oracleElement, local: oracleAbsent.local, leaf: g.nextLeaf()})
	default:
		return g.occurs(&oracleParticle{kind: oracleElement, namespace: oracleTargetNamespace, local: oracleLocalNames[g.rng.IntN(len(oracleLocalNames))], leaf: g.nextLeaf()})
	}
}

func (g *oracleGenerator) modelGroup(depth int) *oracleParticle {
	p := g.occurs(&oracleParticle{kind: []oracleTermKind{oracleSequence, oracleChoice}[g.rng.IntN(2)], named: g.rng.IntN(4) == 0, group: g.groups})
	g.groups++
	for range g.rng.IntN(4) {
		if depth > 0 && g.rng.IntN(3) == 0 {
			p.children = append(p.children, g.modelGroup(depth-1))
		} else {
			p.children = append(p.children, g.element())
		}
	}
	return p
}

func (g *oracleGenerator) allGroup() *oracleParticle {
	p := &oracleParticle{kind: oracleAll, min: g.rng.IntN(2), max: 1}
	for range g.rng.IntN(4) {
		child := &oracleParticle{kind: oracleElement, namespace: oracleTargetNamespace, local: oracleLocalNames[g.rng.IntN(len(oracleLocalNames))], min: g.rng.IntN(2), max: 1, leaf: g.nextLeaf()}
		if g.rng.IntN(5) == 0 {
			child.kind, child.local = oracleHeadRef, ""
		}
		if g.rng.IntN(8) == 0 {
			child.min, child.max = 0, 0
		}
		p.children = append(p.children, child)
	}
	return p
}

// model returns a content model of at most oracleMaxLeaves leaves, so every
// leaf has a bit of the masks oracleSpans holds.
func (g *oracleGenerator) model() *oracleModel {
	for {
		*g = oracleGenerator{rng: g.rng}
		var base, extension *oracleParticle
		switch n := g.rng.IntN(10); {
		case n < 2:
			base = g.allGroup()
		case n < 4:
			base, extension = g.modelGroup(2), g.modelGroup(1)
		default:
			base = g.modelGroup(3)
		}
		if g.leaves <= oracleMaxLeaves {
			return newOracleModel(base, extension)
		}
	}
}

func TestContentModelValidationMatchesReferenceLanguage(t *testing.T) {
	t.Parallel()

	const seed = 0xc0de
	t.Logf("seed %#x, %d models; model i uses rand.NewPCG(seed, i)", seed, *contentModelOracleModels)
	var tally oracleTally
	t.Cleanup(func() {
		t.Logf("%d models validated %d instances; %d rejected for Unique Particle Attribution, of which %d unchecked because a particle matches nothing and %d without a competing pair in the words searched",
			tally.accepted.Load(), tally.instances.Load(), tally.rejected.Load(), tally.unchecked.Load(), tally.unproven.Load())
	})
	for i := range *contentModelOracleModels {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec // Reproducible generated models, not security-sensitive randomness.
			model := (&oracleGenerator{rng: rng}).model()
			checkContentModelLanguage(t, model, rng, &tally)
		})
	}
}

type oracleTally struct {
	accepted, instances, rejected, unchecked, unproven atomic.Int64
}

func checkContentModelLanguage(t *testing.T, model *oracleModel, rng *rand.Rand, tally *oracleTally) {
	t.Helper()
	schema := model.schema()
	alphabet := model.alphabet()
	matcher := newOracleMatcher(model)
	engine, err := xsd.Compile(xsd.Bytes("model.xsd", []byte(schema)))
	if err != nil {
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel {
			t.Fatalf("Compile() error = %v, want nil or %s\nschema: %s", err, xsderrors.CodeSchemaContentModel, schema)
		}
		tally.rejected.Add(1)
		if model.root.holdsParticleMatchingNothing() {
			tally.unchecked.Add(1)
			return
		}
		witness, complete := matcher.ambiguity(alphabet, oracleAmbiguityBudget)
		if witness == nil && !complete {
			tally.unproven.Add(1)
			return
		}
		if witness == nil {
			t.Fatalf("Compile() rejected the content model, but no word that a valid sequence starts with ends in an item two particles can consume\nerror: %v\nschema: %s", err, schema)
		}
		return
	}
	words, ambiguous := matcher.instanceWords(alphabet, rng)
	if ambiguous != nil {
		t.Fatalf("Compile() accepted a content model in which two particles can consume the last item of %v\nschema: %s", ambiguous, schema)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tally.accepted.Add(1)
	tally.instances.Add(int64(len(words)))
	for _, word := range words {
		want := matcher.accepts(word)
		instance := oracleInstance(word)
		if err := session.Validate(strings.NewReader(instance)); (err == nil) != want {
			t.Fatalf("Validate() = %v, want valid = %v\nschema: %s\ninstance: %s", err, want, schema, instance)
		}
	}
}
