package xsd_test

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// The tests in this file enforce the schema document diagnostics order that
// ARCHITECTURE.md states for internal/schema. A violation has a decision
// point: the start tag or the end tag of one element. Parse-tier violations
// are reported in document order of their decision points and, at one
// decision point, in stage order; compile-tier violations are reported only
// when no document has a parse-tier violation.
//
// The catalogue in schema_diagnostic_catalogue_test.go declares each
// violation's tier and stage from that contract. Every expected winner below
// is computed from those declarations and the position of the deciding tag in
// the generated document, never from compiler output.

type diagnosticTier uint8

const (
	parseTier diagnosticTier = iota + 1
	compileTier
)

// The stages of one decision point. Stages up to stageExcludedAttributes are
// decided at a start tag, the others at an end tag. Contract stage 6 decides
// the attributes a tag requires and then the ones that exclude each other.
const (
	stageRoot = iota + 1
	stageVocabulary
	stageEarlierText
	stageChild
	stageAttribute
	stageRequiredAttribute
	stageExcludedAttributes
	stageText
	stageRequiredChild
)

var stageNames = map[int]string{
	stageRoot:               "1 root element",
	stageVocabulary:         "2 unsupported vocabulary",
	stageEarlierText:        "3 earlier character data",
	stageChild:              "4 child admission",
	stageAttribute:          "5 attribute",
	stageRequiredAttribute:  "6 required attribute",
	stageExcludedAttributes: "6 excluded attributes",
	stageText:               "7 character data",
	stageRequiredChild:      "8 required child",
}

const (
	schemaRootKey     = "schema"
	schemaNamespace   = "http://www.w3.org/2001/XMLSchema"
	characterData     = "text"
	firstDocumentName = "a.xsd"
	laterDocumentName = "b.xsd"
)

// node is one element of a generated schema document. Each start tag is
// rendered on its own line, so a diagnostic's position identifies its node.
type node struct {
	key      string
	tag      string
	lead     string
	tail     string
	attrs    []string
	children []*node
	line     int
	column   int
	start    int
	end      int
}

func xs(local string, attrs ...string) *node {
	return &node{tag: "xs:" + local, attrs: attrs}
}

func (n *node) as(key string) *node {
	n.key = key
	return n
}

func (n *node) with(children ...*node) *node {
	n.children = append(n.children, children...)
	return n
}

func newSchemaRoot() *node {
	return xs("schema", `xmlns:xs="`+schemaNamespace+`"`, `xmlns:f="urn:foreign"`).as(schemaRootKey)
}

func attributeName(attr string) string {
	return attr[:strings.IndexByte(attr, '=')]
}

func (n *node) attributeIndex(name string) int {
	return slices.IndexFunc(n.attrs, func(attr string) bool { return attributeName(attr) == name })
}

// setAttribute writes attr in place of an attribute of the same name, or
// after the last attribute when there is none.
func (n *node) setAttribute(attr string) {
	if i := n.attributeIndex(attributeName(attr)); i >= 0 {
		n.attrs[i] = attr
		return
	}
	n.attrs = append(n.attrs, attr)
}

func (n *node) find(key string) *node {
	if n.key == key {
		return n
	}
	for _, child := range n.children {
		if found := child.find(key); found != nil {
			return found
		}
	}
	return nil
}

func (n *node) parentOf(target *node) *node {
	for _, child := range n.children {
		if child == target {
			return n
		}
		if parent := child.parentOf(target); parent != nil {
			return parent
		}
	}
	return nil
}

func (n *node) contains(target *node) bool {
	return n == target || n.parentOf(target) != nil
}

type documentRenderer struct {
	text strings.Builder
	line int
	rank int
}

// renderSchemaDocument renders root and records the position and the
// document-order rank of every start and end tag.
func renderSchemaDocument(root *node) string {
	r := documentRenderer{line: 1}
	r.element(root, 0)
	return r.text.String()
}

func (r *documentRenderer) writeLine(text string) {
	r.text.WriteString(text)
	r.text.WriteByte('\n')
	r.line++
}

func (r *documentRenderer) element(n *node, depth int) {
	indent := strings.Repeat("  ", depth)
	if n.lead != "" {
		r.writeLine(indent + n.lead)
	}
	n.line, n.column = r.line, len(indent)+1
	n.start = r.rank
	r.rank++
	open := indent + "<" + n.tag
	if len(n.attrs) != 0 {
		open += " " + strings.Join(n.attrs, " ")
	}
	if len(n.children) == 0 && n.tail == "" {
		r.writeLine(open + "/>")
	} else {
		r.writeLine(open + ">")
		for _, child := range n.children {
			r.element(child, depth+1)
		}
		if n.tail != "" {
			r.writeLine(indent + "  " + n.tail)
		}
		r.writeLine(indent + "</" + n.tag + ">")
	}
	n.end = r.rank
	r.rank++
}

// fixture is a valid group of top-level components and the single violations
// the catalogue introduces into it. A fixture without build addresses the
// xs:schema element itself.
type fixture struct {
	build      func(n string) []*node
	name       string
	violations []violation
	// prolog places the components before every other fixture's, where
	// xs:schema admits xs:include and xs:import.
	prolog bool
}

// violation is one catalogue entry: a tier and stage transcribed from the
// contract, the diagnostic that identifies it, and the edit that introduces
// it into its fixture.
type violation struct {
	// standalone builds components that already hold the violation, for one
	// that no edit of a valid fixture expresses.
	standalone func(n string) []*node
	// before builds a sibling inserted immediately before the edited element.
	before  func() *node
	name    string
	code    xsderrors.Code
	message string
	// decidedAt is the key of the element whose start or end tag decides the
	// violation and that the diagnostic names; edited defaults to it.
	decidedAt string
	edited    string
	rename    string
	dropAttr  string
	setAttrs  []string
	dropKeys  []string
	stage     int
	tier      diagnosticTier
	leadText  bool
	tailText  bool
	// nowhere marks a diagnostic the compiler reports without a location.
	nowhere bool
}

func parseViolation(stage int, name string, code xsderrors.Code, message string) violation {
	return violation{name: name, tier: parseTier, stage: stage, code: code, message: message}
}

func compileViolation(name string, code xsderrors.Code, message string) violation {
	return violation{name: name, tier: compileTier, code: code, message: message}
}

func (v violation) at(key string) violation {
	v.decidedAt = key
	return v
}

func (v violation) editing(key string) violation {
	v.edited = key
	return v
}

func (v violation) setting(attrs ...string) violation {
	v.setAttrs = attrs
	return v
}

func (v violation) dropping(attr string) violation {
	v.dropAttr = attr
	return v
}

func (v violation) renamed(tag string) violation {
	v.rename = tag
	return v
}

func (v violation) after(sibling func() *node) violation {
	v.before = sibling
	return v
}

func (v violation) without(keys ...string) violation {
	v.dropKeys = keys
	return v
}

func (v violation) unlocated() violation {
	v.nowhere = true
	return v
}

func (v violation) alone(build func(n string) []*node) violation {
	v.standalone = build
	return v
}

func (v violation) editedKey() string {
	if v.edited != "" {
		return v.edited
	}
	return v.decidedAt
}

func (v violation) decidedAtStart() bool {
	return v.stage <= stageExcludedAttributes
}

// entry is one violation of one fixture.
type entry struct {
	violation

	fixture *fixture
}

func (e *entry) fullName() string {
	return e.fixture.name + "/" + e.name
}

func (e *entry) describe() string {
	if e.tier == compileTier {
		return e.fullName() + " (compile tier)"
	}
	return e.fullName() + " (stage " + stageNames[e.stage] + ")"
}

func catalogueEntries() []*entry {
	fixtures := schemaDiagnosticFixtures()
	var entries []*entry
	for i := range fixtures {
		f := &fixtures[i]
		for _, v := range f.violations {
			entries = append(entries, &entry{fixture: f, violation: v})
		}
	}
	return entries
}

// placed is an entry introduced into a document.
type placed struct {
	entry   *entry
	decided *node
	named   *node
}

// schemaDocument is one generated schema document.
type schemaDocument struct {
	root   *node
	prolog int
}

func newSchemaDocument() *schemaDocument {
	return &schemaDocument{root: newSchemaRoot()}
}

// addComponents appends the components of f, built for suffix n, and returns
// the scope that holds its keyed nodes.
func (d *schemaDocument) addComponents(f *fixture, standalone func(n string) []*node, n string) *node {
	build := f.build
	if standalone != nil {
		build = standalone
	}
	if build == nil {
		return d.root
	}
	components := build(n)
	scope := &node{children: components}
	if f.prolog {
		d.root.children = slices.Insert(d.root.children, d.prolog, components...)
		d.prolog += len(components)
		return scope
	}
	d.root.children = append(d.root.children, components...)
	return scope
}

func (d *schemaDocument) lookup(scope *node, key string) *node {
	if key == schemaRootKey {
		return d.root
	}
	return scope.find(key)
}

// introduce applies the edit of e within scope. It reports false when the
// document no longer holds an element the edit addresses.
func (d *schemaDocument) introduce(e *entry, scope *node) (placed, bool) {
	decided := d.lookup(scope, e.decidedAt)
	target := d.lookup(scope, e.editedKey())
	if decided == nil || target == nil {
		return placed{}, false
	}
	named := decided
	for _, attr := range e.setAttrs {
		target.setAttribute(attr)
	}
	if e.dropAttr != "" {
		if i := target.attributeIndex(e.dropAttr); i >= 0 {
			target.attrs = slices.Delete(target.attrs, i, i+1)
		}
	}
	if e.rename != "" {
		target.tag = e.rename
	}
	if e.leadText {
		target.lead = characterData
		named = d.root.parentOf(target)
	}
	if e.tailText {
		target.tail = characterData
	}
	if e.before != nil {
		parent := d.root.parentOf(target)
		parent.children = slices.Insert(parent.children, slices.Index(parent.children, target), e.before())
	}
	if len(e.dropKeys) != 0 {
		target.children = slices.DeleteFunc(target.children, func(child *node) bool {
			return slices.Contains(e.dropKeys, child.key)
		})
	}
	return placed{entry: e, decided: decided, named: named}, true
}

func (d *schemaDocument) holds(p placed) bool {
	return d.root.contains(p.decided) && d.root.contains(p.named)
}

// editsCollide reports whether two violations of one fixture cannot be
// introduced together as two independent violations.
func editsCollide(a, b *entry) bool {
	if a.standalone != nil || b.standalone != nil {
		return true
	}
	if a.editedKey() != b.editedKey() {
		return false
	}
	if a.rename != "" && b.rename != "" || a.before != nil && b.before != nil ||
		a.leadText && b.leadText || a.tailText && b.tailText {
		return true
	}
	if slices.ContainsFunc(a.attributeNames(), func(name string) bool { return slices.Contains(b.attributeNames(), name) }) {
		return true
	}
	return renameVoids(a, b) || renameVoids(b, a)
}

// renameVoids reports whether renaming the element changes what other, another
// edit of the same element, violates: the renamed element has another
// vocabulary and another place in its parent's content. Character data before
// the element is independent of its name, and so is any edit the contract
// reports only after the rename.
func renameVoids(renamed, other *entry) bool {
	if renamed.rename == "" || other.leadText {
		return false
	}
	return renamed.tier == compileTier || other.tier == parseTier && other.stage <= renamed.stage
}

func (v violation) attributeNames() []string {
	names := make([]string, 0, len(v.setAttrs)+1)
	for _, attr := range v.setAttrs {
		names = append(names, attributeName(attr))
	}
	if v.dropAttr != "" {
		names = append(names, v.dropAttr)
	}
	return names
}

func (p placed) decisionRank() int {
	if p.entry.decidedAtStart() {
		return p.decided.start
	}
	return p.decided.end
}

// attributeRank is the start-tag position of the attribute that p writes, or
// -1 when p is not an attribute written on its deciding start tag.
func (p placed) attributeRank() int {
	if len(p.entry.setAttrs) != 1 || p.entry.editedKey() != p.entry.decidedAt {
		return -1
	}
	return p.decided.attributeIndex(attributeName(p.entry.setAttrs[0]))
}

func (p placed) indistinguishable(q placed) bool {
	return p.named == q.named && p.entry.code == q.entry.code && p.entry.message == q.entry.message
}

// expectedWinner returns which of two violations of one rendered document the
// contract reports, and false when the contract does not order them.
func expectedWinner(a, b placed) (placed, bool) {
	switch {
	case a.entry.tier == compileTier && b.entry.tier == compileTier:
		return placed{}, false
	case b.entry.tier == compileTier:
		return a, true
	case a.entry.tier == compileTier:
		return b, true
	}
	if a.decisionRank() != b.decisionRank() {
		return earlier(a, b, a.decisionRank(), b.decisionRank()), true
	}
	if a.entry.stage != b.entry.stage {
		return earlier(a, b, a.entry.stage, b.entry.stage), true
	}
	if a.entry.stage != stageAttribute && a.entry.stage != stageVocabulary {
		return placed{}, false
	}
	if a.attributeRank() < 0 || b.attributeRank() < 0 {
		return placed{}, false
	}
	return earlier(a, b, a.attributeRank(), b.attributeRank()), true
}

func earlier(a, b placed, rankA, rankB int) placed {
	if rankA < rankB {
		return a
	}
	return b
}

type compileOutcome struct {
	diagnostic *xsderrors.Error
	err        error
}

func compileSchemaDocuments(documents map[string]string) compileOutcome {
	names := make([]string, 0, len(documents))
	for name := range documents {
		names = append(names, name)
	}
	sort.Strings(names)
	sources := make([]xsd.SchemaSource, 0, len(names))
	for _, name := range names {
		sources = append(sources, xsd.Bytes(name, []byte(documents[name])))
	}
	_, err := xsd.Compile(sources...)
	diagnostic, _ := errors.AsType[*xsderrors.Error](err)
	return compileOutcome{diagnostic: diagnostic, err: err}
}

func (o compileOutcome) String() string {
	switch {
	case o.err == nil:
		return "no diagnostic"
	case o.diagnostic == nil:
		return "unstructured error: " + o.err.Error()
	}
	d := o.diagnostic
	return fmt.Sprintf("(%s, %q) at %s:%d:%d", d.Code(), d.Message(), d.Path(), d.Line(), d.Column())
}

// identifies reports whether the diagnostic is the one that identifies e,
// wherever it is located.
func (o compileOutcome) identifies(e *entry) bool {
	d := o.diagnostic
	return d != nil && d.Code() == e.code && strings.Contains(d.Message(), e.message)
}

func (o compileOutcome) reports(p placed, path string) bool {
	d := o.diagnostic
	if !o.identifies(p.entry) {
		return false
	}
	if p.entry.nowhere {
		return d.Path() == "" && d.Line() == 0 && d.Column() == 0
	}
	return d.Path() == path && d.Line() == p.named.line && d.Column() == p.named.column
}

func (p placed) want(path string) string {
	if p.entry.nowhere {
		return fmt.Sprintf("(%s, %q) without a location, from %s", p.entry.code, p.entry.message, p.entry.describe())
	}
	return fmt.Sprintf("(%s, %q) at %s:%d:%d, from %s", p.entry.code, p.entry.message, path, p.named.line, p.named.column, p.entry.describe())
}

// orderGaps tracks the known gaps: catalogue entries the compiler decides
// later than the contract declares. A pair whose expected winner is a known
// gap is not asserted; a known gap that wins every such pair is stale.
type orderGaps struct {
	failed map[string]bool
	seen   map[string]bool
}

func newOrderGaps() *orderGaps {
	return &orderGaps{failed: make(map[string]bool), seen: make(map[string]bool)}
}

// excuses reports whether winner, the expected winner of a pair, is a known
// gap, and records whether the compiler reported it all the same.
func (g *orderGaps) excuses(winner placed, outcome compileOutcome, path string) bool {
	name := winner.entry.fullName()
	if _, known := knownSchemaDiagnosticOrderGaps[name]; !known {
		return false
	}
	g.seen[name] = true
	if !outcome.reports(winner, path) {
		g.failed[name] = true
	}
	return true
}

func (g *orderGaps) reportStale(t *testing.T) {
	t.Helper()
	for name := range g.seen {
		if !g.failed[name] {
			t.Errorf("known gap %q is stale: it won every pair the contract says it wins; remove it from knownSchemaDiagnosticOrderGaps", name)
		}
	}
}

func TestSchemaDiagnosticCatalogue(t *testing.T) {
	t.Parallel()
	entries := catalogueEntries()
	names := make(map[string]bool, len(entries))
	stages := make(map[int]int)
	compileEntries := 0
	for _, e := range entries {
		if names[e.fullName()] {
			t.Errorf("catalogue entry %q is declared twice", e.fullName())
		}
		names[e.fullName()] = true
		switch e.tier {
		case parseTier:
			if _, ok := stageNames[e.stage]; !ok {
				t.Errorf("catalogue entry %q has no stage of the contract", e.fullName())
			}
			stages[e.stage]++
		case compileTier:
			compileEntries++
		default:
			t.Errorf("catalogue entry %q has no tier", e.fullName())
		}
	}
	for name := range knownSchemaDiagnosticOrderGaps {
		if !names[name] {
			t.Errorf("known gap %q names no catalogue entry", name)
		}
	}
	for stage, name := range stageNames {
		if stages[stage] < 3 {
			t.Errorf("stage %s has %d catalogue entries, want at least 3", name, stages[stage])
		}
	}
	if compileEntries < 6 {
		t.Errorf("catalogue has %d compile-tier entries, want at least 6", compileEntries)
	}

	fixtures := schemaDiagnosticFixtures()
	for i := range fixtures {
		f := &fixtures[i]
		doc := newSchemaDocument()
		doc.addComponents(f, nil, "a")
		text := renderSchemaDocument(doc.root)
		if outcome := compileSchemaDocuments(map[string]string{firstDocumentName: text}); outcome.err != nil {
			t.Errorf("fixture %q without a violation: got %s, want no diagnostic\n%s", f.name, outcome, text)
		}
	}
	for _, e := range entries {
		doc := newSchemaDocument()
		p, ok := doc.introduce(e, doc.addComponents(e.fixture, e.standalone, "a"))
		if !ok {
			t.Errorf("catalogue entry %q addresses an element its fixture does not hold", e.fullName())
			continue
		}
		text := renderSchemaDocument(doc.root)
		outcome := compileSchemaDocuments(map[string]string{firstDocumentName: text})
		_, knownGap := knownSchemaDiagnosticOrderGaps[e.fullName()]
		if knownGap && outcome.identifies(e) {
			continue
		}
		if !outcome.reports(p, firstDocumentName) {
			t.Errorf("catalogue entry %q alone:\n  got  %s\n  want %s\n%s", e.fullName(), outcome, p.want(firstDocumentName), text)
		}
	}
	t.Logf("catalogue: %d entries in %d fixtures, %d compile-tier", len(entries), len(fixtures), compileEntries)
}

// TestSchemaDiagnosticOrderAcrossElements renders every ordered pair of
// catalogue entries as consecutive top-level components of one document.
func TestSchemaDiagnosticOrderAcrossElements(t *testing.T) {
	t.Parallel()
	entries := catalogueEntries()
	gaps := newOrderGaps()
	asserted, unordered := 0, 0
	for _, first := range entries {
		for _, second := range entries {
			if first.fixture.build == nil && second.fixture.build == nil && first.standalone == nil && second.standalone == nil {
				continue
			}
			doc := newSchemaDocument()
			a, okA := doc.introduce(first, doc.addComponents(first.fixture, first.standalone, "a"))
			b, okB := doc.introduce(second, doc.addComponents(second.fixture, second.standalone, "b"))
			if !okA || !okB {
				t.Fatalf("pair %q then %q cannot be rendered", first.fullName(), second.fullName())
			}
			text := renderSchemaDocument(doc.root)
			winner, ordered := expectedWinner(a, b)
			if !ordered || a.indistinguishable(b) {
				unordered++
				continue
			}
			outcome := compileSchemaDocuments(map[string]string{firstDocumentName: text})
			if gaps.excuses(winner, outcome, firstDocumentName) {
				continue
			}
			asserted++
			if !outcome.reports(winner, firstDocumentName) {
				t.Errorf("order across elements: A=%q then B=%q\n  got  %s\n  want %s\n%s",
					first.fullName(), second.fullName(), outcome, winner.want(firstDocumentName), text)
			}
		}
	}
	gaps.reportStale(t)
	t.Logf("across elements: %d pairs asserted, %d not ordered by the contract", asserted, unordered)
}

// TestSchemaDiagnosticOrderWithinFixture introduces every ordered pair of
// violations of one fixture into the same components: two violations of one
// start tag, a violation of an element and one of its descendant, and
// violations of sibling elements.
func TestSchemaDiagnosticOrderWithinFixture(t *testing.T) {
	t.Parallel()
	fixtures := schemaDiagnosticFixtures()
	gaps := newOrderGaps()
	counts := make(map[string]int)
	sameStartTag := make(map[string]int)
	for i := range fixtures {
		f := &fixtures[i]
		for _, v1 := range f.violations {
			for _, v2 := range f.violations {
				first, second := &entry{fixture: f, violation: v1}, &entry{fixture: f, violation: v2}
				if v1.name == v2.name || editsCollide(first, second) {
					continue
				}
				doc := newSchemaDocument()
				scope := doc.addComponents(f, nil, "a")
				a, okA := doc.introduce(first, scope)
				b, okB := doc.introduce(second, scope)
				if !okA || !okB || !doc.holds(a) || !doc.holds(b) {
					counts["one edit removes the other's element"]++
					continue
				}
				text := renderSchemaDocument(doc.root)
				winner, ordered := expectedWinner(a, b)
				if !ordered || a.indistinguishable(b) {
					counts["not ordered by the contract"]++
					continue
				}
				outcome := compileSchemaDocuments(map[string]string{firstDocumentName: text})
				if gaps.excuses(winner, outcome, firstDocumentName) {
					continue
				}
				kind := pairKind(a, b)
				counts[kind]++
				if kind == sameStartTagPair {
					sameStartTag[f.name]++
				}
				if !outcome.reports(winner, firstDocumentName) {
					t.Errorf("order within fixture (%s): %q then %q\n  got  %s\n  want %s\n%s",
						kind, first.fullName(), second.fullName(), outcome, winner.want(firstDocumentName), text)
				}
			}
		}
	}
	for _, name := range sameStartTagFixtures {
		if sameStartTag[name] == 0 {
			t.Errorf("fixture %q generated no pair of violations on one start tag", name)
		}
	}
	for _, kind := range []string{sameStartTagPair, nestedStartPair, nestedEndPair} {
		if counts[kind] == 0 {
			t.Errorf("no pair of kind %q was generated", kind)
		}
	}
	t.Logf("within fixture: %v", counts)
}

const (
	sameStartTagPair = "same start tag"
	sameEndTagPair   = "same end tag"
	nestedStartPair  = "ancestor start tag and descendant"
	nestedEndPair    = "ancestor end tag and descendant"
	otherPair        = "other elements"
)

func pairKind(a, b placed) string {
	if a.decided == b.decided {
		switch {
		case a.entry.decidedAtStart() && b.entry.decidedAtStart():
			return sameStartTagPair
		case !a.entry.decidedAtStart() && !b.entry.decidedAtStart():
			return sameEndTagPair
		}
		return otherPair
	}
	ancestor, ok := a, a.decided.contains(b.decided)
	if !ok {
		ancestor, ok = b, b.decided.contains(a.decided)
	}
	switch {
	case !ok:
		return otherPair
	case ancestor.entry.decidedAtStart():
		return nestedStartPair
	}
	return nestedEndPair
}

// TestSchemaDiagnosticOrderAcrossDocuments compiles a parse-tier and a
// compile-tier entry as two documents of one set, in both document orders.
func TestSchemaDiagnosticOrderAcrossDocuments(t *testing.T) {
	t.Parallel()
	entries := catalogueEntries()
	gaps := newOrderGaps()
	asserted := 0
	for _, parsed := range entries {
		if parsed.tier != parseTier {
			continue
		}
		for _, compiled := range entries {
			if compiled.tier != compileTier {
				continue
			}
			for _, parsedName := range []string{firstDocumentName, laterDocumentName} {
				compiledName := firstDocumentName
				if parsedName == firstDocumentName {
					compiledName = laterDocumentName
				}
				parsedDoc, compiledDoc := newSchemaDocument(), newSchemaDocument()
				p, okP := parsedDoc.introduce(parsed, parsedDoc.addComponents(parsed.fixture, parsed.standalone, "a"))
				_, okC := compiledDoc.introduce(compiled, compiledDoc.addComponents(compiled.fixture, compiled.standalone, "b"))
				if !okP || !okC {
					t.Fatalf("pair %q and %q cannot be rendered", parsed.fullName(), compiled.fullName())
				}
				documents := map[string]string{
					parsedName:   renderSchemaDocument(parsedDoc.root),
					compiledName: renderSchemaDocument(compiledDoc.root),
				}
				outcome := compileSchemaDocuments(documents)
				if gaps.excuses(p, outcome, parsedName) {
					continue
				}
				asserted++
				if !outcome.reports(p, parsedName) {
					t.Errorf("order across documents: %s=%q and %s=%q\n  got  %s\n  want %s\n%s:\n%s%s:\n%s",
						parsedName, parsed.fullName(), compiledName, compiled.fullName(), outcome, p.want(parsedName),
						firstDocumentName, documents[firstDocumentName], laterDocumentName, documents[laterDocumentName])
				}
			}
		}
	}
	t.Logf("across documents: %d pairs asserted", asserted)
}
