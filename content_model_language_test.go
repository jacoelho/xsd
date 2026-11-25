package xsd_test

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"math/bits"
	"math/rand/v2"
	"slices"
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
	oracleInstanceWord    = 10
	oracleShortWords      = 300
	oracleWalks           = 30
	oracleConfigurations  = 100_000
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

// oracleEmptyChoiceNamespace is the namespace of the items empty choices
// consume in the reference model; no instance holds one.
const oracleEmptyChoiceNamespace = "urn:empty-choice"

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
// component model. leaf names a leaf particle in the leaf masks of
// oracleConfiguration.step; node names the particle in residual keys.
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
	if n.namespace == oracleEmptyChoiceNamespace {
		return false
	}
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

// oracleFrame is one step of what a content model still has to match.
type oracleFrame struct {
	kind   oracleFrameKind
	p      *oracleParticle
	lo, hi int
	used   uint64
}

type oracleFrameKind uint8

const (
	// oracleRepeat matches p between lo and hi more times.
	oracleRepeat oracleFrameKind = iota
	// oracleAllRest matches the children of the all group p outside used.
	oracleAllRest
	// oracleTermEnd closes one repetition of a term: reaching it consumed
	// nothing, and the repetition must consume an item.
	oracleTermEnd
)

// oracleResidual is the frames a valid sequence must still match, in order.
type oracleResidual []oracleFrame

func oracleRepeatOf(p *oracleParticle) oracleFrame {
	f := oracleFrame{kind: oracleRepeat, p: p, lo: p.min, hi: p.max}
	if p.termNullable() {
		f.lo = 0
	}
	return f
}

// emptyChoice reports whether p is a choice without branches. A particle with
// maxOccurs 0 corresponds to no component (XSD 1.0 §3.3.2, §3.7.2, §3.8.2),
// so it is no branch of a choice. An empty choice matches no sequence of
// instance items (I-042 of docs/spec/INTERPRETATIONS.md), yet Unique Particle
// Attribution treats it as an element declaration no instance item matches
// (I-052), so the reference model gives it a leaf that consumes an item of
// its own, emptyChoiceItem.
func (p *oracleParticle) emptyChoice() bool {
	if p.kind != oracleChoice {
		return false
	}
	for _, child := range p.children {
		if child.max != 0 {
			return false
		}
	}
	return true
}

// termNullable reports whether one repetition of p can match the empty
// sequence.
func (p *oracleParticle) termNullable() bool {
	switch p.kind {
	case oracleSequence:
		for _, child := range p.children {
			if child.min > 0 && child.max != 0 && !child.termNullable() {
				return false
			}
		}
		return true
	case oracleChoice:
		for _, child := range p.children {
			if child.max != 0 && (child.min == 0 || child.termNullable()) {
				return true
			}
		}
		return false
	case oracleAll:
		for _, child := range p.children {
			if child.min > 0 && child.max != 0 {
				return false
			}
		}
		return true
	case oracleElement, oracleHeadRef, oracleWildcard:
	}
	return false
}

func (r oracleResidual) nullable() bool {
	for _, f := range r {
		if f.kind == oracleAllRest && !f.allComplete() || f.kind == oracleRepeat && f.lo > 0 {
			return false
		}
	}
	return true
}

func (f oracleFrame) allComplete() bool {
	for i, child := range f.p.children {
		if f.used&(1<<i) == 0 && child.min > 0 && child.max != 0 {
			return false
		}
	}
	return true
}

func (r oracleResidual) key(b *strings.Builder) {
	for _, f := range r {
		fmt.Fprintf(b, "%d.%d.%d.%d.%d;", f.kind, f.p.node, f.lo, f.hi, f.used)
	}
}

// derive calls emit with each leaf that can consume n first in r and the
// residual that follows it.
func (r oracleResidual) derive(n oracleName, emit func(leaf int, rest oracleResidual)) {
	if len(r) == 0 {
		return
	}
	f, rest := r[0], r[1:]
	switch f.kind {
	case oracleTermEnd:
	case oracleRepeat:
		if f.lo == 0 || f.hi == 0 {
			rest.derive(n, emit)
		}
		if f.hi == 0 {
			return
		}
		next := f
		next.lo = max(f.lo-1, 0)
		if f.hi != oracleUnbounded {
			next.hi = f.hi - 1
		}
		f.p.deriveTerm(n, append(oracleResidual{{kind: oracleTermEnd}, next}, rest...), emit)
	case oracleAllRest:
		for i, child := range f.p.children {
			if f.used&(1<<i) == 0 && child.max != 0 && child.accepts(n) {
				next := f
				next.used |= 1 << i
				emit(child.leaf, append(oracleResidual{next}, rest...).withoutTermEnds())
			}
		}
		if f.allComplete() {
			rest.derive(n, emit)
		}
	}
}

func (p *oracleParticle) deriveTerm(n oracleName, then oracleResidual, emit func(leaf int, rest oracleResidual)) {
	switch p.kind {
	case oracleSequence:
		r := make(oracleResidual, 0, len(p.children)+len(then))
		for _, child := range p.children {
			r = append(r, oracleRepeatOf(child))
		}
		append(r, then...).derive(n, emit)
	case oracleChoice:
		if p.emptyChoice() && n == p.emptyChoiceItem() {
			emit(p.leaf, then.withoutTermEnds())
		}
		for _, child := range p.children {
			if child.max != 0 {
				append(oracleResidual{oracleRepeatOf(child)}, then...).derive(n, emit)
			}
		}
	case oracleAll:
		append(oracleResidual{{kind: oracleAllRest, p: p}}, then...).derive(n, emit)
	case oracleElement, oracleHeadRef, oracleWildcard:
		if p.accepts(n) {
			emit(p.leaf, then.withoutTermEnds())
		}
	}
}

func (r oracleResidual) withoutTermEnds() oracleResidual {
	out := make(oracleResidual, 0, len(r))
	for _, f := range r {
		if f.kind != oracleTermEnd {
			out = append(out, f)
		}
	}
	return out
}

// oracleConfiguration is the residuals of every way a model can match a
// word, keyed by their frames.
type oracleConfiguration map[string]oracleResidual

func (model *oracleModel) start() oracleConfiguration {
	c := oracleConfiguration{}
	c.add(oracleResidual{oracleRepeatOf(model.root)})
	return c
}

func (c oracleConfiguration) add(r oracleResidual) {
	var b strings.Builder
	r.key(&b)
	c[b.String()] = r
}

// step returns the configuration after n and the leaves that can consume n.
// Both are empty when no word of the model continues with n.
func (c oracleConfiguration) step(n oracleName) (next oracleConfiguration, leaves uint64) {
	next = oracleConfiguration{}
	for _, r := range c {
		r.derive(n, func(leaf int, rest oracleResidual) {
			leaves |= 1 << leaf
			next.add(rest)
		})
	}
	return next, leaves
}

func (c oracleConfiguration) accepting() bool {
	for _, r := range c {
		if r.nullable() {
			return true
		}
	}
	return false
}

func (c oracleConfiguration) key() string {
	return strings.Join(slices.Sorted(maps.Keys(c)), "|")
}

func (model *oracleModel) accepts(word []oracleName) bool {
	c := model.start()
	for _, n := range word {
		c, _ = c.step(n)
	}
	return c.accepting()
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

func (p *oracleParticle) emptyChoiceItem() oracleName {
	return oracleName{oracleEmptyChoiceNamespace, strconv.Itoa(p.leaf)}
}

// emptyChoiceItems returns the item of each empty choice of the model.
func (model *oracleModel) emptyChoiceItems() []oracleName {
	var items []oracleName
	var walk func(*oracleParticle)
	walk = func(p *oracleParticle) {
		if p.emptyChoice() {
			items = append(items, p.emptyChoiceItem())
		}
		for _, child := range p.children {
			walk(child)
		}
	}
	walk(model.root)
	return items
}

func oracleExtend(word []oracleName, n oracleName) []oracleName {
	return append(append(make([]oracleName, 0, len(word)+1), word...), n)
}

// ambiguity searches the configurations words over items reach, each once
// and shortest word first, for an item two leaves can consume, which Unique
// Particle Attribution forbids, and returns the first word that ends in one.
// complete reports whether the search visited every configuration within
// limit of them.
func (model *oracleModel) ambiguity(items []oracleName, limit int) (witness []oracleName, complete bool) {
	type visit struct {
		configuration oracleConfiguration
		word          []oracleName
	}
	start := model.start()
	seen := map[string]bool{start.key(): true}
	queue := []visit{{configuration: start}}
	for len(queue) != 0 {
		v := queue[0]
		queue = queue[1:]
		for _, n := range items {
			next, leaves := v.configuration.step(n)
			word := oracleExtend(v.word, n)
			if bits.OnesCount64(leaves) > 1 {
				return word, true
			}
			if len(next) == 0 {
				continue
			}
			key := next.key()
			if seen[key] {
				continue
			}
			if len(seen) == limit {
				return nil, false
			}
			seen[key] = true
			queue = append(queue, visit{configuration: next, word: word})
		}
	}
	return nil, true
}

// liveConfigurations returns the keys of the configurations words over
// alphabet reach from the start that some word over alphabet continues to a
// valid sequence. complete reports whether there are at most limit
// configurations to visit.
func (model *oracleModel) liveConfigurations(alphabet []oracleName, limit int) (live map[string]bool, complete bool) {
	start := model.start()
	successors := map[string][]string{}
	live = map[string]bool{}
	queue := []oracleConfiguration{start}
	successors[start.key()] = nil
	for len(queue) != 0 {
		c := queue[0]
		queue = queue[1:]
		key := c.key()
		live[key] = c.accepting()
		for _, n := range alphabet {
			next, _ := c.step(n)
			if len(next) == 0 {
				continue
			}
			nextKey := next.key()
			successors[key] = append(successors[key], nextKey)
			if _, seen := successors[nextKey]; seen {
				continue
			}
			if len(successors) == limit {
				return nil, false
			}
			successors[nextKey] = nil
			queue = append(queue, next)
		}
	}
	for changed := true; changed; {
		changed = false
		for key, next := range successors {
			if live[key] {
				continue
			}
			for _, k := range next {
				if live[k] {
					live[key], changed = true, true
					break
				}
			}
		}
	}
	return live, true
}

// instanceWords returns about oracleShortWords words over alphabet: every
// word, up to some length, whose proper prefixes some word over alphabet
// continues to a valid sequence (whose configurations live holds), followed
// by random walks along such prefixes, each step adding the walk's word and
// one arbitrary extension of it.
func (model *oracleModel) instanceWords(alphabet []oracleName, live map[string]bool, rng *rand.Rand) [][]oracleName {
	type prefix struct {
		word          []oracleName
		configuration oracleConfiguration
	}
	continues := func(c oracleConfiguration) bool {
		return len(c) != 0 && live[c.key()]
	}
	words := [][]oracleName{{}}
	for frontier := []prefix{{configuration: model.start()}}; len(frontier) != 0 && len(words) < oracleShortWords; {
		var next []prefix
		for _, p := range frontier {
			for _, n := range alphabet {
				word := oracleExtend(p.word, n)
				words = append(words, word)
				if c, _ := p.configuration.step(n); continues(c) && len(word) < oracleInstanceWord {
					next = append(next, prefix{word: word, configuration: c})
				}
			}
		}
		frontier = next
	}
	for range oracleWalks {
		c := model.start()
		var walk []oracleName
		for len(walk) < oracleInstanceWord-1 {
			var options []prefix
			for _, n := range alphabet {
				if next, _ := c.step(n); continues(next) {
					options = append(options, prefix{word: oracleExtend(walk, n), configuration: next})
				}
			}
			if len(options) == 0 {
				break
			}
			chosen := options[rng.IntN(len(options))]
			walk, c = chosen.word, chosen.configuration
			words = append(words, walk, oracleExtend(walk, alphabet[rng.IntN(len(alphabet))]))
		}
	}
	return words
}

func (model *oracleModel) schema() string {
	var groups, doc strings.Builder
	defined := map[int]bool{}
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
				if !defined[p.group] {
					defined[p.group] = true
					fmt.Fprintf(&groups, `<xs:group name="%s"><xs:%s>%s</xs:%s></xs:group>`, name, tag, body.String(), tag)
				}
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

// oracleGenerator generates content models. With costShapes it also
// generates the shapes TestChoiceWithoutBranchesCostsNoMoreThanItsElementTwin
// compares: content that is a choice without branches, several of them in a
// choice, which may also hold many distinct elements, and counts up to 20,000
// on them.
type oracleGenerator struct {
	rng        *rand.Rand
	costShapes bool
	leaves     int
	groups     int
	named      []*oracleParticle
}

var (
	oracleOccurrences      = [][2]int{{1, 1}, {1, 1}, {0, 1}, {0, oracleUnbounded}, {1, oracleUnbounded}, {2, 3}, {0, 2}, {1, 3}, {2, 2}, {0, 0}, {3, oracleUnbounded}}
	oracleEmptyChoiceCosts = [][2]int{{1, 1}, {0, 1}, {2, 20_000}, {300, 300}, {20_000, 20_000}, {3, oracleUnbounded}, {0, 20_000}}
	oracleConstraints      = [][]string{{"##any"}, {"##other"}, {"##local"}, {"##targetNamespace"}, {"urn:u"}, {"##local", "urn:u"}, {"##targetNamespace", "urn:u"}}
	oracleLocalNames       = []string{"a", "b", "c"}
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
	if len(g.named) != 0 && g.rng.IntN(4) == 0 {
		return g.occurs(g.copyWithOwnLeaves(g.named[g.rng.IntN(len(g.named))]))
	}
	p := g.occurs(&oracleParticle{kind: []oracleTermKind{oracleSequence, oracleChoice}[g.rng.IntN(2)], named: g.rng.IntN(4) == 0, group: g.groups})
	g.groups++
	for range g.rng.IntN(4) {
		if depth > 0 && g.rng.IntN(3) == 0 {
			p.children = append(p.children, g.modelGroup(depth-1))
		} else {
			p.children = append(p.children, g.element())
		}
	}
	if g.costShapes && p.kind == oracleChoice && g.rng.IntN(3) == 0 {
		if g.rng.IntN(2) == 0 {
			for range 20 + g.rng.IntN(20) {
				p.children = append(p.children, &oracleParticle{kind: oracleElement, namespace: oracleTargetNamespace, local: "w" + strconv.Itoa(g.leaves), min: 1, max: 1, leaf: g.nextLeaf()})
			}
		}
		for range 1 + g.rng.IntN(3) {
			p.children = append(p.children, g.choiceWithoutBranches())
		}
	}
	if p.emptyChoice() {
		p.leaf = g.nextLeaf()
		if g.costShapes {
			g.emptyChoiceCost(p)
		}
	}
	if p.named {
		g.named = append(g.named, p)
	}
	return p
}

// copyWithOwnLeaves returns a copy of p with leaves of its own. Another
// reference to a named group holds the group's particles again, and Unique
// Particle Attribution compares them as particles distinct from those of
// every other reference (I-028 of docs/spec/INTERPRETATIONS.md).
func (g *oracleGenerator) copyWithOwnLeaves(p *oracleParticle) *oracleParticle {
	c := *p
	c.children = make([]*oracleParticle, 0, len(p.children))
	for _, child := range p.children {
		c.children = append(c.children, g.copyWithOwnLeaves(child))
	}
	switch {
	case c.kind == oracleElement, c.kind == oracleHeadRef, c.kind == oracleWildcard, c.emptyChoice():
		c.leaf = g.nextLeaf()
	}
	return &c
}

func (g *oracleGenerator) choiceWithoutBranches() *oracleParticle {
	p := &oracleParticle{kind: oracleChoice, group: g.groups, leaf: g.nextLeaf()}
	g.groups++
	g.emptyChoiceCost(p)
	return p
}

func (g *oracleGenerator) emptyChoiceCost(p *oracleParticle) {
	o := oracleEmptyChoiceCosts[g.rng.IntN(len(oracleEmptyChoiceCosts))]
	p.min, p.max = o[0], o[1]
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
// leaf has a bit of the leaf masks oracleConfiguration.step returns.
func (g *oracleGenerator) model() *oracleModel {
	for {
		*g = oracleGenerator{rng: g.rng, costShapes: g.costShapes}
		var base, extension *oracleParticle
		switch n := g.rng.IntN(10); {
		case g.costShapes && n < 1:
			base = g.choiceWithoutBranches()
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
		t.Logf("%d models validated %d instances; %d rejected for Unique Particle Attribution",
			tally.accepted.Load(), tally.instances.Load(), tally.rejected.Load())
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
	accepted, instances, rejected atomic.Int64
}

func checkContentModelLanguage(t *testing.T, model *oracleModel, rng *rand.Rand, tally *oracleTally) {
	t.Helper()
	schema := model.schema()
	alphabet := model.alphabet()
	witness, complete := model.ambiguity(append(slices.Clip(alphabet), model.emptyChoiceItems()...), oracleConfigurations)
	if !complete {
		t.Fatalf("the reference model reaches more than %d configurations\nschema: %s", oracleConfigurations, schema)
	}
	engine, err := xsd.Compile(xsd.Bytes("model.xsd", []byte(schema)))
	if err != nil {
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel || !strings.Contains(diagnostic.Message(), "UPA violation") {
			t.Fatalf("Compile() error = %v, want nil or a %s UPA violation\nschema: %s", err, xsderrors.CodeSchemaContentModel, schema)
		}
		tally.rejected.Add(1)
		if witness == nil {
			t.Fatalf("Compile() rejected the content model, but no prefix of a word of the model ends in an item two particles can consume\nerror: %v\nschema: %s", err, schema)
		}
		return
	}
	if witness != nil {
		t.Fatalf("Compile() accepted a content model in which two particles can consume the last item of %v\nschema: %s", witness, schema)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	live, complete := model.liveConfigurations(alphabet, oracleConfigurations)
	if !complete {
		t.Fatalf("words over the instance alphabet reach more than %d configurations\nschema: %s", oracleConfigurations, schema)
	}
	words := model.instanceWords(alphabet, live, rng)
	tally.accepted.Add(1)
	tally.instances.Add(int64(len(words)))
	for _, word := range words {
		want := model.accepts(word)
		instance := oracleInstance(word)
		err := session.Validate(strings.NewReader(instance))
		if want {
			if err != nil {
				t.Fatalf("Validate() = %v, want valid\nschema: %s\ninstance: %s", err, schema, instance)
			}
			continue
		}
		if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Category() != xsderrors.CategoryValidation {
			t.Fatalf("Validate() = %v, want a %s diagnostic\nschema: %s\ninstance: %s", err, xsderrors.CategoryValidation, schema, instance)
		}
	}
}
