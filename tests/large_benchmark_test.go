package tests_test

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	defaultLargeBenchmarkIdentityRows = 100_000
	defaultLargeBenchmarkRuns         = 20
)

var defaultLargeBenchmarkSizes = []largeBenchmarkSize{
	{name: "20MB", bytes: 20 * 1024 * 1024},
	{name: "100MB", bytes: 100 * 1024 * 1024},
	{name: "500MB", bytes: 500 * 1024 * 1024},
	{name: "1GB", bytes: 1 << 30},
	{name: "2GB", bytes: 2 << 30},
}

type largeBenchmarkSize struct {
	name  string
	bytes int64
}

type largeBenchmarkConfig struct {
	dir          string
	keep         bool
	sizes        []largeBenchmarkSize
	identityRows int
	runs         int
}

type largeBenchmarkProfile struct {
	name               string
	schema             string
	xml                string
	bytes              int64
	maxIdentityEntries int
}

type commandMetrics struct {
	elapsed     time.Duration
	maxRSSBytes uint64
	statistic   string
}

type largeBenchmarkResult struct {
	name    string
	metrics commandMetrics
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func TestLargeXMLLintBenchmark(t *testing.T) {
	if os.Getenv("XSD_LARGE_BENCHMARK") != "1" {
		t.Skip("set XSD_LARGE_BENCHMARK=1")
	}
	cfg := largeBenchmarkConfigFromEnv(t)
	if err := os.MkdirAll(cfg.dir, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Logf("large benchmark dir: %s", cfg.dir)
	if cfg.keep {
		t.Logf("keeping generated files in %s", cfg.dir)
	}
	t.Logf("large benchmark runs: %d", cfg.runs)

	libraryXMLLint := largeLibraryCommand(t)
	var results []largeBenchmarkResult

	streamingSchema := filepath.Join(cfg.dir, "streaming", "schema.xsd")
	writeFileString(t, streamingSchema, largeStreamingSchema)
	for _, size := range cfg.sizes {
		if !t.Run("streaming/"+size.name, func(t *testing.T) {
			dir := filepath.Join(cfg.dir, "streaming", size.name)
			if !cfg.keep {
				defer removeAll(t, dir)
			}
			profile := generateStreamingProfile(t, streamingSchema, dir, size)
			results = append(results, runLargeBenchmarkProfile(t, libraryXMLLint, profile, cfg.runs))
		}) {
			return
		}
	}

	if !t.Run("identity", func(t *testing.T) {
		dir := filepath.Join(cfg.dir, "identity")
		if !cfg.keep {
			defer removeAll(t, dir)
		}
		profile := generateIdentityProfile(t, dir, cfg.identityRows)
		results = append(results, runLargeBenchmarkProfile(t, libraryXMLLint, profile, cfg.runs))
	}) {
		return
	}
	logLargeBenchmarkSummary(t, results)
}

// Frozen generated inputs. Gates compare this harness across commits, so a
// generator change must fail the run instead of silently changing the workload.
const (
	largeStreamingBenchmarkBytes = 20 << 20
	largeStreamingDocumentLen    = 20971534
	largeStreamingDocumentSHA256 = "66521e4a59810161b4e5ab94c2b78dcfc3e9a2e03c23dc632122108b701c6f3e"
	largeStreamingSchemaSHA256   = "5c5662fe30a483fe7ffa098784d658d3b20ca96027fcc78d5ab37c2124c9eba4"
	largeIdentityDocumentLen     = 4766707
	largeIdentityDocumentSHA256  = "e68902788f6296696e441f853b989ab5a8003c1bc7e9bda7feb1444b80842d34"
	largeIdentitySchemaSHA256    = "cfea58d20827f4d46aa0f904e892699e494969f7232d5b21f2e1929552afd072"
	largeStreamingShortReadBytes = 1 << 10
	largePeakHeapSampleInterval  = time.Millisecond
)

// BenchmarkLargeStreamingValidation validates the 20 MiB streaming fixture
// from memory, excluding generation and compilation. memory_short_1KiB serves
// at most 1 KiB per Read, below the parser's initial window.
func BenchmarkLargeStreamingValidation(b *testing.B) {
	var buf bytes.Buffer
	buf.Grow(largeStreamingDocumentLen)
	writeLargeStreamingDocument(b, &buf, largeStreamingBenchmarkBytes)
	data := buf.Bytes()
	requireFrozenInput(b, "streaming document", data, largeStreamingDocumentLen, largeStreamingDocumentSHA256)
	requireFrozenInput(b, "streaming schema", []byte(largeStreamingSchema), len(largeStreamingSchema), largeStreamingSchemaSHA256)
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(largeStreamingSchema)))
	if err != nil {
		b.Fatal(err)
	}
	opts := xsd.ValidateOptions{MaxInstanceBytes: int64(len(data))}
	requireStreamingValidationWork(b, engine, data)
	for _, shape := range []struct {
		name  string
		chunk int
	}{
		{name: "memory"},
		{name: "memory_short_1KiB", chunk: largeStreamingShortReadBytes},
	} {
		b.Run(shape.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				r := &benchmarkInputReader{r: bytes.NewReader(data), chunk: shape.chunk}
				if err := engine.ValidateWithOptions(r, opts); err != nil {
					b.Fatal(err)
				}
				if r.n != int64(len(data)) {
					b.Fatalf("validator read %d bytes, want %d", r.n, len(data))
				}
			}
		})
	}
}

// streamingDefect breaks one schema construct in the last row that exercises
// it: the content between the last open and the following close is replaced
// by value, or the whole span is deleted when value is nil.
type streamingDefect struct {
	name        string
	open, close string
	value       func(old string) string
	message     string // empty means the diagnostic quotes the replacement
	code        xsderrors.Code
}

func replaceWith(s string) func(string) string { return func(string) string { return s } }

var streamingDefects = []streamingDefect{
	{name: "enumeration", open: ` status="`, close: `"`, value: replaceWith("lost"), code: xsderrors.CodeValidationFacet},
	{name: "pattern", open: ` code="`, close: `"`, value: func(old string) string { return "zz" + old[2:] }, code: xsderrors.CodeValidationFacet},
	{name: "fixed attribute", open: ` fixed="`, close: `"`, value: replaceWith("v2"), code: xsderrors.CodeValidationAttribute},
	{name: "decimal bound", open: `<amount>`, close: `</amount>`, value: replaceWith("1000000.00"), code: xsderrors.CodeValidationFacet},
	{name: "union", open: `<flag>`, close: `</flag>`, value: replaceWith("maybe"), code: xsderrors.CodeValidationFacet},
	{name: "list item", open: `<tags>`, close: `</tags>`, value: replaceWith("alpha b@d"), message: "b@d", code: xsderrors.CodeValidationFacet},
	{name: "int", open: `<id>`, close: `</id>`, value: replaceWith("x1"), code: xsderrors.CodeValidationFacet},
	{name: "missing extension member", open: `<extra>`, close: `</extra>`, message: "extra", code: xsderrors.CodeValidationContent},
	{name: "missing all member", open: `<created>`, close: `</created>`, message: "created", code: xsderrors.CodeValidationContent},
	{name: "nil with content", open: `<optional xsi:nil="true"`, close: `>`, value: replaceWith(`>x</optional`), message: `xsi:nil="true"`, code: xsderrors.CodeValidationNil},
}

// requireStreamingValidationWork proves, before any timer, that both read
// shapes validate every construct the rows exercise: each defect is reported
// once, inside the row that carries it, after the whole input is read, and the
// engine then accepts the valid document again.
func requireStreamingValidationWork(b *testing.B, engine *xsd.Engine, data []byte) {
	b.Helper()
	shapes := []int{0, largeStreamingShortReadBytes}
	validate := func(doc []byte, chunk int) error {
		r := &benchmarkInputReader{r: bytes.NewReader(doc), chunk: chunk}
		err := engine.ValidateWithOptions(r, xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))})
		if r.n != int64(len(doc)) {
			b.Fatalf("chunk %d: validator read %d bytes, want %d", chunk, r.n, len(doc))
		}
		return err
	}
	for _, chunk := range shapes {
		if err := validate(data, chunk); err != nil {
			b.Fatalf("chunk %d: valid streaming document: %v", chunk, err)
		}
	}
	for _, defect := range streamingDefects {
		start := bytes.LastIndex(data, []byte(defect.open))
		if start < 0 {
			b.Fatalf("%s: %s not in the document", defect.name, defect.open)
		}
		inner := start + len(defect.open)
		end := bytes.Index(data[inner:], []byte(defect.close))
		if end < 0 {
			b.Fatalf("%s: %s is not closed", defect.name, defect.open)
		}
		end += inner
		with, cut := "", end+len(defect.close)
		if defect.value != nil {
			with, start, cut = defect.value(string(data[inner:end])), inner, end
		}
		message := cmp.Or(defect.message, with)
		mutated := slices.Concat(data[:start], []byte(with), data[cut:])
		rowStart := bytes.LastIndex(mutated[:start], []byte("<record "))
		rowEnd := start + bytes.Index(mutated[start:], []byte("</record>")) + len("</record>")
		var first string
		for _, chunk := range shapes {
			err := validate(mutated, chunk)
			diagnostics := xsderrors.Flatten(err)
			if len(diagnostics) != 1 {
				b.Fatalf("%s, chunk %d: diagnostics = %d (%v), want 1", defect.name, chunk, len(diagnostics), err)
			}
			xerr, ok := errors.AsType[*xsderrors.Error](diagnostics[0])
			if !ok || xerr.Code() != defect.code || !strings.Contains(xerr.Message(), message) {
				b.Fatalf("%s, chunk %d: diagnostic %v, want %s containing %q", defect.name, chunk, diagnostics[0], defect.code, message)
			}
			if xerr.Line() != 1 || xerr.Column() <= rowStart || xerr.Column() > rowEnd {
				b.Fatalf("%s, chunk %d: diagnostic at %d:%d, want line 1 in the row at columns (%d, %d]", defect.name, chunk, xerr.Line(), xerr.Column(), rowStart, rowEnd)
			}
			if first == "" {
				first = err.Error()
			} else if err.Error() != first {
				b.Fatalf("%s: chunk %d diagnostics %q differ from whole reads %q", defect.name, chunk, err, first)
			}
		}
	}
	for _, chunk := range shapes {
		if err := validate(data, chunk); err != nil {
			b.Fatalf("chunk %d: valid streaming document after defects: %v", chunk, err)
		}
	}
}

// BenchmarkLargeIdentityValidation uses the README fixture and limits while
// excluding CLI startup and schema compilation from the measured operation.
func BenchmarkLargeIdentityValidation(b *testing.B) {
	fixture := newLargeIdentityFixture(b)
	b.SetBytes(fixture.profile.bytes)
	b.ReportAllocs()
	for b.Loop() {
		fixture.rewind(b)
		if err := fixture.engine.ValidateWithOptions(fixture.file, fixture.opts); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLargeIdentityPeakHeap reports the peak in-document HeapInuse while
// validating the identity fixture. Gate it with -benchtime=1x -count=N on the
// reported metrics only: ReadMemStats stops the world, so ns/op is perturbed.
func BenchmarkLargeIdentityPeakHeap(b *testing.B) {
	// The peak depends on GC pacing; pin it so the metric is independent of GOGC.
	defer debug.SetGCPercent(debug.SetGCPercent(100))
	fixture := newLargeIdentityFixture(b)
	var deltas, peaks []uint64
	for b.Loop() {
		fixture.rewind(b)
		runtime.GC()
		runtime.GC()
		baseline := heapInuse()
		sampler := startHeapSampler(largePeakHeapSampleInterval)
		err := fixture.engine.ValidateWithOptions(fixture.file, fixture.opts)
		peak := sampler.stop()
		if err != nil {
			b.Fatal(err)
		}
		deltas = append(deltas, peak-min(peak, baseline))
		peaks = append(peaks, peak)
	}
	b.ReportMetric(float64(medianUint64(deltas)), "peak-HeapInuse-delta-B")
	b.ReportMetric(float64(medianUint64(peaks)), "peak-HeapInuse-B")
}

// The sampler's final sample must observe memory held when it stops, even when
// no tick fires, and stop must return only after the sampling goroutine sends
// its result.
func TestPeakHeapSampler(t *testing.T) {
	const blockBytes = 32 << 20
	runtime.GC()
	runtime.GC()
	baseline := heapInuse()
	sampler := startHeapSampler(time.Hour)
	block := make([]byte, blockBytes)
	peak := sampler.stop()
	runtime.KeepAlive(block)
	if peak < baseline || peak-baseline < blockBytes*3/4 {
		t.Fatalf("peak HeapInuse = %d with baseline %d, want at least %d above baseline", peak, baseline, blockBytes*3/4)
	}
}

type largeIdentityFixture struct {
	profile largeBenchmarkProfile
	engine  *xsd.Engine
	file    *os.File
	opts    xsd.ValidateOptions
}

// newLargeIdentityFixture generates the identity fixture and proves, before
// any timer, that validation still performs the identity work: injected
// defects in the first and last references are reported in their rows, and
// the engine recovers.
func newLargeIdentityFixture(b *testing.B) largeIdentityFixture {
	b.Helper()
	profile := generateIdentityProfile(b, b.TempDir(), defaultLargeBenchmarkIdentityRows)
	schema, err := os.ReadFile(profile.schema)
	if err != nil {
		b.Fatal(err)
	}
	requireFrozenInput(b, "identity schema", schema, len(largeIdentitySchema), largeIdentitySchemaSHA256)
	engine, err := xsd.Compile(xsd.File(profile.schema))
	if err != nil {
		b.Fatal(err)
	}
	f, err := os.Open(profile.xml)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := f.Close(); err != nil {
			b.Fatal(err)
		}
	})
	fixture := largeIdentityFixture{
		profile: profile,
		engine:  engine,
		file:    f,
		opts: xsd.ValidateOptions{
			MaxInstanceBytes:   profile.bytes,
			MaxIdentityEntries: profile.maxIdentityEntries,
		},
	}
	fixture.requireIdentityWork(b)
	return fixture
}

func (f largeIdentityFixture) rewind(b *testing.B) {
	b.Helper()
	if _, err := f.file.Seek(0, io.SeekStart); err != nil {
		b.Fatal(err)
	}
}

func (f largeIdentityFixture) requireIdentityWork(b *testing.B) {
	b.Helper()
	data, err := io.ReadAll(f.file)
	if err != nil {
		b.Fatal(err)
	}
	requireFrozenInput(b, "identity document", data, largeIdentityDocumentLen, largeIdentityDocumentSHA256)
	validate := func(doc []byte) error {
		opts := f.opts
		opts.MaxInstanceBytes = int64(len(doc))
		return f.engine.ValidateWithOptions(bytes.NewReader(doc), opts)
	}
	if err := validate(data); err != nil {
		b.Fatalf("valid identity document: %v", err)
	}
	type diagnostic struct {
		code    xsderrors.Code
		message string
	}
	unresolved := func(ref string) []diagnostic {
		// The reference is both a keyref and an IDREF. Codes are sorted.
		return []diagnostic{
			{code: xsderrors.CodeValidationIdentity, message: `identity constraint "rowKey"`},
			{code: xsderrors.CodeValidationType, message: strconv.Quote(ref)},
		}
	}
	lastRow := bytes.LastIndex(data, []byte("<row "))
	for _, defect := range []struct {
		name        string
		from        string
		to          string
		last        bool
		diagnostics []diagnostic
	}{
		{name: "duplicate unique group", from: `group="g99999"`, to: `group="g0"`, last: true, diagnostics: []diagnostic{{code: xsderrors.CodeValidationIdentity, message: `"rowGroup"`}}},
		{name: "unresolved last reference", from: `ref="id99998"`, to: `ref="none"`, last: true, diagnostics: unresolved("none")},
		{name: "unresolved first reference", from: `ref="id0"`, to: `ref="missing0"`, diagnostics: unresolved("missing0")},
	} {
		at := bytes.Index(data, []byte(defect.from))
		if at < 0 || bytes.Count(data, []byte(defect.from)) != 1 || (at > lastRow) != defect.last {
			b.Fatalf("%s: %s is not unique in the expected row", defect.name, defect.from)
		}
		mutated := bytes.Replace(data, []byte(defect.from), []byte(defect.to), 1)
		rowStart := bytes.LastIndex(mutated[:at], []byte("<row "))
		rowEnd := at + bytes.Index(mutated[at:], []byte("/>")) + len("/>")
		err := validate(mutated)
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) != len(defect.diagnostics) {
			b.Fatalf("%s: diagnostics = %d (%v), want %d", defect.name, len(diagnostics), err, len(defect.diagnostics))
		}
		got := make([]*xsderrors.Error, len(diagnostics))
		for i, d := range diagnostics {
			xerr, ok := errors.AsType[*xsderrors.Error](d)
			if !ok || xerr.Line() != 1 || xerr.Column() <= rowStart || xerr.Column() > rowEnd {
				b.Fatalf("%s: diagnostic %v is not in the row at columns (%d, %d]", defect.name, d, rowStart, rowEnd)
			}
			got[i] = xerr
		}
		slices.SortFunc(got, func(x, y *xsderrors.Error) int { return strings.Compare(string(x.Code()), string(y.Code())) })
		for i, want := range defect.diagnostics {
			if got[i].Code() != want.code || !strings.Contains(got[i].Message(), want.message) {
				b.Fatalf("%s: diagnostic %v, want %s containing %s", defect.name, got[i], want.code, want.message)
			}
		}
		if err := validate(data); err != nil {
			b.Fatalf("valid identity document after %s: %v", defect.name, err)
		}
	}
}

// requireFrozenInput pins a generated benchmark input to its recorded length
// and SHA-256.
func requireFrozenInput(tb testing.TB, name string, data []byte, wantLen int, wantSHA256 string) {
	tb.Helper()
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); len(data) != wantLen || got != wantSHA256 {
		tb.Fatalf("%s = %d bytes sha256 %s, want %d bytes sha256 %s; the generator changed", name, len(data), got, wantLen, wantSHA256)
	}
}

// benchmarkInputReader counts the bytes it serves and optionally caps each Read
// at chunk bytes.
type benchmarkInputReader struct {
	r     io.Reader
	chunk int
	n     int64
}

func (r *benchmarkInputReader) Read(p []byte) (int, error) {
	if r.chunk > 0 && len(p) > r.chunk {
		p = p[:r.chunk]
	}
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

// heapSampler tracks the peak HeapInuse from one sample at start, one per
// tick, and one at stop. A transient between ticks can be missed, so the peak
// is a lower bound.
type heapSampler struct {
	done chan struct{}
	peak chan uint64
}

func startHeapSampler(interval time.Duration) heapSampler {
	s := heapSampler{done: make(chan struct{}), peak: make(chan uint64)}
	started := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		peak := heapInuse()
		close(started)
		for {
			select {
			case <-ticker.C:
				peak = max(peak, heapInuse())
			case <-s.done:
				s.peak <- max(peak, heapInuse())
				return
			}
		}
	}()
	<-started
	return s
}

// stop takes the final sample and returns the peak once the sampling
// goroutine has delivered it.
func (s heapSampler) stop() uint64 {
	close(s.done)
	return <-s.peak
}

func heapInuse() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapInuse
}

func medianUint64(values []uint64) uint64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

func largeBenchmarkConfigFromEnv(t *testing.T) largeBenchmarkConfig {
	t.Helper()
	dir := os.Getenv("XSD_LARGE_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	return largeBenchmarkConfig{
		dir:          dir,
		keep:         os.Getenv("XSD_LARGE_DIR") != "",
		sizes:        largeBenchmarkSizesFromEnv(t),
		identityRows: envInt(t, "XSD_LARGE_IDENTITY_ROWS", defaultLargeBenchmarkIdentityRows),
		runs:         envInt(t, "XSD_LARGE_RUNS", defaultLargeBenchmarkRuns),
	}
}

func largeBenchmarkSizesFromEnv(t *testing.T) []largeBenchmarkSize {
	t.Helper()
	sizeBytes := os.Getenv("XSD_LARGE_SIZE_BYTES")
	if sizeBytes == "" {
		return slices.Clone(defaultLargeBenchmarkSizes)
	}
	n := envInt64(t, "XSD_LARGE_SIZE_BYTES", 0)
	return []largeBenchmarkSize{{name: sizeLabel(n), bytes: n}}
}

func TestLargeBenchmarkDefaultSizesIncludeReadmeSizes(t *testing.T) {
	t.Setenv("XSD_LARGE_SIZE_BYTES", "")
	sizes := largeBenchmarkSizesFromEnv(t)
	want := []largeBenchmarkSize{
		{name: "20MB", bytes: 20 * 1024 * 1024},
		{name: "100MB", bytes: 100 * 1024 * 1024},
		{name: "500MB", bytes: 500 * 1024 * 1024},
		{name: "1GB", bytes: 1 << 30},
		{name: "2GB", bytes: 2 << 30},
	}
	if !slices.Equal(sizes, want) {
		t.Fatalf("largeBenchmarkSizesFromEnv() = %#v, want %#v", sizes, want)
	}
}

func TestLargeBenchmarkSizeOverrideUsesOneCustomSize(t *testing.T) {
	t.Setenv("XSD_LARGE_SIZE_BYTES", "1048576")
	sizes := largeBenchmarkSizesFromEnv(t)
	want := []largeBenchmarkSize{{name: "1MB", bytes: 1024 * 1024}}
	if !slices.Equal(sizes, want) {
		t.Fatalf("largeBenchmarkSizesFromEnv() = %#v, want %#v", sizes, want)
	}
}

func TestLargeBenchmarkRunsFromEnv(t *testing.T) {
	t.Setenv("XSD_LARGE_RUNS", "")
	if got := largeBenchmarkConfigFromEnv(t).runs; got != defaultLargeBenchmarkRuns {
		t.Fatalf("largeBenchmarkConfigFromEnv().runs = %d, want %d", got, defaultLargeBenchmarkRuns)
	}
	t.Setenv("XSD_LARGE_RUNS", "5")
	if got := largeBenchmarkConfigFromEnv(t).runs; got != 5 {
		t.Fatalf("largeBenchmarkConfigFromEnv().runs = %d, want 5", got)
	}
}

func TestRunLibrarySamplesUsesConfiguredSampleCount(t *testing.T) {
	countPath := filepath.Join(t.TempDir(), "count")
	t.Setenv("XSD_TEST_LIBRARY_COMMAND", "1")
	t.Setenv("XSD_TEST_LIBRARY_COUNT", countPath)
	command, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	_, samples := runLibrarySamples(t, 5, command, "-test.run=^TestLibraryCommandProcess$")
	if len(samples) != 5 {
		t.Fatalf("runLibrarySamples() returned %d samples, want 5", len(samples))
	}
	count, err := os.ReadFile(countPath) //nolint:gosec // Parent test owns the t.TempDir path passed to this helper process.
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", countPath, err)
	}
	if got, want := string(count), "6"; got != want {
		t.Fatalf("library command invocations = %s, want %s (one warm-up plus five samples)", got, want)
	}
}

func TestLibraryCommandProcess(t *testing.T) {
	if os.Getenv("XSD_TEST_LIBRARY_COMMAND") != "1" {
		return
	}
	path := os.Getenv("XSD_TEST_LIBRARY_COUNT")
	count := 0
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // Parent test owns the t.TempDir path passed through the environment.
		count, err = strconv.Atoi(string(data))
		if err != nil {
			t.Fatalf("Atoi(%q) error = %v", string(data), err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(count+1)), 0o600); err != nil { //nolint:gosec // Parent test owns the t.TempDir path passed through the environment.
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func TestP95CommandMetricsUsesNearestRank(t *testing.T) {
	samples := make([]commandMetrics, 10)
	for i := range samples {
		samples[i] = commandMetrics{
			elapsed:     time.Duration(i+1) * time.Second,
			maxRSSBytes: uint64((i + 1) * 1024),
		}
	}
	summary := p95CommandMetrics(samples)
	if summary.elapsed != 10*time.Second {
		t.Fatalf("p95 elapsed = %s, want 10s", summary.elapsed)
	}
	if summary.maxRSSBytes != 10*1024 {
		t.Fatalf("p95 rss = %d, want %d", summary.maxRSSBytes, 10*1024)
	}
	if summary.statistic != "p95" {
		t.Fatalf("p95 statistic = %q, want p95", summary.statistic)
	}
}

func TestCommandMetricsSummaryUsesMedianBelowTwentySamples(t *testing.T) {
	samples := make([]commandMetrics, 5)
	for i := range samples {
		samples[i] = commandMetrics{
			elapsed:     time.Duration(i+1) * time.Second,
			maxRSSBytes: uint64((i + 1) * 1024),
		}
	}
	summary := summarizeCommandMetrics(samples)
	if summary.statistic != "median" {
		t.Fatalf("summary statistic = %q, want median", summary.statistic)
	}
	if summary.elapsed != 3*time.Second {
		t.Fatalf("median elapsed = %s, want 3s", summary.elapsed)
	}
	if summary.maxRSSBytes != 3*1024 {
		t.Fatalf("median rss = %d, want %d", summary.maxRSSBytes, 3*1024)
	}
}

func envInt64(t *testing.T, name string, def int64) int64 {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		t.Fatalf("%s must be a positive integer", name)
	}
	return n
}

func envInt(t *testing.T, name string, def int) int {
	t.Helper()
	n := envInt64(t, name, int64(def))
	if n > int64(^uint(0)>>1) {
		t.Fatalf("%s is too large", name)
	}
	return int(n)
}

func generateStreamingProfile(t *testing.T, schema, dir string, size largeBenchmarkSize) largeBenchmarkProfile {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	profile := largeBenchmarkProfile{
		name:   "streaming/" + size.name,
		schema: schema,
		xml:    filepath.Join(dir, "document.xml"),
	}
	profile.bytes = writeLargeStreamingXML(t, profile.xml, size.bytes)
	return profile
}

func generateIdentityProfile(tb testing.TB, dir string, rows int) largeBenchmarkProfile {
	tb.Helper()
	if rows > math.MaxInt/5 {
		tb.Fatal("identity row count is too large to derive a validation-entry budget")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		tb.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	profile := largeBenchmarkProfile{
		name:               "identity",
		schema:             filepath.Join(dir, "schema.xsd"),
		xml:                filepath.Join(dir, "document.xml"),
		maxIdentityEntries: rows * 5,
	}
	writeFileString(tb, profile.schema, largeIdentitySchema)
	profile.bytes = writeLargeIdentityXML(tb, profile.xml, rows)
	return profile
}

func runLargeBenchmarkProfile(t *testing.T, libraryXMLLint string, profile largeBenchmarkProfile, runs int) largeBenchmarkResult {
	t.Helper()
	t.Logf("schema=%s", profile.schema)
	t.Logf("xml=%s bytes=%d", profile.xml, profile.bytes)
	args := []string{"--schema", profile.schema, "--max-instance-bytes", strconv.FormatInt(profile.bytes, 10), profile.xml}
	if profile.maxIdentityEntries != 0 {
		args = slices.Insert(args, len(args)-1, "--max-identity-entries", strconv.Itoa(profile.maxIdentityEntries))
	}
	metrics, samples := runLibrarySamples(t, runs, libraryXMLLint, args...)
	logLibraryCommandSamples(t, samples, profile.bytes)
	logCommandMetrics(t, "bin/xmllint", metrics, profile.bytes)
	return largeBenchmarkResult{
		name:    profile.name,
		metrics: metrics,
	}
}

func removeAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll(%s) error = %v", dir, err)
	}
}

func writeFileString(tb testing.TB, path, data string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		tb.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		tb.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

const largeStreamingSchema = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:large"
           targetNamespace="urn:large"
           elementFormDefault="qualified">
  <xs:simpleType name="Status">
    <xs:restriction base="xs:string">
      <xs:enumeration value="new"/>
      <xs:enumeration value="done"/>
      <xs:enumeration value="held"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]{2}[0-9]{6}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Money">
    <xs:restriction base="xs:decimal">
      <xs:minInclusive value="0.00"/>
      <xs:maxInclusive value="999999.99"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="TagList">
    <xs:list itemType="xs:NMTOKEN"/>
  </xs:simpleType>
  <xs:simpleType name="FlagOrInt">
    <xs:union memberTypes="xs:boolean xs:int"/>
  </xs:simpleType>
  <xs:complexType name="Meta">
    <xs:all>
      <xs:element name="created" type="xs:date"/>
      <xs:element name="active" type="xs:boolean" minOccurs="0"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="BaseRecord">
    <xs:sequence>
      <xs:element name="id" type="xs:int"/>
      <xs:choice>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="alias" type="xs:string"/>
      </xs:choice>
      <xs:element name="amount" type="t:Money"/>
      <xs:element name="tags" type="t:TagList"/>
      <xs:element name="flag" type="t:FlagOrInt"/>
      <xs:element name="meta" type="t:Meta"/>
      <xs:element name="optional" nillable="true" minOccurs="0" type="xs:string"/>
      <xs:any namespace="##other" minOccurs="0" processContents="skip"/>
    </xs:sequence>
    <xs:attribute name="code" type="t:Code" use="required"/>
    <xs:attribute name="status" type="t:Status" use="required"/>
    <xs:attribute name="fixed" type="xs:string" fixed="v1"/>
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="ExtendedRecord">
    <xs:complexContent>
      <xs:extension base="t:BaseRecord">
        <xs:sequence>
          <xs:element name="extra" type="xs:string"/>
        </xs:sequence>
        <xs:attribute name="kind" type="xs:string" fixed="extended"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Batch">
    <xs:sequence>
      <xs:element name="record" type="t:BaseRecord" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="batch" type="t:Batch"/>
</xs:schema>
`

func writeLargeStreamingXML(t *testing.T, path string, targetBytes int64) int64 {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // Large benchmark writes generated XML to configured output path.
	if err != nil {
		t.Fatalf("Create(%s) error = %v", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("Close(%s) error = %v", path, closeErr)
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<20)
	writeLargeStreamingDocument(t, bw, targetBytes)
	if flushErr := bw.Flush(); flushErr != nil {
		t.Fatalf("Flush(%s) error = %v", path, flushErr)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
}

// writeLargeStreamingDocument writes rows until the document reaches
// targetBytes.
func writeLargeStreamingDocument(tb testing.TB, w io.Writer, targetBytes int64) {
	tb.Helper()
	cw := &countingWriter{w: w}
	header := `<?xml version="1.0" encoding="UTF-8"?><batch xmlns="urn:large" xmlns:t="urn:large" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:f="urn:foreign">`
	footer := `</batch>`
	writeString(tb, cw, header)
	for i := int64(0); cw.n+int64(len(footer)) < targetBytes; i++ {
		writeStreamingRow(tb, cw, i)
	}
	writeString(tb, cw, footer)
}

func writeStreamingRow(tb testing.TB, w io.Writer, i int64) {
	tb.Helper()
	code := i % 1_000_000
	amount := i % 100_000
	status := "new"
	switch i % 3 {
	case 1:
		status = "done"
	case 2:
		status = "held"
	}
	if i%4 == 0 {
		writeFormatf(tb, w, `<record code="AB%06d" status="%s" fixed="v1" f:trace="trace%d">`, code, status, i)
		writeFormatf(tb, w, `<id>%d</id><name>name-%d</name><amount>%d.50</amount>`, i, i, amount)
		writeString(tb, w, `<tags>alpha beta gamma</tags><flag>true</flag><meta><active>true</active><created>2026-05-05</created></meta>`)
		writeString(tb, w, `<optional xsi:nil="true"/><f:payload>skip</f:payload></record>`)
		return
	}
	if i%4 == 1 {
		writeFormatf(tb, w, `<record xsi:type="t:ExtendedRecord" code="CD%06d" status="%s" fixed="v1" kind="extended">`, code, status)
		writeFormatf(tb, w, `<id>%d</id><alias>alias-%d</alias><amount>%d.75</amount>`, i, i, amount)
		writeString(tb, w, `<tags>delta epsilon</tags><flag>7</flag><meta><created>2026-05-05</created></meta>`)
		writeString(tb, w, `<extra>extended</extra></record>`)
		return
	}
	writeFormatf(tb, w, `<record code="EF%06d" status="%s" fixed="v1">`, code, status)
	writeFormatf(tb, w, `<id>%d</id><name>name-%d</name><amount>%d.00</amount>`, i, i, amount)
	writeString(tb, w, `<tags>zeta eta</tags><flag>false</flag><meta><created>2026-05-05</created><active>false</active></meta></record>`)
}

const largeIdentitySchema = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="group" type="xs:string" use="required"/>
            <xs:attribute name="ref" type="xs:IDREF" use="optional"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="rowKey"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
    <xs:unique name="rowGroup"><xs:selector xpath="row"/><xs:field xpath="@group"/></xs:unique>
    <xs:keyref name="rowRef" refer="rowKey"><xs:selector xpath="row"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>
`

func writeLargeIdentityXML(tb testing.TB, path string, rows int) int64 {
	tb.Helper()
	f, err := os.Create(path) //nolint:gosec // Large benchmark writes generated XML to configured output path.
	if err != nil {
		tb.Fatalf("Create(%s) error = %v", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			tb.Fatalf("Close(%s) error = %v", path, closeErr)
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<20)
	cw := &countingWriter{w: bw}
	writeString(tb, cw, `<?xml version="1.0" encoding="UTF-8"?><rows>`)
	for i := range rows {
		if i == 0 {
			writeFormatf(tb, cw, `<row id="id%d" group="g%d"/>`, i, i)
			continue
		}
		writeFormatf(tb, cw, `<row id="id%d" group="g%d" ref="id%d"/>`, i, i, i-1)
	}
	writeString(tb, cw, `</rows>`)
	if flushErr := bw.Flush(); flushErr != nil {
		tb.Fatalf("Flush(%s) error = %v", path, flushErr)
	}
	info, err := f.Stat()
	if err != nil {
		tb.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
}

func writeString(tb testing.TB, w io.Writer, s string) {
	tb.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		tb.Fatalf("WriteString() error = %v", err)
	}
}

func writeFormatf(tb testing.TB, w io.Writer, format string, args ...any) {
	tb.Helper()
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		tb.Fatalf("Fprintf() error = %v", err)
	}
}

func largeLibraryCommand(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	libraryXMLLint, err := filepath.Abs(filepath.Join(root, "bin", "xmllint"))
	if err != nil {
		t.Fatalf("Abs(bin/xmllint) error = %v", err)
	}
	info, err := os.Stat(libraryXMLLint)
	if err != nil {
		t.Fatalf("bin/xmllint not found; run make xmllint")
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("bin/xmllint is not executable; run make xmllint")
	}
	t.Logf("library xmllint: %s", libraryXMLLint)
	return libraryXMLLint
}

func runLibrarySamples(t *testing.T, runs int, libraryName string, args ...string) (commandMetrics, []commandMetrics) {
	t.Helper()
	if runs <= 0 {
		t.Fatalf("XSD_LARGE_RUNS must be a positive integer")
	}
	_ = runMeasuredCommandOnce(t, libraryName, args...)
	samples := make([]commandMetrics, runs)
	for i := range runs {
		samples[i] = runMeasuredCommandOnce(t, libraryName, args...)
	}
	if runs < 20 {
		logCommandSampleSummary(t, "bin/xmllint", samples)
	}
	return summarizeCommandMetrics(samples), samples
}

func runMeasuredCommandOnce(t *testing.T, name string, args ...string) commandMetrics {
	t.Helper()
	var metrics commandMetrics
	cmdName, cmdArgs := measuredCommand(name, args...)
	cmd := exec.CommandContext(t.Context(), cmdName, cmdArgs...) //nolint:gosec // Benchmark intentionally runs the configured library command.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	metrics.elapsed = time.Since(start)
	out := stdout.String() + stderr.String()
	metrics.maxRSSBytes = parseMaxRSS(runtime.GOOS, out)
	if err != nil {
		t.Fatalf("%s %s error = %v\n%s", cmdName, strings.Join(cmdArgs, " "), err, out)
	}
	return metrics
}

func p95CommandMetrics(samples []commandMetrics) commandMetrics {
	idx := p95Index(len(samples))
	return commandMetrics{
		elapsed:     summaryElapsed(samples, idx),
		maxRSSBytes: summaryRSS(samples, idx),
		statistic:   "p95",
	}
}

func summarizeCommandMetrics(samples []commandMetrics) commandMetrics {
	if len(samples) < 20 {
		return medianCommandMetrics(samples)
	}
	return p95CommandMetrics(samples)
}

func medianCommandMetrics(samples []commandMetrics) commandMetrics {
	idx := len(samples) / 2
	return commandMetrics{
		elapsed:     summaryElapsed(samples, idx),
		maxRSSBytes: summaryRSS(samples, idx),
		statistic:   "median",
	}
}

func summaryElapsed(samples []commandMetrics, idx int) time.Duration {
	elapsed := make([]time.Duration, len(samples))
	for i, sample := range samples {
		elapsed[i] = sample.elapsed
	}
	slices.Sort(elapsed)
	return elapsed[idx]
}

func summaryRSS(samples []commandMetrics, idx int) uint64 {
	rss := make([]uint64, len(samples))
	for i, sample := range samples {
		rss[i] = sample.maxRSSBytes
	}
	slices.Sort(rss)
	return rss[idx]
}

func p95Index(n int) int {
	return int(math.Ceil(0.95*float64(n))) - 1
}

func measuredCommand(name string, args ...string) (string, []string) {
	timePath := "/usr/bin/time"
	if _, err := os.Stat(timePath); err != nil {
		return name, args
	}
	switch runtime.GOOS {
	case "darwin":
		return timePath, append([]string{"-l", name}, args...)
	case "linux":
		return timePath, append([]string{"-v", name}, args...)
	default:
		return name, args
	}
}

func parseMaxRSS(goos, out string) uint64 {
	for line := range strings.SplitSeq(out, "\n") {
		line = trimCommandOutputSpace(line)
		switch {
		case goos == "darwin" && strings.Contains(line, "maximum resident set size"):
			first := firstCommandOutputField(line)
			if first == "" {
				continue
			}
			n, err := strconv.ParseUint(first, 10, 64)
			if err != nil {
				continue
			}
			return n
		case goos == "linux" && strings.Contains(line, "Maximum resident set size"):
			_, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			n, err := strconv.ParseUint(trimCommandOutputSpace(value), 10, 64)
			if err != nil {
				continue
			}
			return n * 1024
		}
	}
	return 0
}

func trimCommandOutputSpace(s string) string {
	start := 0
	for start < len(s) && isCommandOutputSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isCommandOutputSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func firstCommandOutputField(s string) string {
	s = trimCommandOutputSpace(s)
	for i := range len(s) {
		if isCommandOutputSpace(s[i]) {
			return s[:i]
		}
	}
	return s
}

func isCommandOutputSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

func logCommandMetrics(t *testing.T, label string, metrics commandMetrics, inputBytes int64) {
	t.Helper()
	statistic := metrics.statistic
	if statistic == "" {
		statistic = "p95"
	}
	t.Logf(
		"%s: %s_elapsed=%s %s_throughput=%0.2f MiB/s %s_max_rss_bytes=%d",
		label,
		statistic,
		metrics.elapsed,
		statistic,
		throughputMiB(inputBytes, metrics.elapsed),
		statistic,
		metrics.maxRSSBytes,
	)
}

func logCommandSampleSummary(t *testing.T, label string, samples []commandMetrics) {
	t.Helper()
	minMetrics := minCommandMetrics(samples)
	medianMetrics := medianCommandMetrics(samples)
	maxMetrics := maxCommandMetrics(samples)
	t.Logf(
		"%s samples: elapsed_min=%s elapsed_median=%s elapsed_max=%s rss_min=%s rss_median=%s rss_max=%s",
		label,
		formatBenchDuration(minMetrics.elapsed),
		formatBenchDuration(medianMetrics.elapsed),
		formatBenchDuration(maxMetrics.elapsed),
		formatBenchBytes(minMetrics.maxRSSBytes),
		formatBenchBytes(medianMetrics.maxRSSBytes),
		formatBenchBytes(maxMetrics.maxRSSBytes),
	)
}

func minCommandMetrics(samples []commandMetrics) commandMetrics {
	return commandMetrics{statistic: "min", elapsed: summaryElapsed(samples, 0), maxRSSBytes: summaryRSS(samples, 0)}
}

func maxCommandMetrics(samples []commandMetrics) commandMetrics {
	idx := len(samples) - 1
	return commandMetrics{statistic: "max", elapsed: summaryElapsed(samples, idx), maxRSSBytes: summaryRSS(samples, idx)}
}

func logLibraryCommandSamples(t *testing.T, samples []commandMetrics, inputBytes int64) {
	t.Helper()
	for i, metrics := range samples {
		t.Logf(
			"sample %02d: elapsed=%s rss=%s throughput=%0.2f MiB/s",
			i+1,
			formatBenchDuration(metrics.elapsed),
			formatBenchBytes(metrics.maxRSSBytes),
			throughputMiB(inputBytes, metrics.elapsed),
		)
	}
}

func logLargeBenchmarkSummary(t *testing.T, results []largeBenchmarkResult) {
	t.Helper()
	t.Log("")
	t.Logf("goos: %s", runtime.GOOS)
	t.Logf("goarch: %s", runtime.GOARCH)
	t.Log("pkg: github.com/jacoelho/xsd")
	logLargeBenchmarkTimeSummary(t, results)
	logLargeBenchmarkRSSSummary(t, results)
}

func logLargeBenchmarkTimeSummary(t *testing.T, results []largeBenchmarkResult) {
	t.Helper()
	statistic := largeBenchmarkStatistic(results)
	t.Log("                         |       bin/xmllint |")
	t.Logf("                         | %s sec/op        |", statistic)
	var values []float64
	for _, result := range results {
		seconds := result.metrics.elapsed.Seconds()
		values = append(values, seconds)
		t.Logf(
			"%-24s   %13s",
			result.name,
			formatBenchDuration(result.metrics.elapsed),
		)
	}
	t.Logf(
		"%-24s   %13s",
		"geomean",
		formatBenchSeconds(geomean(values)),
	)
}

func logLargeBenchmarkRSSSummary(t *testing.T, results []largeBenchmarkResult) {
	t.Helper()
	statistic := largeBenchmarkStatistic(results)
	t.Log("")
	t.Log("                         |       bin/xmllint |")
	t.Logf("                         | %s rss/op        |", statistic)
	var values []float64
	for _, result := range results {
		rss := float64(result.metrics.maxRSSBytes)
		values = append(values, rss)
		t.Logf(
			"%-24s   %13s",
			result.name,
			formatBenchBytes(result.metrics.maxRSSBytes),
		)
	}
	t.Logf(
		"%-24s   %13s",
		"geomean",
		formatBenchBytes(uint64(geomean(values))),
	)
}

func largeBenchmarkStatistic(results []largeBenchmarkResult) string {
	for _, result := range results {
		if result.metrics.statistic != "" {
			return result.metrics.statistic
		}
	}
	return "p95"
}

func throughputMiB(inputBytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(inputBytes) / 1024 / 1024 / elapsed.Seconds()
}

func geomean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		if value <= 0 {
			return 0
		}
		sum += math.Log(value)
	}
	return math.Exp(sum / float64(len(values)))
}

func formatBenchDuration(d time.Duration) string {
	return formatBenchSeconds(d.Seconds())
}

func formatBenchSeconds(seconds float64) string {
	switch {
	case seconds == 0:
		return "0s"
	case seconds < 0.001:
		return fmt.Sprintf("%.3fus", seconds*1_000_000)
	case seconds < 1:
		return fmt.Sprintf("%.3fms", seconds*1_000)
	default:
		return fmt.Sprintf("%.3fs", seconds)
	}
}

func formatBenchBytes(inputBytes uint64) string {
	switch {
	case inputBytes >= 1<<30:
		return fmt.Sprintf("%.2fGiB", float64(inputBytes)/(1<<30))
	case inputBytes >= 1<<20:
		return fmt.Sprintf("%.2fMiB", float64(inputBytes)/(1<<20))
	case inputBytes >= 1<<10:
		return fmt.Sprintf("%.2fKiB", float64(inputBytes)/(1<<10))
	default:
		return fmt.Sprintf("%dB", inputBytes)
	}
}

func sizeLabel(inputBytes int64) string {
	switch {
	case inputBytes%(1<<30) == 0:
		return fmt.Sprintf("%dGB", inputBytes/(1<<30))
	case inputBytes%(1024*1024) == 0:
		return fmt.Sprintf("%dMB", inputBytes/(1024*1024))
	default:
		return fmt.Sprintf("%dB", inputBytes)
	}
}
