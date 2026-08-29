package tests_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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

func generateIdentityProfile(t *testing.T, dir string, rows int) largeBenchmarkProfile {
	t.Helper()
	if rows > math.MaxInt/5 {
		t.Fatal("identity row count is too large to derive a validation-entry budget")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	profile := largeBenchmarkProfile{
		name:               "identity",
		schema:             filepath.Join(dir, "schema.xsd"),
		xml:                filepath.Join(dir, "document.xml"),
		maxIdentityEntries: rows * 5,
	}
	writeFileString(t, profile.schema, largeIdentitySchema)
	profile.bytes = writeLargeIdentityXML(t, profile.xml, rows)
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

func writeFileString(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
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
	cw := &countingWriter{w: bw}
	header := `<?xml version="1.0" encoding="UTF-8"?><batch xmlns="urn:large" xmlns:t="urn:large" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:f="urn:foreign">`
	footer := `</batch>`
	writeString(t, cw, header)
	for i := int64(0); cw.n+int64(len(footer)) < targetBytes; i++ {
		writeStreamingRow(t, cw, i)
	}
	writeString(t, cw, footer)
	if flushErr := bw.Flush(); flushErr != nil {
		t.Fatalf("Flush(%s) error = %v", path, flushErr)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
}

func writeStreamingRow(t *testing.T, w io.Writer, i int64) {
	t.Helper()
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
		writeFormatf(t, w, `<record code="AB%06d" status="%s" fixed="v1" f:trace="trace%d">`, code, status, i)
		writeFormatf(t, w, `<id>%d</id><name>name-%d</name><amount>%d.50</amount>`, i, i, amount)
		writeString(t, w, `<tags>alpha beta gamma</tags><flag>true</flag><meta><active>true</active><created>2026-05-05</created></meta>`)
		writeString(t, w, `<optional xsi:nil="true"/><f:payload>skip</f:payload></record>`)
		return
	}
	if i%4 == 1 {
		writeFormatf(t, w, `<record xsi:type="t:ExtendedRecord" code="CD%06d" status="%s" fixed="v1" kind="extended">`, code, status)
		writeFormatf(t, w, `<id>%d</id><alias>alias-%d</alias><amount>%d.75</amount>`, i, i, amount)
		writeString(t, w, `<tags>delta epsilon</tags><flag>7</flag><meta><created>2026-05-05</created></meta>`)
		writeString(t, w, `<extra>extended</extra></record>`)
		return
	}
	writeFormatf(t, w, `<record code="EF%06d" status="%s" fixed="v1">`, code, status)
	writeFormatf(t, w, `<id>%d</id><name>name-%d</name><amount>%d.00</amount>`, i, i, amount)
	writeString(t, w, `<tags>zeta eta</tags><flag>false</flag><meta><created>2026-05-05</created><active>false</active></meta></record>`)
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

func writeLargeIdentityXML(t *testing.T, path string, rows int) int64 {
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
	cw := &countingWriter{w: bw}
	writeString(t, cw, `<?xml version="1.0" encoding="UTF-8"?><rows>`)
	for i := range rows {
		if i == 0 {
			writeFormatf(t, cw, `<row id="id%d" group="g%d"/>`, i, i)
			continue
		}
		writeFormatf(t, cw, `<row id="id%d" group="g%d" ref="id%d"/>`, i, i, i-1)
	}
	writeString(t, cw, `</rows>`)
	if flushErr := bw.Flush(); flushErr != nil {
		t.Fatalf("Flush(%s) error = %v", path, flushErr)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
}

func writeString(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
}

func writeFormatf(t *testing.T, w io.Writer, format string, args ...any) {
	t.Helper()
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Fatalf("Fprintf() error = %v", err)
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

func logCommandMetrics(t *testing.T, label string, metrics commandMetrics, bytes int64) {
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
		throughputMiB(bytes, metrics.elapsed),
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

func logLibraryCommandSamples(t *testing.T, samples []commandMetrics, bytes int64) {
	t.Helper()
	for i, metrics := range samples {
		t.Logf(
			"sample %02d: elapsed=%s rss=%s throughput=%0.2f MiB/s",
			i+1,
			formatBenchDuration(metrics.elapsed),
			formatBenchBytes(metrics.maxRSSBytes),
			throughputMiB(bytes, metrics.elapsed),
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

func throughputMiB(bytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / 1024 / 1024 / elapsed.Seconds()
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

func formatBenchBytes(bytes uint64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2fGiB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2fMiB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.2fKiB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func sizeLabel(bytes int64) string {
	switch {
	case bytes%(1<<30) == 0:
		return fmt.Sprintf("%dGB", bytes/(1<<30))
	case bytes%(1024*1024) == 0:
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
