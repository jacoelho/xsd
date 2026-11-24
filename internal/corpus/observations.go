package corpus

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// ProfilePath holds the measurement profile shared by every observation row.
const ProfilePath = "oracles/profile.json"

// Provider is a cross-validator oracle key, as the tests/README.md
// Cross-validator Diagnosis table defines.
type Provider string

// The cross-validator providers.
const (
	Xerces    Provider = "xerces"
	DotNet    Provider = "dotnet"
	XMLSchema Provider = "xmlschema"
	LibXML2   Provider = "libxml2"
)

// Providers returns every provider in tie-break order.
func Providers() []Provider {
	return []Provider{Xerces, DotNet, LibXML2, XMLSchema}
}

// Known reports whether p is a cross-validator provider.
func (p Provider) Known() bool {
	return slices.Contains(Providers(), p)
}

// Verdict is the raw verdict tools/oracles/run.sh prints for one provider.
type Verdict string

// The verdicts tools/oracles/run.sh documents.
const (
	SchemaValid   Verdict = "schema-valid"
	SchemaInvalid Verdict = "schema-invalid"
	Valid         Verdict = "valid"   //nolint:goconst // A verdict, not an Expectation; the two vocabularies share this spelling.
	Invalid       Verdict = "invalid" //nolint:goconst // A verdict, not an Expectation; the two vocabularies share this spelling.
	Unassessed    Verdict = "unassessed"
	Blocked       Verdict = "blocked"
	Error         Verdict = "error"
)

// Outcome is what a verdict establishes about a phase: a validity, or the
// reason it establishes none.
type Outcome int

// The outcomes of a verdict. An error verdict reads as blocked: the provider
// produced no validity result on the frozen inputs.
const (
	OutcomeValid Outcome = iota + 1
	OutcomeInvalid
	OutcomeUnassessed
	OutcomeBlocked
)

// Outcome maps the raw verdict to what it establishes. It reports false for a
// verdict tools/oracles/run.sh does not print.
func (v Verdict) Outcome() (Outcome, bool) {
	switch v {
	case SchemaValid, Valid:
		return OutcomeValid, true
	case SchemaInvalid, Invalid:
		return OutcomeInvalid, true
	case Unassessed:
		return OutcomeUnassessed, true
	case Blocked, Error:
		return OutcomeBlocked, true
	}
	return 0, false
}

// Expectation returns the project expectation that the verdict's validity
// matches, and false when the verdict measured no validity.
func (v Verdict) Expectation() (Expectation, bool) {
	switch o, _ := v.Outcome(); o {
	case OutcomeValid:
		return ExpectedValid, true
	case OutcomeInvalid:
		return ExpectedInvalid, true
	case OutcomeUnassessed, OutcomeBlocked:
	}
	return "", false
}

// Row is one observation: the verdict and first diagnostic one provider
// reported for one phase of one case.
type Row struct {
	CaseID   string
	Phase    string
	Provider Provider
	Verdict  Verdict
	Message  string
}

// CompareRows orders rows by case ID, phase and provider: the order of an
// observation file.
func CompareRows(a, b *Row) int {
	return cmp.Or(
		strings.Compare(a.CaseID, b.CaseID),
		strings.Compare(a.Phase, b.Phase),
		strings.Compare(string(a.Provider), string(b.Provider)),
	)
}

// ParseRow parses one tab-separated observation line and validates the row
// as Row.Validate does.
func ParseRow(line string) (Row, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 5 {
		return Row{}, fmt.Errorf("row has %d fields, want 5", len(fields))
	}
	r := Row{CaseID: fields[0], Phase: fields[1], Provider: Provider(fields[2]), Verdict: Verdict(fields[3]), Message: fields[4]}
	if err := r.Validate(); err != nil {
		return Row{}, fmt.Errorf("row %q: %w", line, err)
	}
	return r, nil
}

// Validate reports whether r has an observation line: a case and a phase, a
// known provider and verdict, and no field holding a tab or a line break. A
// message names files relative to the mounted root, so a file URI or a path
// through tests/corpus, the root's parent in every machine's checkout, is
// rejected as a leaked absolute path.
func (r *Row) Validate() error {
	switch {
	case r.CaseID == "" || r.Phase == "":
		return errors.New("empty case or phase")
	case strings.ContainsAny(r.CaseID, "\t\r\n") || strings.ContainsAny(r.Phase, "\t\r\n"):
		return fmt.Errorf("case %q or phase %q holds a tab or a line break", r.CaseID, r.Phase)
	case !r.Provider.Known():
		return fmt.Errorf("unknown provider %q", r.Provider)
	case strings.ContainsAny(r.Message, "\t\r\n"):
		return errors.New("a tab or a line break in its message")
	case strings.Contains(r.Message, "file:/") || strings.Contains(r.Message, "tests/corpus/"):
		return errors.New("an absolute path in its message")
	}
	if _, ok := r.Verdict.Outcome(); !ok {
		return fmt.Errorf("unknown verdict %q", r.Verdict)
	}
	return nil
}

// FormatRows returns the observation file text for rows, sorted. It fails if
// a row does not validate or repeats the case, phase and provider of another.
func FormatRows(rows []Row) ([]byte, error) {
	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(a, b Row) int { return CompareRows(&a, &b) })
	var buf bytes.Buffer
	for i := range sorted {
		r := &sorted[i]
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("row of case %q phase %q provider %q: %w", r.CaseID, r.Phase, r.Provider, err)
		}
		if i > 0 && CompareRows(&sorted[i-1], r) == 0 {
			return nil, fmt.Errorf("case %q phase %q has two rows for %s", r.CaseID, r.Phase, r.Provider)
		}
		buf.WriteString(strings.Join([]string{r.CaseID, r.Phase, string(r.Provider), string(r.Verdict), r.Message}, "\t"))
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// ObservationPath returns the observation file that holds the rows of the
// project or W3C case caseID, relative to the tests directory. Project files
// are sharded like the project inventory; every W3C case shares one file.
func ObservationPath(caseID string) (string, error) {
	name, err := ShardName(caseID)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(name, SourceW3C+"/") {
		return w3cObservationPath, nil
	}
	area, ok := strings.CutPrefix(name, SourceProject+"/")
	if !ok {
		return "", fmt.Errorf("case %q is neither a project nor a W3C case", caseID)
	}
	return areaObservationPath(area), nil
}

// ObservationPaths returns the observation file of every project shard of
// inv, in shard order, then the W3C file when inv has a W3C shard.
func (inv *Inventory) ObservationPaths() []string {
	var paths []string
	w3c := false
	for _, name := range inv.Shards {
		if area, ok := strings.CutPrefix(name, SourceProject+"/"); ok {
			paths = append(paths, areaObservationPath(area))
		}
		w3c = w3c || strings.HasPrefix(name, SourceW3C+"/")
	}
	if w3c {
		paths = append(paths, w3cObservationPath)
	}
	return paths
}

const w3cObservationPath = "oracles/" + SourceW3C + ".tsv"

func areaObservationPath(area string) string {
	return "oracles/" + area + ".tsv"
}

// Profile identifies the measurement every observation row comes from. Image
// is the digest of the oracle image's inputs under tools/oracles, since a
// rebuilt image's ID differs even from unchanged inputs.
type Profile struct {
	Providers map[Provider]string `json:"providers"`
	Date      string              `json:"date"`
	Image     string              `json:"image"`
}

// Observations are the rows of every observation file and their profile.
type Observations struct {
	Profile Profile
	Rows    []Row
}

// LoadObservations reads the profile and every observation file of inv from
// fsys, rooted at the tests directory. Each file must hold sorted, unique rows
// of the cases ObservationPath assigns to it.
func LoadObservations(fsys fs.FS, inv *Inventory) (Observations, error) {
	var obs Observations
	if err := readJSON(fsys, ProfilePath, &obs.Profile); err != nil {
		return Observations{}, err
	}
	if err := validateProfile(obs.Profile); err != nil {
		return Observations{}, err
	}
	for _, name := range inv.ObservationPaths() {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return Observations{}, err
		}
		rows, err := parseRows(name, string(data))
		if err != nil {
			return Observations{}, err
		}
		obs.Rows = append(obs.Rows, rows...)
	}
	return obs, nil
}

// FormatProfile returns the ProfilePath file text for p. It fails if p lacks a
// date, an image or the version of a provider, or names an unknown provider,
// so its text always loads.
func FormatProfile(p Profile) ([]byte, error) {
	if err := validateProfile(p); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateProfile(p Profile) error {
	if p.Date == "" || p.Image == "" {
		return fmt.Errorf("%s needs a date and an image", ProfilePath)
	}
	for _, provider := range Providers() {
		if p.Providers[provider] == "" {
			return fmt.Errorf("%s has no version for %s", ProfilePath, provider)
		}
	}
	if len(p.Providers) != len(Providers()) {
		return fmt.Errorf("%s lists an unknown provider", ProfilePath)
	}
	return nil
}

func parseRows(name, data string) ([]Row, error) {
	if data == "" {
		return nil, nil
	}
	if !strings.HasSuffix(data, "\n") {
		return nil, fmt.Errorf("%s does not end with a line break", name)
	}
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	rows := make([]Row, 0, len(lines))
	for i, line := range lines {
		r, err := parseShardRow(name, line)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", name, i+1, err)
		}
		if i > 0 && CompareRows(&rows[i-1], &r) >= 0 {
			return nil, fmt.Errorf("%s line %d is not sorted after line %d, or repeats its case, phase and provider", name, i+1, i)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func parseShardRow(name, line string) (Row, error) {
	r, err := ParseRow(line)
	if err != nil {
		return Row{}, err
	}
	owner, err := ObservationPath(r.CaseID)
	if err != nil {
		return Row{}, fmt.Errorf("case %q has no observation file: %w", r.CaseID, err)
	}
	if owner != name {
		return Row{}, fmt.Errorf("case %q belongs in %q", r.CaseID, owner)
	}
	return r, nil
}
