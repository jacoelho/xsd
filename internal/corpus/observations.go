package corpus

import (
	"bytes"
	"cmp"
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

// String formats r as its tab-separated line, without the line break.
func (r *Row) String() string {
	return strings.Join([]string{r.CaseID, r.Phase, string(r.Provider), string(r.Verdict), r.Message}, "\t")
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

// ParseRow parses one tab-separated observation line. A message names files
// relative to the mounted root, so a file URI or a path through tests/corpus,
// the root's parent in every machine's checkout, is rejected as a leaked
// absolute path.
func ParseRow(line string) (Row, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 5 {
		return Row{}, fmt.Errorf("row has %d fields, want 5", len(fields))
	}
	r := Row{CaseID: fields[0], Phase: fields[1], Provider: Provider(fields[2]), Verdict: Verdict(fields[3]), Message: fields[4]}
	switch {
	case r.CaseID == "" || r.Phase == "":
		return Row{}, fmt.Errorf("row %q has an empty case or phase", line)
	case !r.Provider.Known():
		return Row{}, fmt.Errorf("row %q has unknown provider %q", line, r.Provider)
	case strings.ContainsAny(r.Message, "\r\n"):
		return Row{}, fmt.Errorf("row %q has a line break in its message", line)
	case strings.Contains(r.Message, "file:/") || strings.Contains(r.Message, "tests/corpus/"):
		return Row{}, fmt.Errorf("row %q has an absolute path in its message", line)
	}
	if _, ok := r.Verdict.Outcome(); !ok {
		return Row{}, fmt.Errorf("row %q has unknown verdict %q", line, r.Verdict)
	}
	return r, nil
}

// FormatRows returns the observation file text for rows, sorted.
func FormatRows(rows []Row) []byte {
	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(a, b Row) int { return CompareRows(&a, &b) })
	var buf bytes.Buffer
	for i := range sorted {
		buf.WriteString(sorted[i].String())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// ObservationPath returns the observation file that holds the rows of the
// project case caseID, relative to the tests directory. Observation files are
// sharded like the project inventory.
func ObservationPath(caseID string) (string, error) {
	name, err := ShardName(caseID)
	if err != nil {
		return "", err
	}
	area, ok := strings.CutPrefix(name, SourceProject+"/")
	if !ok {
		return "", fmt.Errorf("case %q is not a project case", caseID)
	}
	return areaObservationPath(area), nil
}

// ObservationPaths returns the observation file of every project shard of
// inv, in shard order.
func (inv *Inventory) ObservationPaths() []string {
	var paths []string
	for _, name := range inv.Shards {
		if area, ok := strings.CutPrefix(name, SourceProject+"/"); ok {
			paths = append(paths, areaObservationPath(area))
		}
	}
	return paths
}

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

// Observations are the measured rows of every project shard and their
// profile.
type Observations struct {
	Profile Profile
	Rows    []Row
}

// LoadObservations reads the profile and the observation file of every
// project shard of inv from fsys, rooted at the tests directory. Each file must
// hold sorted, unique rows of its own shard's cases.
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
