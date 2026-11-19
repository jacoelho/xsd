package tests_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/jacoelho/xsd/internal/corpus"
)

func TestProjectPhasesHaveObservations(t *testing.T) {
	dir := testDir(t)
	inv := loadInventory(t, dir)
	obs := loadObservations(t, dir, &inv)
	if err := checkObservationCoverage(&inv, obs.Rows); err != nil {
		t.Fatal(err)
	}
}

func TestObservedDifferencesAreDiagnosed(t *testing.T) {
	dir := testDir(t)
	inv := loadInventory(t, dir)
	obs := loadObservations(t, dir, &inv)
	records := parseRecords(readTestFile(t, filepath.Dir(dir), "docs/spec/INTERPRETATIONS.md"))
	notes, err := parseNoteAllowlist(readTestFile(t, dir, "oracle-notes.txt"))
	if err != nil {
		t.Fatalf("oracle notes allowlist: %v", err)
	}
	backlog, err := parseOracleBacklog(readTestFile(t, dir, "oracle-backlog.txt"))
	if err != nil {
		t.Fatalf("oracle backlog: %v", err)
	}
	if err := checkDifferences(&inv, obs.Rows, oracleEvidence{records: records, notes: notes, backlog: backlog}); err != nil {
		t.Fatal(err)
	}
}

func TestCorpusDirectoriesHoldOnlyListedFiles(t *testing.T) {
	dir := testDir(t)
	inv := loadInventory(t, dir)
	want := append(inv.ShardPaths(), inv.ObservationPaths()...)
	want = append(want, corpus.ProfilePath)
	var got []string
	for _, root := range []string{"manifest", "oracles"} {
		err := fs.WalkDir(os.DirFS(dir), root, func(name string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				got = append(got, name)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("corpus files = %v, want the index, its shards, their observation files and the profile %v", got, want)
	}
}

func loadObservations(t *testing.T, dir string, inv *corpus.Inventory) corpus.Observations {
	t.Helper()
	obs, err := corpus.LoadObservations(os.DirFS(dir), inv)
	if err != nil {
		t.Fatalf("LoadObservations() error = %v", err)
	}
	return obs
}

func readTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))) //nolint:gosec // Test harness reads repository-owned paths.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

type phaseKey struct {
	caseID string
	phase  string
}

type rowKey struct {
	phaseKey

	provider corpus.Provider
}

func (k rowKey) String() string {
	return k.caseID + "\t" + k.phase + "\t" + string(k.provider)
}

func indexRows(rows []corpus.Row) map[rowKey]corpus.Row {
	out := make(map[rowKey]corpus.Row, len(rows))
	for _, r := range rows {
		out[rowKey{phaseKey{r.CaseID, r.Phase}, r.Provider}] = r
	}
	return out
}

// requiredRows returns the rows a measured project case has, as the
// tests/README.md Corpus Contract defines measured: every provider on the
// schema phase, and on each instance phase every provider whose schema-phase
// verdict is schema-valid.
func requiredRows(c *corpus.Case, rows map[rowKey]corpus.Row) []rowKey {
	if c.Schema == nil {
		return nil
	}
	var keys []rowKey
	for _, p := range corpus.Providers() {
		schema := rowKey{phaseKey{c.ID, corpus.SchemaPhase}, p}
		keys = append(keys, schema)
		if rows[schema].Verdict != corpus.SchemaValid {
			continue
		}
		for _, inst := range c.Instances {
			keys = append(keys, rowKey{phaseKey{c.ID, inst.TestName}, p})
		}
	}
	return keys
}

// checkObservationCoverage enforces the oracle rule of the tests/README.md
// Corpus Contract: every project case has every required row, and every row
// names a phase of a project case and a provider that phase is measured with.
func checkObservationCoverage(inv *corpus.Inventory, rows []corpus.Row) error {
	byKey := indexRows(rows)
	var errs []error
	allowed := make(map[rowKey]bool, len(rows))
	for i := range inv.Cases {
		c := &inv.Cases[i]
		if c.ExpectedSource != corpus.SourceProject {
			continue
		}
		for _, k := range requiredRows(c, byKey) {
			allowed[k] = true
			if _, ok := byKey[k]; !ok {
				errs = append(errs, fmt.Errorf("project case %q has no row for phase %q provider %s", c.ID, k.phase, k.provider))
			}
		}
	}
	for _, r := range rows {
		k := rowKey{phaseKey{r.CaseID, r.Phase}, r.Provider}
		if !allowed[k] {
			errs = append(errs, fmt.Errorf("row %s names no project phase measured with that provider", k))
		}
	}
	return errors.Join(errs...)
}

// oracleEvidence is what owns a difference: the interpretation records and
// anchored qualification rows by name, the note allowlist, and the backlog of
// cases whose differences are not yet owned.
type oracleEvidence struct {
	records map[string]string
	notes   map[rowKey]bool
	backlog []string
}

// checkDifferences enforces that every row whose validity differs from the
// project expectation has a differences entry, and that every entry explains
// such a row: a record or anchored row that exists, lists the case and names
// the provider, or a note the allowlist names. Instance phases of a case whose
// schema the project expects invalid have no expectation to differ from. A
// backlog case is exempt from the ownership requirement, and must still have a
// row it exempts; the backlog lists only project cases.
func checkDifferences(inv *corpus.Inventory, rows []corpus.Row, ev oracleEvidence) error {
	byKey := indexRows(rows)
	listed := make(map[string]bool, len(ev.backlog))
	for _, id := range ev.backlog {
		listed[id] = true
	}
	var errs, undiagnosed []error
	usedNotes := make(map[rowKey]bool, len(ev.notes))
	for i := range inv.Cases {
		c := &inv.Cases[i]
		if c.ExpectedSource != corpus.SourceProject {
			continue
		}
		unowned := 0
		for _, phase := range comparablePhases(c) {
			for _, p := range corpus.Providers() {
				k := rowKey{phaseKey{c.ID, phase.Name}, p}
				differs := rowDiffers(byKey, k, phase.Expected)
				d, ok := phase.Differences[p]
				switch {
				case differs && !ok:
					unowned++
					if !listed[c.ID] {
						undiagnosed = append(undiagnosed, fmt.Errorf("%s\t%s (expected %s)", k, byKey[k].Verdict, phase.Expected))
					}
				case ok && !differs:
					errs = append(errs, fmt.Errorf("%s has a difference but no row whose validity differs", k))
				case ok:
					if err := checkDifference(k, d, ev); err != nil {
						errs = append(errs, err)
					}
					usedNotes[k] = d.Note != ""
				}
			}
		}
		if listed[c.ID] && unowned == 0 {
			errs = append(errs, fmt.Errorf("oracle backlog lists %q, whose differences are all owned; remove it", c.ID))
		}
		delete(listed, c.ID)
		errs = append(errs, uncomparableDifferences(c)...)
	}
	for _, id := range slices.Sorted(maps.Keys(listed)) {
		errs = append(errs, fmt.Errorf("oracle backlog lists %q, which is not a project case", id))
	}
	for _, k := range slices.SortedFunc(maps.Keys(ev.notes), func(a, b rowKey) int { return strings.Compare(a.String(), b.String()) }) {
		if !usedNotes[k] {
			errs = append(errs, fmt.Errorf("oracle notes allowlist lists %s, which has no note", k))
		}
	}
	if len(undiagnosed) > 0 {
		errs = append(errs, fmt.Errorf("%d rows differ from the project expectation without a differences entry:\n%w", len(undiagnosed), errors.Join(undiagnosed...)))
	}
	return errors.Join(errs...)
}

// comparablePhases returns the phases with an expectation a provider can
// differ from: the schema phase, and the instance phases when the project
// expects the schema valid.
func comparablePhases(c *corpus.Case) []corpus.Phase {
	phases := c.Phases()
	if c.Schema == nil || c.Schema.Expected == "valid" {
		return phases
	}
	return phases[:1]
}

func uncomparableDifferences(c *corpus.Case) []error {
	if c.Schema == nil || c.Schema.Expected == "valid" {
		return nil
	}
	var errs []error
	for _, inst := range c.Instances {
		if len(inst.Differences) > 0 {
			errs = append(errs, fmt.Errorf("case %q instance %q has differences, but its schema is expected invalid", c.ID, inst.TestName))
		}
	}
	return errs
}

func rowDiffers(rows map[rowKey]corpus.Row, k rowKey, expected string) bool {
	r, ok := rows[k]
	if !ok {
		return false
	}
	validity, ok := r.Verdict.Validity()
	return ok && validity != expected
}

// providerName returns the name an interpretation record uses for p.
func providerName(p corpus.Provider) string {
	switch p {
	case corpus.Xerces:
		return "Xerces"
	case corpus.DotNet:
		return ".NET"
	case corpus.XMLSchema:
		return "xmlschema"
	case corpus.LibXML2:
		return "libxml2"
	}
	return string(p)
}

func checkDifference(k rowKey, d corpus.Difference, ev oracleEvidence) error {
	if d.Note != "" {
		if !ev.notes[k] {
			return fmt.Errorf("%s has a note, but the oracle notes allowlist does not list it; cite a record", k)
		}
		return nil
	}
	text, ok := ev.records[d.Record]
	switch {
	case !ok:
		return fmt.Errorf("%s cites %s, which docs/spec/INTERPRETATIONS.md does not define", k, d.Record)
	case !recordListsCase(text, k.caseID):
		return fmt.Errorf("%s cites %s, which does not list the case", k, d.Record)
	case !strings.Contains(linkTargets.ReplaceAllString(text, ""), providerName(k.provider)):
		return fmt.Errorf("%s cites %s, which does not name %s", k, d.Record, providerName(k.provider))
	}
	return nil
}

// linkTargets matches Markdown link targets and bare URLs, whose text names no
// provider: specification URLs contain "xmlschema".
var linkTargets = regexp.MustCompile(`\]\([^)]*\)|<?https?://[^\s)>]*>?`)

// parseRecords returns, keyed by name, the text of every "## I-NNN" section
// of docs/spec/INTERPRETATIONS.md and the line of every row carrying an
// <a id="..."> anchor.
func parseRecords(doc string) map[string]string {
	records := make(map[string]string)
	var current string
	for line := range strings.Lines(doc) {
		if _, rest, ok := strings.Cut(line, `<a id="`); ok {
			if anchor, _, ok := strings.Cut(rest, `"`); ok {
				records[anchor] = line
			}
		}
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			current = ""
			if id, _, _ := strings.Cut(heading, " "); strings.HasPrefix(id, "I-") {
				current = id
			}
			continue
		}
		if current != "" {
			records[current] += line
		}
	}
	return records
}

// recordListsCase reports whether text lists caseID as a backticked ID or a
// backticked pattern whose "*" matches any run of characters.
func recordListsCase(text, caseID string) bool {
	parts := strings.Split(text, "`")
	for i := 1; i < len(parts); i += 2 {
		if globMatch(parts[i], caseID) {
			return true
		}
	}
	return false
}

func globMatch(pattern, s string) bool {
	pieces := strings.Split(pattern, "*")
	if len(pieces) == 1 {
		return pattern == s
	}
	rest, ok := strings.CutPrefix(s, pieces[0])
	if !ok {
		return false
	}
	for _, piece := range pieces[1 : len(pieces)-1] {
		i := strings.Index(rest, piece)
		if i < 0 {
			return false
		}
		rest = rest[i+len(piece):]
	}
	return strings.HasSuffix(rest, pieces[len(pieces)-1])
}

// parseNoteAllowlist reads tests/oracle-notes.txt: one sorted, unique
// "case<TAB>phase<TAB>provider" line per difference whose diagnosis may be a
// note instead of a record. A note is accepted only for a listed key, and the
// list only shrinks.
func parseNoteAllowlist(data string) (map[rowKey]bool, error) {
	notes := make(map[rowKey]bool)
	if data == "" {
		return notes, nil
	}
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		switch {
		case len(fields) != 3 || slices.Contains(fields, ""):
			return nil, fmt.Errorf("line %d is not case, phase and provider", i+1)
		case !corpus.Provider(fields[2]).Known():
			return nil, fmt.Errorf("line %d has unknown provider %q", i+1, fields[2])
		case i > 0 && line <= lines[i-1]:
			return nil, fmt.Errorf("line %d is not sorted after %q", i+1, lines[i-1])
		}
		notes[rowKey{phaseKey{fields[0], fields[1]}, corpus.Provider(fields[2])}] = true
	}
	return notes, nil
}

// parseOracleBacklog reads tests/oracle-backlog.txt: one project case ID per
// line, sorted and unique, each a measured case with a provider difference
// that no record owns yet.
func parseOracleBacklog(data string) ([]string, error) {
	if data == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	for i, line := range lines {
		switch {
		case line == "":
			return nil, fmt.Errorf("line %d is empty", i+1)
		case strings.TrimFunc(line, unicode.IsSpace) != line:
			return nil, fmt.Errorf("line %d has surrounding space", i+1)
		case i > 0 && line <= lines[i-1]:
			return nil, fmt.Errorf("line %d is not sorted after %q", i+1, lines[i-1])
		}
	}
	return lines, nil
}
