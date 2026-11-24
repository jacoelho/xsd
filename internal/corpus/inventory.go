// Package corpus reads the conformance corpus under tests: the hand-written
// inventory of cases and expectations in tests/manifest, and the measured
// cross-validator observations in tests/oracles. tests/README.md owns the
// meaning of both; this package owns their file layout and grammar.
//
// Every loader reads from an fs.FS rooted at the tests directory, and every
// formatter returns the bytes of a file, so callers choose the filesystem and
// the package performs no file I/O of its own.
package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
)

// IndexPath is the inventory index: the header and the ordered shard list.
const IndexPath = "manifest/index.json"

// The expected sources of a case, each the first segment of its ID.
const (
	SourceProject = "project"
	SourceW3C     = "w3c"
	SourceXercesJ = "xerces-j"
)

// SchemaPhase names the schema phase of a case; an instance phase is named by
// its instance's test name.
const SchemaPhase = "schema"

// Inventory is the hand-written corpus inventory: every case, in shard order.
type Inventory struct {
	Version       string
	Specification string
	Shards        []string
	Cases         []Case
}

// Case is one corpus case: an optional schema phase and its instance phases.
// Title, Source, DocRefs and Internal are provenance metadata that no
// consumer interprets; Source and DocRefs keep their upstream shape.
type Case struct {
	Schema         *Schema         `json:"schema"`
	ID             string          `json:"id"`
	ExpectedSource string          `json:"expectedSource"`
	Title          string          `json:"title"`
	Source         json.RawMessage `json:"source"`
	DocRefs        json.RawMessage `json:"docRefs"`
	Internal       json.RawMessage `json:"internal"`
	Instances      []Instance      `json:"instances"`
	Files          []File          `json:"files"`
}

// Schema is the schema phase of a case. TestName, Status and Version are
// upstream provenance metadata that no consumer interprets.
type Schema struct {
	Rule        *Rule                   `json:"rule"`
	Oracle      map[string]Provenance   `json:"oracle"`
	Differences map[Provider]Difference `json:"differences"`
	Expected    Expectation             `json:"expected"`
	ErrorCode   string                  `json:"errorCode"`
	TestName    string                  `json:"testName"`
	Status      string                  `json:"status"`
	Version     string                  `json:"version"`
	Documents   []Document              `json:"documents"`
}

// Instance is one instance phase of a case. Status, Version and
// InternalExpected are upstream or extraction provenance metadata that no
// consumer interprets.
type Instance struct {
	Rule             *Rule                   `json:"rule"`
	Oracle           map[string]Provenance   `json:"oracle"`
	Differences      map[Provider]Difference `json:"differences"`
	TestName         string                  `json:"testName"`
	File             string                  `json:"file"`
	Expected         Expectation             `json:"expected"`
	ErrorCode        string                  `json:"errorCode"`
	Status           string                  `json:"status"`
	Version          string                  `json:"version"`
	InternalExpected string                  `json:"internalExpected"`
}

// Phase is what every schema or instance phase has: its name within its case,
// the project expectation, the rule that decides it, the hand-written
// provenance and the diagnosed differences of cross-validator providers.
type Phase struct {
	Rule        *Rule
	Oracle      map[string]Provenance
	Differences map[Provider]Difference
	Name        string
	Expected    Expectation
}

// Document is a schema compilation root. Role is upstream metadata that no
// consumer interprets.
type Document struct {
	File string `json:"file"`
	Role string `json:"role"`
}

// File is a corpus file a case references.
type File struct {
	File string `json:"file"`
	Role string `json:"role"`
}

// Expectation is the project's expected result of a schema or instance phase.
type Expectation string

// The project expectations.
const (
	ExpectedValid   Expectation = "valid"
	ExpectedInvalid Expectation = "invalid"
)

// Known reports whether e is a project expectation.
func (e Expectation) Known() bool {
	return e == ExpectedValid || e == ExpectedInvalid
}

// Provenance is a hand-written original expectation under oracle.w3c or
// oracle.project: the upstream or earlier project result that the effective
// expectation corrects.
type Provenance struct {
	Expected Expectation `json:"expected"`
	Reason   string      `json:"reason"`
	SpecRef  string      `json:"specRef"`
}

// Rule names the specification clause or interpretation record that decides
// a phase's expectation. Clause is empty where no label was recorded.
type Rule struct {
	SpecRef string `json:"specRef"`
	Clause  string `json:"clause"`
}

// Difference explains why a provider's measured validity differs from the
// project expectation: Record names an interpretation record (I-NNN) or the
// anchor of a qualification-table row in docs/spec/INTERPRETATIONS.md.
type Difference struct {
	Record string `json:"record"`
}

// Phases returns the case's phases in order: the schema phase, when present,
// then every instance phase.
func (c *Case) Phases() []Phase {
	phases := make([]Phase, 0, 1+len(c.Instances))
	if s := c.Schema; s != nil {
		phases = append(phases, Phase{Name: SchemaPhase, Expected: s.Expected, Rule: s.Rule, Oracle: s.Oracle, Differences: s.Differences})
	}
	for _, inst := range c.Instances {
		phases = append(phases, Phase{Name: inst.TestName, Expected: inst.Expected, Rule: inst.Rule, Oracle: inst.Oracle, Differences: inst.Differences})
	}
	return phases
}

// Measured reports whether the oracle observation contract of the
// tests/README.md Corpus Contract covers c: c is a project case, or a W3C case
// with a phase that carries an oracle.w3c expectation. LoadInventory rejects
// an oracle expectation equal to the effective one, so such a phase overrides
// the W3C expectation.
func (c *Case) Measured() bool {
	switch c.ExpectedSource {
	case SourceProject:
		return true
	case SourceW3C:
		return slices.ContainsFunc(c.Phases(), func(p Phase) bool {
			_, ok := p.Oracle[SourceW3C]
			return ok
		})
	}
	return false
}

// ShardName returns the inventory shard that owns caseID: W3C cases by upstream
// test set, with the NIST datatype set split by its atomic, list and union
// groups to bound file size; Xerces-J cases together; project cases by their
// second ID segment, and single-segment project IDs in project/top-level.
func ShardName(caseID string) (string, error) {
	segments := strings.Split(caseID, "/")
	switch {
	case segments[0] == SourceXercesJ && len(segments) > 1:
		return SourceXercesJ, nil
	case segments[0] == SourceProject && len(segments) == 2:
		return SourceProject + "/top-level", nil
	case segments[0] == SourceProject && len(segments) > 2:
		return SourceProject + "/" + segments[1], nil
	case segments[0] == SourceW3C && len(segments) > 3:
		set := strings.TrimSuffix(strings.TrimSuffix(segments[2], ".testSet"), ".xml")
		name := SourceW3C + "/" + segments[1] + "/" + set
		if segments[2] == "NISTXMLSchemaDatatypes.testSet" {
			group, _, _ := strings.Cut(segments[3], "-")
			name += "-" + group
		}
		return name, nil
	}
	return "", fmt.Errorf("case ID %q has no shard", caseID)
}

func shardPath(name string) string {
	return "manifest/" + name + ".json"
}

// index is the IndexPath file; source records the upstream suites the
// inventory was extracted from, which no consumer interprets.
type index struct {
	Version       string          `json:"version"`
	Specification string          `json:"specification"`
	Source        json.RawMessage `json:"source"`
	Shards        []string        `json:"shards"`
}

type shard struct {
	Cases []Case `json:"cases"`
}

// LoadInventory reads the index and every shard it lists from fsys, rooted at
// the tests directory, and validates the result. A key the grammar does not
// declare is an error, so a misspelt optional key cannot load as an absent one.
func LoadInventory(fsys fs.FS) (Inventory, error) {
	var idx index
	if err := readJSON(fsys, IndexPath, &idx); err != nil {
		return Inventory{}, err
	}
	if idx.Version == "" || idx.Specification == "" || len(idx.Shards) == 0 {
		return Inventory{}, fmt.Errorf("%s needs a version, a specification and shards", IndexPath)
	}
	inv := Inventory{Version: idx.Version, Specification: idx.Specification, Shards: idx.Shards}
	seen := make(map[string]bool, len(idx.Shards))
	for _, name := range idx.Shards {
		if seen[name] {
			return Inventory{}, fmt.Errorf("%s lists shard %q twice", IndexPath, name)
		}
		seen[name] = true
		cases, err := loadShard(fsys, name)
		if err != nil {
			return Inventory{}, err
		}
		inv.Cases = append(inv.Cases, cases...)
	}
	if err := ValidateCases(inv.Cases); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

// readJSON decodes the one JSON value of file name into v, rejecting keys v
// does not declare and data after the value.
func readJSON(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: data after the JSON value", name)
	}
	return nil
}

// loadShard reads one shard; every case in it must belong to it.
func loadShard(fsys fs.FS, name string) ([]Case, error) {
	var s shard
	if err := readJSON(fsys, shardPath(name), &s); err != nil {
		return nil, err
	}
	for i := range s.Cases {
		want, err := ShardName(s.Cases[i].ID)
		if err != nil {
			return nil, err
		}
		if want != name {
			return nil, fmt.Errorf("case %q is in shard %q, want %q", s.Cases[i].ID, name, want)
		}
	}
	return s.Cases, nil
}

// ValidateCases checks the case invariants the tests/README.md Corpus Contract
// states: unique case IDs prefixed by their expected source, known expected
// values, a schema phase with at least one root, unique schema roots and
// instance names, an instance file per instance, well-formed provenance whose
// original expectation differs from the effective one, and well-formed rules
// and differences.
func ValidateCases(cases []Case) error {
	ids := make(map[string]bool, len(cases))
	for i := range cases {
		c := &cases[i]
		if ids[c.ID] {
			return fmt.Errorf("case %q is duplicated", c.ID)
		}
		ids[c.ID] = true
		if err := validateCase(c); err != nil {
			return err
		}
	}
	return nil
}

func validateCase(c *Case) error {
	switch c.ExpectedSource {
	case SourceProject, SourceW3C, SourceXercesJ:
	default:
		return fmt.Errorf("case %q has unknown expectedSource %q", c.ID, c.ExpectedSource)
	}
	if !strings.HasPrefix(c.ID, c.ExpectedSource+"/") {
		return fmt.Errorf("case %q is not prefixed by its expectedSource %q", c.ID, c.ExpectedSource)
	}
	if c.Schema != nil {
		if err := validateDocuments(c.ID, c.Schema.Documents); err != nil {
			return err
		}
	}
	if err := validateInstances(c); err != nil {
		return err
	}
	for _, p := range c.Phases() {
		if err := validatePhase(&p); err != nil {
			return fmt.Errorf("case %q phase %q: %w", c.ID, p.Name, err)
		}
	}
	return nil
}

// validateInstances requires every instance a file and a unique test name
// other than the schema phase's, since the name keys the instance phase.
func validateInstances(c *Case) error {
	names := make(map[string]bool, len(c.Instances))
	for _, inst := range c.Instances {
		switch {
		case inst.TestName == "" || inst.TestName == SchemaPhase:
			return fmt.Errorf("case %q has instance test name %q", c.ID, inst.TestName)
		case names[inst.TestName]:
			return fmt.Errorf("case %q duplicates instance %q", c.ID, inst.TestName)
		case inst.File == "":
			return fmt.Errorf("case %q instance %q has no file", c.ID, inst.TestName)
		}
		names[inst.TestName] = true
	}
	return nil
}

func validateDocuments(id string, docs []Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("case %q has a schema phase without documents", id)
	}
	files := make(map[string]bool, len(docs))
	for _, doc := range docs {
		switch {
		case doc.File == "":
			return fmt.Errorf("case %q has a schema document without a file", id)
		case files[doc.File]:
			return fmt.Errorf("case %q duplicates schema document %q", id, doc.File)
		}
		files[doc.File] = true
	}
	return nil
}

func validatePhase(p *Phase) error {
	if !p.Expected.Known() {
		return fmt.Errorf("unknown expected value %q", p.Expected)
	}
	for _, key := range slices.Sorted(maps.Keys(p.Oracle)) {
		entry := p.Oracle[key]
		switch {
		case key != SourceW3C && key != SourceProject:
			return fmt.Errorf("unknown oracle key %q", key)
		case !entry.Expected.Known():
			return fmt.Errorf("oracle %q has unknown expected value %q", key, entry.Expected)
		case entry.Expected == p.Expected:
			return fmt.Errorf("oracle %q expectation %q equals the effective expectation, so it corrects nothing", key, entry.Expected)
		case blank(entry.Reason):
			return fmt.Errorf("oracle %q has no reason", key)
		}
	}
	if p.Rule != nil && blank(p.Rule.SpecRef) {
		return errors.New("rule has no specRef")
	}
	for _, provider := range slices.Sorted(maps.Keys(p.Differences)) {
		if err := validateDifference(provider, p.Differences[provider]); err != nil {
			return err
		}
	}
	return nil
}

func validateDifference(provider Provider, d Difference) error {
	if !provider.Known() {
		return fmt.Errorf("difference for unknown provider %q", provider)
	}
	switch {
	case d.Record == "":
		return fmt.Errorf("difference for %s has no record", provider)
	case !isRecordName(d.Record):
		return fmt.Errorf("difference for %s names record %q, want I-NNN or a row anchor", provider, d.Record)
	}
	return nil
}

// isRecordName reports whether s names an interpretation record, I-NNN, or a
// qualification-table row anchor of lowercase letters, digits and hyphens.
func isRecordName(s string) bool {
	if digits, ok := strings.CutPrefix(s, "I-"); ok {
		return len(digits) == 3 && strings.Trim(digits, "0123456789") == ""
	}
	return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

func blank(s string) bool {
	return strings.Trim(s, " \t\r\n") == ""
}

// ShardPaths returns the slash-separated paths, relative to the tests
// directory, of every inventory file: the index, then the shards in order.
func (inv *Inventory) ShardPaths() []string {
	paths := []string{IndexPath}
	for _, name := range inv.Shards {
		paths = append(paths, path.Clean(shardPath(name)))
	}
	return paths
}
