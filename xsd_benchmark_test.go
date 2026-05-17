package xsd

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
	defaultLargeCompareIdentityRows = 100_000
)

var defaultLargeCompareSizes = []largeCompareSize{
	{name: "100MB", bytes: 100 * 1024 * 1024},
	{name: "500MB", bytes: 500 * 1024 * 1024},
	{name: "1GB", bytes: 1 << 30},
	{name: "2GB", bytes: 2 << 30},
}

type largeCompareSize struct {
	name  string
	bytes int64
}

type largeCompareConfig struct {
	dir          string
	keep         bool
	sizes        []largeCompareSize
	identityRows int
}

type largeProfile struct {
	name   string
	schema string
	xml    string
	bytes  int64
}

type commandMetrics struct {
	elapsed     time.Duration
	maxRSSBytes uint64
}

type largeCompareResult struct {
	name           string
	goMetrics      commandMetrics
	libxml2Metrics commandMetrics
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

func TestLargeXMLLintComparison(t *testing.T) {
	if os.Getenv("XSD_LARGE_COMPARE") != "1" {
		t.Skip("set XSD_LARGE_COMPARE=1")
	}
	cfg := largeCompareConfigFromEnv(t)
	if err := os.MkdirAll(cfg.dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Logf("large comparison dir: %s", cfg.dir)
	if cfg.keep {
		t.Logf("keeping generated files in %s", cfg.dir)
	}

	repoXMLLint, libxml2XMLLint := largeCompareCommands(t)
	var results []largeCompareResult

	streamingSchema := filepath.Join(cfg.dir, "streaming", "schema.xsd")
	writeFileString(t, streamingSchema, largeStreamingSchema)
	for _, size := range cfg.sizes {
		if !t.Run("streaming/"+size.name, func(t *testing.T) {
			dir := filepath.Join(cfg.dir, "streaming", size.name)
			if !cfg.keep {
				defer removeAll(t, dir)
			}
			profile := generateStreamingProfile(t, streamingSchema, dir, size)
			results = append(results, compareLargeProfile(t, repoXMLLint, libxml2XMLLint, profile))
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
		results = append(results, compareLargeProfile(t, repoXMLLint, libxml2XMLLint, profile))
	}) {
		return
	}
	logLargeCompareSummary(t, results)
}

func largeCompareConfigFromEnv(t *testing.T) largeCompareConfig {
	t.Helper()
	dir := os.Getenv("XSD_LARGE_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	return largeCompareConfig{
		dir:          dir,
		keep:         os.Getenv("XSD_LARGE_DIR") != "",
		sizes:        largeCompareSizesFromEnv(t),
		identityRows: envInt(t, "XSD_LARGE_IDENTITY_ROWS", defaultLargeCompareIdentityRows),
	}
}

func largeCompareSizesFromEnv(t *testing.T) []largeCompareSize {
	t.Helper()
	sizeBytes := os.Getenv("XSD_LARGE_SIZE_BYTES")
	if sizeBytes == "" {
		return slices.Clone(defaultLargeCompareSizes)
	}
	n := envInt64(t, "XSD_LARGE_SIZE_BYTES", 0)
	return []largeCompareSize{{name: sizeLabel(n), bytes: n}}
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

func generateStreamingProfile(t *testing.T, schema, dir string, size largeCompareSize) largeProfile {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	profile := largeProfile{
		name:   "streaming/" + size.name,
		schema: schema,
		xml:    filepath.Join(dir, "document.xml"),
	}
	profile.bytes = writeLargeStreamingXML(t, profile.xml, size.bytes)
	return profile
}

func generateIdentityProfile(t *testing.T, dir string, rows int) largeProfile {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	profile := largeProfile{
		name:   "identity",
		schema: filepath.Join(dir, "schema.xsd"),
		xml:    filepath.Join(dir, "document.xml"),
	}
	writeFileString(t, profile.schema, largeIdentitySchema)
	profile.bytes = writeLargeIdentityXML(t, profile.xml, rows)
	return profile
}

func compareLargeProfile(t *testing.T, repoXMLLint, libxml2 string, profile largeProfile) largeCompareResult {
	t.Helper()
	t.Logf("schema=%s", profile.schema)
	t.Logf("xml=%s bytes=%d", profile.xml, profile.bytes)
	goMetrics := runMeasuredCommand(t, repoXMLLint, "--noout", "--huge", "--schema", profile.schema, profile.xml)
	libxml2Metrics := runMeasuredCommand(t, libxml2, "--noout", "--huge", "--schema", profile.schema, profile.xml)
	logCommandMetrics(t, "bin/xmllint", goMetrics, profile.bytes)
	logCommandMetrics(t, "libxml2-xmllint", libxml2Metrics, profile.bytes)
	return largeCompareResult{
		name:           profile.name,
		goMetrics:      goMetrics,
		libxml2Metrics: libxml2Metrics,
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
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
	f, err := os.Create(path)
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
		writeFormat(t, w, `<record code="AB%06d" status="%s" fixed="v1" f:trace="trace%d">`, code, status, i)
		writeFormat(t, w, `<id>%d</id><name>name-%d</name><amount>%d.50</amount>`, i, i, amount)
		writeString(t, w, `<tags>alpha beta gamma</tags><flag>true</flag><meta><active>true</active><created>2026-05-05</created></meta>`)
		writeString(t, w, `<optional xsi:nil="true"/><f:payload>skip</f:payload></record>`)
		return
	}
	if i%4 == 1 {
		writeFormat(t, w, `<record xsi:type="t:ExtendedRecord" code="CD%06d" status="%s" fixed="v1" kind="extended">`, code, status)
		writeFormat(t, w, `<id>%d</id><alias>alias-%d</alias><amount>%d.75</amount>`, i, i, amount)
		writeString(t, w, `<tags>delta epsilon</tags><flag>7</flag><meta><created>2026-05-05</created></meta>`)
		writeString(t, w, `<extra>extended</extra></record>`)
		return
	}
	writeFormat(t, w, `<record code="EF%06d" status="%s" fixed="v1">`, code, status)
	writeFormat(t, w, `<id>%d</id><name>name-%d</name><amount>%d.00</amount>`, i, i, amount)
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
	f, err := os.Create(path)
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
			writeFormat(t, cw, `<row id="id%d" group="g%d"/>`, i, i)
			continue
		}
		writeFormat(t, cw, `<row id="id%d" group="g%d" ref="id%d"/>`, i, i, i-1)
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

func writeFormat(t *testing.T, w io.Writer, format string, args ...any) {
	t.Helper()
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Fatalf("Fprintf() error = %v", err)
	}
}

func largeCompareCommands(t *testing.T) (string, string) {
	t.Helper()
	repoXMLLint, err := filepath.Abs(filepath.Join("bin", "xmllint"))
	if err != nil {
		t.Fatalf("Abs(bin/xmllint) error = %v", err)
	}
	repoInfo, err := os.Stat(repoXMLLint)
	if err != nil {
		t.Fatalf("bin/xmllint not found; run make xmllint")
	}
	if repoInfo.IsDir() || repoInfo.Mode()&0o111 == 0 {
		t.Fatalf("bin/xmllint is not executable; run make xmllint")
	}

	libxml2XMLLint, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatalf("libxml2 xmllint not found in PATH")
	}
	libxml2Info, err := os.Stat(libxml2XMLLint)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", libxml2XMLLint, err)
	}
	if os.SameFile(repoInfo, libxml2Info) {
		t.Fatalf("PATH xmllint resolves to bin/xmllint; put libxml2 xmllint earlier in PATH")
	}
	t.Logf("repo xmllint: %s", repoXMLLint)
	t.Logf("libxml2 xmllint: %s", libxml2XMLLint)
	return repoXMLLint, libxml2XMLLint
}

func runMeasuredCommand(t *testing.T, name string, args ...string) commandMetrics {
	t.Helper()
	var metrics commandMetrics
	cmdName, cmdArgs := measuredCommand(name, args...)
	cmd := exec.CommandContext(t.Context(), cmdName, cmdArgs...)
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
		line = strings.TrimSpace(line)
		switch {
		case goos == "darwin" && strings.Contains(line, "maximum resident set size"):
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			n, _ := strconv.ParseUint(fields[0], 10, 64)
			return n
		case goos == "linux" && strings.Contains(line, "Maximum resident set size"):
			_, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
			return n * 1024
		}
	}
	return 0
}

func logCommandMetrics(t *testing.T, label string, metrics commandMetrics, bytes int64) {
	t.Helper()
	t.Logf(
		"%s: elapsed=%s throughput=%0.2f MiB/s max_rss_bytes=%d",
		label,
		metrics.elapsed,
		throughputMiB(bytes, metrics.elapsed),
		metrics.maxRSSBytes,
	)
}

func logLargeCompareSummary(t *testing.T, results []largeCompareResult) {
	t.Helper()
	t.Log("")
	t.Logf("goos: %s", runtime.GOOS)
	t.Logf("goarch: %s", runtime.GOARCH)
	t.Log("pkg: github.com/jacoelho/xsd")
	logLargeCompareTimeSummary(t, results)
	logLargeCompareRSSSummary(t, results)
}

func logLargeCompareTimeSummary(t *testing.T, results []largeCompareResult) {
	t.Helper()
	t.Log("                         | libxml2 xmllint |             go xmllint             |")
	t.Log("                         | sec/op          | sec/op          vs base           |")
	var libxml2Values, goValues []float64
	for _, result := range results {
		libxml2Seconds := result.libxml2Metrics.elapsed.Seconds()
		goSeconds := result.goMetrics.elapsed.Seconds()
		libxml2Values = append(libxml2Values, libxml2Seconds)
		goValues = append(goValues, goSeconds)
		t.Logf(
			"%-24s   %13s   %13s   %10s",
			result.name,
			formatBenchDuration(result.libxml2Metrics.elapsed),
			formatBenchDuration(result.goMetrics.elapsed),
			percentChange(libxml2Seconds, goSeconds),
		)
	}
	t.Logf(
		"%-24s   %13s   %13s   %10s",
		"geomean",
		formatBenchSeconds(geomean(libxml2Values)),
		formatBenchSeconds(geomean(goValues)),
		percentChange(geomean(libxml2Values), geomean(goValues)),
	)
}

func logLargeCompareRSSSummary(t *testing.T, results []largeCompareResult) {
	t.Helper()
	t.Log("")
	t.Log("                         | libxml2 xmllint |             go xmllint             |")
	t.Log("                         | rss/op          | rss/op          vs base           |")
	var libxml2Values, goValues []float64
	for _, result := range results {
		libxml2RSS := float64(result.libxml2Metrics.maxRSSBytes)
		goRSS := float64(result.goMetrics.maxRSSBytes)
		libxml2Values = append(libxml2Values, libxml2RSS)
		goValues = append(goValues, goRSS)
		t.Logf(
			"%-24s   %13s   %13s   %10s",
			result.name,
			formatBenchBytes(result.libxml2Metrics.maxRSSBytes),
			formatBenchBytes(result.goMetrics.maxRSSBytes),
			percentChange(libxml2RSS, goRSS),
		)
	}
	t.Logf(
		"%-24s   %13s   %13s   %10s",
		"geomean",
		formatBenchBytes(uint64(geomean(libxml2Values))),
		formatBenchBytes(uint64(geomean(goValues))),
		percentChange(geomean(libxml2Values), geomean(goValues)),
	)
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

func percentChange(base, value float64) string {
	if base == 0 {
		return "~"
	}
	change := (value/base - 1) * 100
	if math.Abs(change) < 0.005 {
		return "~"
	}
	return fmt.Sprintf("%+.2f%%", change)
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
