// Command measure runs tools/oracles/run.sh on the corpus cases the
// observation contract covers, as corpus.Case.Measured selects them, and
// rewrites their rows in tests/oracles.
//
// Usage, from the repository root:
//
//	go run ./tools/oracles/measure [-j N] [-cache DIR] [case-id...]
//
// Without case IDs it measures every such case and rewrites every
// observation file and the profile. With case IDs it replaces only those
// cases' rows and keeps the profile, whose image digest and provider versions
// the run must match.
//
// It builds the oracle image once and runs every phase against private tags of
// that image ID, one per concurrent run, so a concurrent rebuild of the tag
// cannot change providers mid-run and an interrupted run's container can be
// found and removed. A rebuild yields a new image ID even from unchanged
// inputs, so the image is identified by the digest of its inputs, every file
// git tracks under tools/oracles outside this command, and is built from a
// context holding exactly those files. Each run's output is cached under a
// key covering that digest, the run arguments and the bytes of every argument
// and case file, so an interrupted measurement resumes where it stopped. Runs
// that report an error verdict are not cached and are retried once, serially,
// after the parallel pass.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	"github.com/jacoelho/xsd/internal/corpus"
)

// runTimeout bounds one run.sh invocation: four providers, each bounded by
// run.sh's own per-validator limit, plus container start.
const runTimeout = 15 * time.Minute

// dockerCleanupTimeout bounds each docker command that removes a container or
// tag, and the wait for an interrupted run's I/O.
const dockerCleanupTimeout = 30 * time.Second

// dockerForce makes docker rm remove a running container and docker rmi remove
// a tag whose image a container still uses.
const dockerForce = "--force"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "measure:", err)
		os.Exit(1)
	}
}

// run measures the selected cases. It removes the slot tags on every exit
// after tagging them, and reports a removal failure after the measurement's
// own error.
func run(argv []string) (err error) {
	flags := flag.NewFlagSet("measure", flag.ContinueOnError)
	jobs := flags.Int("j", 6, "concurrent run.sh invocations")
	cacheDir := flags.String("cache", "", "run output cache (default: the user cache directory)")
	if err = flags.Parse(argv); err != nil {
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
	const imageDir = "tools/oracles"
	inputs, err := readImageInputs(ctx, imageDir)
	if err != nil {
		return err
	}
	digest := imageDigest(inputs)
	imageID, err := buildImage(ctx, imageDir, inputs)
	if err != nil {
		return err
	}
	slots, err := tagSlots(ctx, imageID, *jobs)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, untagSlots(ctx, slots)) }()
	m := &measurer{digest: digest, cache: cache}
	results, err := m.measureAll(ctx, slots, cases)
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
	if all {
		for _, p := range corpus.Providers() {
			if versions[p] == "" {
				return fmt.Errorf("no run reported a %s version", p)
			}
		}
		profile := corpus.Profile{Providers: versions, Date: time.Now().UTC().Format(time.DateOnly), Image: digest}
		return write(inv, results, nil, &profile)
	}
	kept, err := unmeasuredRows(inv, results, digest, versions)
	if err != nil {
		return err
	}
	return write(inv, results, kept, nil)
}

// imageInput is one file of the oracle image's build context: its path as git
// lists it and its bytes.
type imageInput struct {
	path string
	data []byte
}

// readImageInputs reads every file git tracks under dir except this command's
// own directory, so untracked scratch files can neither change the digest nor
// enter the image.
func readImageInputs(ctx context.Context, dir string) ([]imageInput, error) {
	out, err := exec.CommandContext(ctx, "git", "ls-files", "-z", "--", dir).Output() //nolint:gosec // dir is the fixed tools/oracles path.
	if err != nil {
		return nil, fmt.Errorf("git ls-files %s: %w", dir, err)
	}
	own := dir + "/measure/"
	var inputs []imageInput
	for name := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if name == "" || strings.HasPrefix(name, own) {
			continue
		}
		in, err := readImageInput(name)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func readImageInput(name string) (imageInput, error) {
	info, err := os.Lstat(filepath.FromSlash(name))
	if err != nil {
		return imageInput{}, err
	}
	if !info.Mode().IsRegular() {
		return imageInput{}, fmt.Errorf("image input %s is not a regular file", name)
	}
	data, err := os.ReadFile(filepath.FromSlash(name))
	if err != nil {
		return imageInput{}, err
	}
	return imageInput{path: name, data: data}, nil
}

func imageDigest(inputs []imageInput) string {
	h := sha256.New()
	for _, in := range inputs {
		writeField(h, in.path)
		writeField(h, string(in.data))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// contextFileMode is the mode of every build context file. One fixed mode keeps
// the image a function of the paths and bytes imageDigest covers; it is
// executable so scripts run whatever their checkout mode.
const contextFileMode = 0o755

// buildContext returns a tar archive of inputs with paths relative to dir, the
// build context docker reads from standard input.
func buildContext(dir string, inputs []imageInput) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, in := range inputs {
		name, ok := strings.CutPrefix(in.path, dir+"/")
		if !ok {
			return nil, fmt.Errorf("image input %s is outside %s", in.path, dir)
		}
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: contextFileMode, Size: int64(len(in.data))}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(in.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// selectCases returns the measured cases named by ids, or every measured case
// when ids is empty, and whether the selection is every measured case.
func selectCases(inv *corpus.Inventory, ids []string) ([]*corpus.Case, bool, error) {
	byID := make(map[string]*corpus.Case)
	var measured []*corpus.Case
	for i := range inv.Cases {
		c := &inv.Cases[i]
		if c.Measured() {
			byID[c.ID] = c
			measured = append(measured, c)
		}
	}
	if len(ids) == 0 {
		return measured, true, nil
	}
	selected := make([]*corpus.Case, 0, len(ids))
	for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
		c, ok := byID[id]
		if !ok {
			return nil, false, fmt.Errorf("%q is not a measured case", id)
		}
		selected = append(selected, c)
	}
	return selected, len(selected) == len(measured), nil
}

// buildImage builds the image from exactly inputs, sent as the build context
// on standard input, and returns its ID.
func buildImage(ctx context.Context, dir string, inputs []imageInput) (string, error) {
	tag := os.Getenv("XSD_ORACLES_IMAGE")
	if tag == "" {
		tag = "xsd-oracles:local"
	}
	buildCtx, err := buildContext(dir, inputs)
	if err != nil {
		return "", err
	}
	//nolint:gosec // The tag is the caller's own image name, passed as one argument without a shell.
	cmd := exec.CommandContext(ctx, "docker", "build", "--quiet", "--tag", tag, "-")
	cmd.Stdin = bytes.NewReader(buildCtx)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker build: %w: %s", err, stderr.String())
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// tagSlots gives imageID one private tag per concurrent run. A container
// started from a slot's tag reports that tag as its image, so an interrupted
// run's container can be found and removed without touching any other
// container of the image.
func tagSlots(ctx context.Context, imageID string, n int) ([]string, error) {
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	slots := make([]string, n)
	for i := range slots {
		slots[i] = fmt.Sprintf("xsd-oracles-measure:%x-%d", nonce, i)
		//nolint:gosec // The image ID comes from docker build and the tag is generated here.
		if out, err := exec.CommandContext(ctx, "docker", "tag", imageID, slots[i]).CombinedOutput(); err != nil {
			return nil, errors.Join(fmt.Errorf("docker tag %s: %w: %s", slots[i], err, out), untagSlots(ctx, slots[:i]))
		}
	}
	return slots, nil
}

// untagSlots removes the slot tags. It also runs after an interrupt, so it
// ignores ctx's cancellation.
func untagSlots(ctx context.Context, slots []string) error {
	if len(slots) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dockerCleanupTimeout)
	defer cancel()
	//nolint:gosec // The tags are generated by tagSlots.
	if out, err := exec.CommandContext(ctx, "docker", append([]string{"rmi", dockerForce}, slots...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker rmi --force %s: %w: %s", strings.Join(slots, " "), err, out)
	}
	return nil
}

// removeContainers force-removes every container, running or only created,
// started from the slot tag image. It runs once the run's context is done, so
// it ignores ctx's cancellation.
func removeContainers(ctx context.Context, image string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dockerCleanupTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "ps", "--all", "--no-trunc", "--format", "{{.ID}}\t{{.Image}}").Output()
	if err != nil {
		return fmt.Errorf("docker ps: %w", err)
	}
	var ids []string
	for line := range strings.Lines(string(out)) {
		id, img, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if img == image {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	//nolint:gosec // The IDs come from docker ps.
	if out, err := exec.CommandContext(ctx, "docker", append([]string{"rm", dockerForce}, ids...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm %s: %w: %s", strings.Join(ids, " "), err, out)
	}
	return nil
}

type measurer struct {
	digest string
	cache  string
}

// caseResult is the rows of one case and the provider versions its runs
// reported.
type caseResult struct {
	versions map[corpus.Provider]string
	rows     []corpus.Row
	errored  bool
}

// measureAll measures cases with one concurrent run per slot, stopping
// scheduling on interrupt, and fails if any case failed.
func (m *measurer) measureAll(ctx context.Context, slots []string, cases []*corpus.Case) (map[string]caseResult, error) {
	col := &collector{results: make(map[string]caseResult, len(cases)), total: len(cases)}
	queue := make(chan *corpus.Case)
	var wg sync.WaitGroup
	for _, slot := range slots {
		wg.Go(func() {
			for c := range queue {
				res, err := m.measureCase(ctx, slot, c)
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
	return m.retryErrors(ctx, slots[0], cases, col.results)
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
func (m *measurer) retryErrors(ctx context.Context, slot string, cases []*corpus.Case, results map[string]caseResult) (map[string]caseResult, error) {
	for _, c := range cases {
		if !results[c.ID].errored {
			continue
		}
		res, err := m.measureCase(ctx, slot, c)
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
func (m *measurer) measureCase(ctx context.Context, slot string, c *corpus.Case) (caseResult, error) {
	res := caseResult{versions: make(map[corpus.Provider]string)}
	if c.Schema == nil {
		return res, nil
	}
	schemaRows, err := m.measurePhase(ctx, slot, c, corpus.SchemaPhase, "", &res)
	if err != nil {
		return caseResult{}, err
	}
	res.rows = schemaRows
	accepted := acceptedProviders(schemaRows)
	if len(accepted) == 0 {
		return res, nil
	}
	for _, inst := range c.Instances {
		rows, err := m.measurePhase(ctx, slot, c, inst.TestName, inst.File, &res)
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

func (m *measurer) measurePhase(ctx context.Context, slot string, c *corpus.Case, phase, instance string, res *caseResult) ([]corpus.Row, error) {
	in := phaseInputs(c, instance)
	args := in.args()
	key, err := cacheKey(m.digest, c, in)
	if err != nil {
		return nil, err
	}
	cached := filepath.Join(m.cache, key[:2], key+".tsv")
	out, err := os.ReadFile(cached) //nolint:gosec // The cache path is derived from a content hash.
	if err != nil {
		out, err = runOracles(ctx, slot, args)
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

// runInputs is the files of one phase run: the case's schema documents in
// manifest order and, for an instance phase, the instance.
type runInputs struct {
	instance string
	schemas  []string
}

func phaseInputs(c *corpus.Case, instance string) runInputs {
	in := runInputs{schemas: make([]string, 0, len(c.Schema.Documents))}
	for _, doc := range c.Schema.Documents {
		in.schemas = append(in.schemas, filepath.Join("tests", doc.File))
	}
	if instance != "" {
		in.instance = filepath.Join("tests", instance)
	}
	return in
}

// args returns the run.sh arguments: every schema document after the first as
// a --schema option, then the first, then the instance. run.sh mounts the
// deepest directory containing every argument.
func (in runInputs) args() []string {
	var args []string
	for _, doc := range in.schemas[1:] {
		args = append(args, "--schema", doc)
	}
	args = append(args, in.schemas[0])
	if in.instance != "" {
		args = append(args, in.instance)
	}
	return args
}

func (in runInputs) files() []string {
	if in.instance == "" {
		return in.schemas
	}
	return append(slices.Clip(in.schemas), in.instance)
}

// cacheKey covers the image digest, the case, the run arguments and the bytes
// of each run file and each declared case file, since a phase file need not be
// listed among the case files.
func cacheKey(digest string, c *corpus.Case, in runInputs) (string, error) {
	h := sha256.New()
	writeField(h, digest)
	writeField(h, c.ID)
	for _, a := range in.args() {
		writeField(h, a)
	}
	files := slices.Clone(in.files())
	for _, f := range c.Files {
		files = append(files, filepath.Join("tests", filepath.FromSlash(f.File)))
	}
	slices.Sort(files)
	for _, name := range slices.Compact(files) {
		data, err := os.ReadFile(name) //nolint:gosec // Names are inventory paths under tests.
		if err != nil {
			return "", err
		}
		writeField(h, filepath.ToSlash(name))
		writeField(h, string(data))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeField(h hash.Hash, s string) {
	h.Write(fmt.Appendf(nil, "%d:%s,", len(s), s))
}

// runOracles runs run.sh on the slot tag image. On timeout or interrupt the
// run.sh process, which execs the docker CLI, is killed, and once it has exited
// the slot's container is removed: the container outlives the CLI, since its
// init process ignores the signals the CLI forwards. Removing it once the CLI
// has exited means the CLI cannot create a container after the removal looked.
// A run that never started has no container to remove.
func runOracles(ctx context.Context, slot string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tools/oracles/run.sh", args...) //nolint:gosec // Arguments are inventory paths.
	cmd.Env = append(os.Environ(), "XSD_ORACLES_IMAGE="+slot, "XSD_ORACLES_SKIP_BUILD=1")
	cmd.WaitDelay = dockerCleanupTimeout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		err = errors.Join(err, ctx.Err())
		if cmd.Process != nil {
			err = errors.Join(err, removeContainers(ctx, slot))
		}
	}
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
// every run. A provider whose every run failed, which run.sh reports with
// version "?", has none.
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
	return versions, nil
}

// unmeasuredRows returns the existing rows of every measured case of inv not
// in results, dropping rows of cases inv no longer holds or no longer measures.
// The
// profile they come from must name image digest and every version in versions.
// It reads the tree without writing to it, so a refused run leaves it unchanged.
func unmeasuredRows(inv *corpus.Inventory, results map[string]caseResult, digest string, versions map[corpus.Provider]string) ([]corpus.Row, error) {
	recorded, err := recordedObservations(inv)
	if err != nil {
		return nil, err
	}
	obs, err := corpus.LoadObservations(recorded, inv)
	if err != nil {
		return nil, err
	}
	if obs.Profile.Image != digest {
		return nil, fmt.Errorf("image inputs are %s in %s but %s now; measure every case", obs.Profile.Image, corpus.ProfilePath, digest)
	}
	for _, p := range corpus.Providers() {
		if v, reported := versions[p]; reported && obs.Profile.Providers[p] != v {
			return nil, fmt.Errorf("%s is %q in %s but %q now; measure every case", p, obs.Profile.Providers[p], corpus.ProfilePath, v)
		}
	}
	members := measuredCaseIDs(inv)
	return slices.DeleteFunc(obs.Rows, func(r corpus.Row) bool {
		_, remeasured := results[r.CaseID]
		return remeasured || !members[r.CaseID]
	}), nil
}

func measuredCaseIDs(inv *corpus.Inventory) map[string]bool {
	ids := make(map[string]bool, len(inv.Cases))
	for i := range inv.Cases {
		if inv.Cases[i].Measured() {
			ids[inv.Cases[i].ID] = true
		}
	}
	return ids
}

// recordedObservations returns the profile and every observation file of inv
// in memory. An observation file that does not exist, one whose shard was
// added since the last measurement, reads as an empty file: its cases have no
// rows to keep.
func recordedObservations(inv *corpus.Inventory) (fs.FS, error) {
	tree := os.DirFS("tests")
	profile, err := fs.ReadFile(tree, corpus.ProfilePath)
	if err != nil {
		return nil, err
	}
	recorded := fstest.MapFS{corpus.ProfilePath: {Data: profile}}
	for _, name := range inv.ObservationPaths() {
		data, err := fs.ReadFile(tree, name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		recorded[name] = &fstest.MapFile{Data: data}
	}
	return recorded, nil
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
