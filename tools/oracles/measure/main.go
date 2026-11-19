// Command measure re-measures project corpus cases with tools/oracles/run.sh
// and rewrites their rows in tests/oracles.
//
// Usage, from the repository root:
//
//	go run ./tools/oracles/measure [-j N] [-cache DIR] [case-id...]
//
// Without case IDs it measures every project case and rewrites every
// observation file and the profile. With case IDs it replaces only those
// cases' rows and keeps the profile, whose image digest and provider versions
// the run must match.
//
// It builds the oracle image once and runs every phase against that image ID,
// so a concurrent rebuild of the tag cannot change providers mid-run. A
// rebuild yields a new image ID even from unchanged inputs, so the image is
// identified by the digest of its inputs, every file under tools/oracles
// outside this command. Each run's output is cached under a key covering that
// digest, the run arguments and the bytes of every case file, so an
// interrupted measurement resumes where it stopped. Runs that report an error
// verdict are not cached and are retried once, serially, after the parallel
// pass.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacoelho/xsd/internal/corpus"
)

// runTimeout bounds one run.sh invocation: four providers, each bounded by
// run.sh's own per-validator limit, plus container start.
const runTimeout = 15 * time.Minute

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "measure:", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	flags := flag.NewFlagSet("measure", flag.ContinueOnError)
	jobs := flags.Int("j", 6, "concurrent run.sh invocations")
	cacheDir := flags.String("cache", "", "run output cache (default: the user cache directory)")
	if err := flags.Parse(argv); err != nil {
		return err
	}
	if *jobs < 1 {
		return errors.New("-j must be at least 1")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		return err
	}
	cases, all, err := selectCases(&inv, flags.Args())
	if err != nil {
		return err
	}
	cache, err := cachePath(*cacheDir)
	if err != nil {
		return err
	}
	digest, err := imageDigest("tools/oracles")
	if err != nil {
		return err
	}
	imageID, err := buildImage(ctx)
	if err != nil {
		return err
	}
	m := &measurer{imageID: imageID, digest: digest, cache: cache}
	results, err := m.measureAll(ctx, cases, *jobs)
	if err != nil {
		return err
	}
	return record(&inv, results, all, digest)
}

// record writes results as observation rows. Unless all is set it keeps the
// rows of unmeasured cases and the profile they share.
func record(inv *corpus.Inventory, results map[string]caseResult, all bool, digest string) error { //nolint:revive // all selects between a full and a partial rewrite, a single decision.
	versions, err := providerVersions(results)
	if err != nil {
		return err
	}
	profile := corpus.Profile{Providers: versions, Date: time.Now().UTC().Format(time.DateOnly), Image: digest}
	if all {
		return write(inv, results, nil, &profile)
	}
	kept, err := unmeasuredRows(inv, results, profile)
	if err != nil {
		return err
	}
	return write(inv, results, kept, nil)
}

// imageDigest returns the SHA-256 digest of the image's inputs: the path and
// bytes of every file under dir except this command's own directory.
func imageDigest(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(name string, entry os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && name == filepath.Join(dir, "measure"):
			return filepath.SkipDir
		case entry.IsDir():
			return nil
		}
		data, err := os.ReadFile(name) //nolint:gosec // The walk stays under tools/oracles.
		if err != nil {
			return err
		}
		writeField(h, filepath.ToSlash(name))
		writeField(h, string(data))
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func cachePath(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "xsd-oracles-measure"), nil
}

// selectCases returns the project cases named by ids, or every project case
// when ids is empty, and whether the selection is every project case.
func selectCases(inv *corpus.Inventory, ids []string) ([]*corpus.Case, bool, error) {
	byID := make(map[string]*corpus.Case)
	var project []*corpus.Case
	for i := range inv.Cases {
		c := &inv.Cases[i]
		if c.ExpectedSource == corpus.SourceProject {
			byID[c.ID] = c
			project = append(project, c)
		}
	}
	if len(ids) == 0 {
		return project, true, nil
	}
	selected := make([]*corpus.Case, 0, len(ids))
	for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
		c, ok := byID[id]
		if !ok {
			return nil, false, fmt.Errorf("%q is not a project case", id)
		}
		selected = append(selected, c)
	}
	return selected, len(selected) == len(project), nil
}

func buildImage(ctx context.Context) (string, error) {
	tag := os.Getenv("XSD_ORACLES_IMAGE")
	if tag == "" {
		tag = "xsd-oracles:local"
	}
	//nolint:gosec // The tag is the caller's own image name, passed as one argument without a shell.
	out, err := exec.CommandContext(ctx, "docker", "build", "--quiet", "--tag", tag, "tools/oracles").Output()
	if err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

type measurer struct {
	imageID string
	digest  string
	cache   string
}

// caseResult is the rows of one case and the provider versions its runs
// reported.
type caseResult struct {
	versions map[corpus.Provider]string
	rows     []corpus.Row
	errored  bool
}

// measureAll measures cases with at most jobs concurrent runs, stopping
// scheduling on interrupt, and fails if any case failed.
func (m *measurer) measureAll(ctx context.Context, cases []*corpus.Case, jobs int) (map[string]caseResult, error) {
	col := &collector{results: make(map[string]caseResult, len(cases)), total: len(cases)}
	queue := make(chan *corpus.Case)
	var wg sync.WaitGroup
	for range jobs {
		wg.Go(func() {
			for c := range queue {
				res, err := m.measureCase(ctx, c)
				col.add(c, res, err)
			}
		})
	}
	for _, c := range cases {
		if ctx.Err() != nil {
			break
		}
		queue <- c
	}
	close(queue)
	wg.Wait()
	if err := errors.Join(append(col.failures, ctx.Err())...); err != nil {
		return nil, err
	}
	return m.retryErrors(ctx, cases, col.results)
}

// collector gathers the workers' case results and reports progress.
type collector struct {
	results  map[string]caseResult
	failures []error
	total    int
	mu       sync.Mutex
}

func (col *collector) add(c *corpus.Case, res caseResult, err error) {
	col.mu.Lock()
	defer col.mu.Unlock()
	if err != nil {
		col.failures = append(col.failures, fmt.Errorf("%s: %w", c.ID, err))
	} else {
		col.results[c.ID] = res
	}
	if done := len(col.results) + len(col.failures); done%100 == 0 || done == col.total {
		fmt.Fprintf(os.Stderr, "measured %d/%d cases\n", done, col.total)
	}
}

// retryErrors measures again, one at a time, every case with an error
// verdict, since concurrent load can make a provider time out.
func (m *measurer) retryErrors(ctx context.Context, cases []*corpus.Case, results map[string]caseResult) (map[string]caseResult, error) {
	for _, c := range cases {
		if !results[c.ID].errored {
			continue
		}
		res, err := m.measureCase(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.ID, err)
		}
		results[c.ID] = res
	}
	return results, nil
}

// measureCase runs the schema phase and then, if any provider accepted the
// schema, every instance phase. Instance rows are kept only for providers
// that accepted the schema.
func (m *measurer) measureCase(ctx context.Context, c *corpus.Case) (caseResult, error) {
	res := caseResult{versions: make(map[corpus.Provider]string)}
	if c.Schema == nil {
		return res, nil
	}
	schemaRows, err := m.measurePhase(ctx, c, corpus.SchemaPhase, "", &res)
	if err != nil {
		return caseResult{}, err
	}
	res.rows = schemaRows
	accepted := acceptedProviders(schemaRows)
	if len(accepted) == 0 {
		return res, nil
	}
	for _, inst := range c.Instances {
		rows, err := m.measurePhase(ctx, c, inst.TestName, inst.File, &res)
		if err != nil {
			return caseResult{}, err
		}
		res.rows = append(res.rows, slices.DeleteFunc(rows, func(r corpus.Row) bool {
			return !slices.Contains(accepted, r.Provider)
		})...)
	}
	return res, nil
}

func acceptedProviders(schemaRows []corpus.Row) []corpus.Provider {
	var accepted []corpus.Provider
	for _, r := range schemaRows {
		if r.Verdict == corpus.SchemaValid {
			accepted = append(accepted, r.Provider)
		}
	}
	return accepted
}

func (m *measurer) measurePhase(ctx context.Context, c *corpus.Case, phase, instance string, res *caseResult) ([]corpus.Row, error) {
	args := runArgs(c, instance)
	key, err := cacheKey(m.digest, c, args)
	if err != nil {
		return nil, err
	}
	cached := filepath.Join(m.cache, key[:2], key+".tsv")
	out, err := os.ReadFile(cached) //nolint:gosec // The cache path is derived from a content hash.
	if err != nil {
		out, err = m.runOracles(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("phase %s: %w", phase, err)
		}
	}
	rows, versions, err := parseRun(c.ID, phase, out)
	if err != nil {
		return nil, fmt.Errorf("phase %s: %w", phase, err)
	}
	if slices.ContainsFunc(rows, func(r corpus.Row) bool { return r.Verdict == corpus.Error }) {
		res.errored = true
	} else if err := writeFile(cached, out); err != nil {
		return nil, err
	}
	maps.Copy(res.versions, versions)
	return rows, nil
}

// runArgs returns the run.sh arguments for one phase: every schema root in
// manifest order, then the instance. run.sh mounts the deepest directory
// containing every argument.
func runArgs(c *corpus.Case, instance string) []string {
	var args []string
	for _, doc := range c.Schema.Documents[1:] {
		args = append(args, "--schema", filepath.Join("tests", doc.File))
	}
	args = append(args, filepath.Join("tests", c.Schema.Documents[0].File))
	if instance != "" {
		args = append(args, filepath.Join("tests", instance))
	}
	return args
}

func cacheKey(digest string, c *corpus.Case, args []string) (string, error) {
	h := sha256.New()
	writeField(h, digest)
	writeField(h, c.ID)
	for _, a := range args {
		writeField(h, a)
	}
	files := make([]string, 0, len(c.Files))
	for _, f := range c.Files {
		files = append(files, f.File)
	}
	slices.Sort(files)
	for _, name := range slices.Compact(files) {
		data, err := os.ReadFile(filepath.Join("tests", filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		writeField(h, name)
		writeField(h, string(data))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeField(h hash.Hash, s string) {
	h.Write(fmt.Appendf(nil, "%d:%s,", len(s), s))
}

func (m *measurer) runOracles(ctx context.Context, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tools/oracles/run.sh", args...) //nolint:gosec // Arguments are inventory paths.
	cmd.Env = append(os.Environ(), "XSD_ORACLES_IMAGE="+m.imageID, "XSD_ORACLES_SKIP_BUILD=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run.sh %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return out, nil
}

// parseRun converts run.sh output, a header and one row per provider, into
// observation rows, and returns the version each provider reported.
func parseRun(caseID, phase string, out []byte) ([]corpus.Row, map[corpus.Provider]string, error) {
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 1+len(corpus.Providers()) || lines[0] != "oracle\tversion\tverdict\tmessage" {
		return nil, nil, fmt.Errorf("unexpected run.sh output %q", out)
	}
	rows := make([]corpus.Row, 0, len(lines)-1)
	versions := make(map[corpus.Provider]string)
	for _, line := range lines[1:] {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return nil, nil, fmt.Errorf("unexpected run.sh row %q", line)
		}
		r, err := corpus.ParseRow(caseID + "\t" + phase + "\t" + fields[0] + "\t" + fields[2])
		if err != nil {
			return nil, nil, err
		}
		if fields[1] != "?" {
			versions[r.Provider] = fields[1]
		}
		rows = append(rows, r)
	}
	return rows, versions, nil
}

// providerVersions returns the one version each provider reported across
// every run.
func providerVersions(results map[string]caseResult) (map[corpus.Provider]string, error) {
	versions := make(map[corpus.Provider]string)
	for id, res := range results {
		for p, v := range res.versions {
			if prev, ok := versions[p]; ok && prev != v {
				return nil, fmt.Errorf("%s reported %s version %q and %q", id, p, prev, v)
			}
			versions[p] = v
		}
	}
	for _, p := range corpus.Providers() {
		if versions[p] == "" {
			return nil, fmt.Errorf("no run reported a %s version", p)
		}
	}
	return versions, nil
}

// unmeasuredRows returns the existing rows of every case not in results. They
// must come from the image and provider versions of profile.
func unmeasuredRows(inv *corpus.Inventory, results map[string]caseResult, profile corpus.Profile) ([]corpus.Row, error) {
	obs, err := corpus.LoadObservations(os.DirFS("tests"), inv)
	if err != nil {
		return nil, err
	}
	if obs.Profile.Image != profile.Image {
		return nil, fmt.Errorf("image inputs are %s in %s but %s now; measure every project case", obs.Profile.Image, corpus.ProfilePath, profile.Image)
	}
	for _, p := range corpus.Providers() {
		if obs.Profile.Providers[p] != profile.Providers[p] {
			return nil, fmt.Errorf("%s is %q in %s but %q now; measure every project case", p, obs.Profile.Providers[p], corpus.ProfilePath, profile.Providers[p])
		}
	}
	return slices.DeleteFunc(obs.Rows, func(r corpus.Row) bool {
		_, measured := results[r.CaseID]
		return measured
	}), nil
}

// write replaces every observation file with the rows of results and kept,
// and the profile with profile unless it is nil.
func write(inv *corpus.Inventory, results map[string]caseResult, kept []corpus.Row, profile *corpus.Profile) error {
	rows := kept
	for _, res := range results {
		rows = append(rows, res.rows...)
	}
	files := make(map[string][]corpus.Row)
	for _, name := range inv.ObservationPaths() {
		files[name] = nil
	}
	for _, r := range rows {
		name, err := corpus.ObservationPath(r.CaseID)
		if err != nil {
			return err
		}
		files[name] = append(files[name], r)
	}
	for name, shardRows := range files {
		if err := writeFile(filepath.Join("tests", filepath.FromSlash(name)), corpus.FormatRows(shardRows)); err != nil {
			return err
		}
	}
	if profile == nil {
		return nil
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join("tests", filepath.FromSlash(corpus.ProfilePath)), append(data, '\n'))
}

func writeFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return err
	}
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { //nolint:gosec // Observation files are checked-in repository files.
		return err
	}
	return os.Rename(tmp, name)
}
