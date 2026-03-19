package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jacoelho/xsd"
)

const (
	defaultGMLURL  = "https://download.data.public.lu/resources/inspire-annex-ii-theme-land-cover-landcoversurfaces-land-information-system-for-luxembourg-lis-l-2021/20251127-085716/lc.lisl-landcover2021-redange.gml"
	defaultGMLPath = "testdata/gml/example.gml"
	defaultXSDDir  = "testdata/gml/xsd"
)

var schemaLocAttrRe = regexp.MustCompile(`xsi:schemaLocation="([^"]+)"`)
var schemaLocRe = regexp.MustCompile(`schemaLocation\s*=\s*"([^"]+)"`)
var targetNSRe = regexp.MustCompile(`targetNamespace\s*=\s*"([^"]+)"`)

type doc struct {
	URL     string
	Content []byte
	Hash    string
	Name    string
}

type commonFlags struct {
	gmlURL        string
	gmlPath       string
	xsdDir        string
	skipDownload  bool
	forceDownload bool
}

type prepareDeps struct {
	downloadFile func(rawURL, out string) error
	crawlSchemas func(roots []string) ([]*doc, error)
}

type preparedGMLKind uint8

const (
	preparedGMLRemoteRoots preparedGMLKind = iota
	preparedGMLLocalizedValid
	preparedGMLLocalizedInvalid
)

type preparedGML struct {
	kind          preparedGMLKind
	data          []byte
	roots         []string
	validationErr error
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "prepare":
		fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
		fs.SetOutput(stderr)
		cfg := bindCommonFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runPrepare(cfg, stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func bindCommonFlags(fs *flag.FlagSet) commonFlags {
	cfg := commonFlags{}
	fs.StringVar(&cfg.gmlURL, "gml-url", defaultGMLURL, "source URL for example.gml")
	fs.StringVar(&cfg.gmlPath, "gml-path", defaultGMLPath, "path to local GML file")
	fs.StringVar(&cfg.xsdDir, "xsd-dir", defaultXSDDir, "target directory for flattened XSD files")
	fs.BoolVar(&cfg.skipDownload, "skip-download", false, "never re-download gml when local file exists")
	fs.BoolVar(&cfg.forceDownload, "force-download", false, "always re-download gml file")
	return cfg
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: go run ./testdata/gml/setup.go <command> [flags]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  prepare   download/refresh gml and local xsd set")
}

func runPrepare(cfg commonFlags, stdout io.Writer) error {
	return runPrepareWithDeps(cfg, stdout, prepareDeps{})
}

func runPrepareWithDeps(cfg commonFlags, stdout io.Writer, deps prepareDeps) error {
	if deps.downloadFile == nil {
		deps.downloadFile = downloadFile
	}
	if deps.crawlSchemas == nil {
		deps.crawlSchemas = crawlSchemas
	}
	if err := os.MkdirAll(filepath.Dir(cfg.gmlPath), 0o755); err != nil {
		return fmt.Errorf("create gml dir: %w", err)
	}
	if err := ensureCurrentGML(cfg, deps); err != nil {
		return err
	}

	state, err := classifyPreparedGML(cfg.gmlPath)
	if err != nil {
		return err
	}

	switch state.kind {
	case preparedGMLRemoteRoots:
		return rebuildFromRoots(state.data, state.roots, cfg, stdout, deps)
	case preparedGMLLocalizedValid:
		_, _ = fmt.Fprintln(stdout, "gml schemaLocation already points to local xsd; skipping schema refresh")
		return nil
	case preparedGMLLocalizedInvalid:
		if cfg.skipDownload {
			return fmt.Errorf("local schema state invalid and --skip-download is set: %w", state.validationErr)
		}
		refreshed, roots, err := refreshLocalizedGML(cfg, deps)
		if err != nil {
			return err
		}
		return rebuildFromRoots(refreshed, roots, cfg, stdout, deps)
	default:
		return fmt.Errorf("unknown prepared gml state")
	}
}

func ensureCurrentGML(cfg commonFlags, deps prepareDeps) error {
	needsDownload, err := shouldDownloadGML(cfg)
	if err != nil {
		return err
	}
	if !needsDownload {
		return nil
	}
	return downloadPreparedGML(cfg, deps, "download gml")
}

func classifyPreparedGML(gmlPath string) (preparedGML, error) {
	gml, err := os.ReadFile(gmlPath)
	if err != nil {
		return preparedGML{}, fmt.Errorf("read gml: %w", err)
	}

	roots, err := parseSchemaLocationRoots(gml)
	if err == nil {
		return preparedGML{
			kind:  preparedGMLRemoteRoots,
			data:  gml,
			roots: roots,
		}, nil
	}
	if !errors.Is(err, errNoRemoteSchemaLocations) {
		return preparedGML{}, fmt.Errorf("parse schemaLocation: %w", err)
	}

	readyErr := validateLocalPreparedState(gmlPath)
	if readyErr == nil {
		return preparedGML{kind: preparedGMLLocalizedValid}, nil
	}
	return preparedGML{
		kind:          preparedGMLLocalizedInvalid,
		validationErr: readyErr,
	}, nil
}

func refreshLocalizedGML(cfg commonFlags, deps prepareDeps) ([]byte, []string, error) {
	if err := downloadPreparedGML(cfg, deps, "download gml for refresh"); err != nil {
		return nil, nil, err
	}

	refreshed, err := os.ReadFile(cfg.gmlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read refreshed gml: %w", err)
	}

	roots, err := parseSchemaLocationRoots(refreshed)
	if err != nil {
		return nil, nil, fmt.Errorf("parse refreshed schemaLocation: %w", err)
	}
	return refreshed, roots, nil
}

func downloadPreparedGML(cfg commonFlags, deps prepareDeps, action string) error {
	downloadURL, err := canonicalizeURL(cfg.gmlURL)
	if err != nil {
		return fmt.Errorf("invalid gml url: %w", err)
	}
	if err := deps.downloadFile(downloadURL, cfg.gmlPath); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func shouldDownloadGML(cfg commonFlags) (bool, error) {
	if cfg.forceDownload {
		return true, nil
	}
	if _, err := os.Stat(cfg.gmlPath); errors.Is(err, os.ErrNotExist) {
		if cfg.skipDownload {
			return false, fmt.Errorf("gml file %s does not exist and --skip-download is set", cfg.gmlPath)
		}
		return true, nil
	}
	return false, nil
}

var errNoRemoteSchemaLocations = errors.New("no http(s) schema locations found")

func parseSchemaLocationRoots(gml []byte) ([]string, error) {
	m := schemaLocAttrRe.FindSubmatch(gml)
	if len(m) < 2 {
		return nil, fmt.Errorf("xsi:schemaLocation not found")
	}
	tok := strings.Fields(string(m[1]))
	if len(tok) < 2 {
		return nil, fmt.Errorf("xsi:schemaLocation has too few tokens")
	}
	var roots []string
	for i := 0; i+1 < len(tok); i += 2 {
		loc := tok[i+1]
		u, err := url.Parse(loc)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			roots = append(roots, u.String())
		}
	}
	if len(roots) == 0 {
		return nil, errNoRemoteSchemaLocations
	}
	return roots, nil
}

func canonicalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported url scheme %q", u.Scheme)
	}
	return u.String(), nil
}

func downloadFile(rawURL, out string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func detectEntrypointSchema(gmlPath string) (string, error) {
	b, err := os.ReadFile(gmlPath)
	if err != nil {
		return "", err
	}
	m := schemaLocAttrRe.FindSubmatch(b)
	if len(m) < 2 {
		return "", fmt.Errorf("xsi:schemaLocation not found")
	}
	tok := strings.Fields(string(m[1]))
	if len(tok) < 2 {
		return "", fmt.Errorf("xsi:schemaLocation has too few tokens")
	}
	loc := tok[1]
	if isRemoteSchemaLocation(loc) {
		loc = path.Base(loc)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(gmlPath), filepath.FromSlash(loc))), nil
}

func parseSchemaLocationPairs(gml []byte) ([][2]string, error) {
	m := schemaLocAttrRe.FindSubmatch(gml)
	if len(m) < 2 {
		return nil, fmt.Errorf("xsi:schemaLocation not found")
	}
	tok := strings.Fields(string(m[1]))
	if len(tok) < 2 {
		return nil, fmt.Errorf("xsi:schemaLocation has too few tokens")
	}
	pairs := make([][2]string, 0, len(tok)/2)
	for i := 0; i+1 < len(tok); i += 2 {
		pairs = append(pairs, [2]string{tok[i], tok[i+1]})
	}
	return pairs, nil
}

func validateLocalPreparedState(gmlPath string) error {
	gml, err := os.ReadFile(gmlPath)
	if err != nil {
		return fmt.Errorf("read gml: %w", err)
	}
	pairs, err := parseSchemaLocationPairs(gml)
	if err != nil {
		return err
	}
	if err := validateLocalSchemaHints(gmlPath, pairs); err != nil {
		return err
	}

	entry, err := detectEntrypointSchema(gmlPath)
	if err != nil {
		return err
	}
	if _, err := xsd.CompileFile(entry, xsd.NewSourceOptions(), xsd.NewBuildOptions()); err != nil {
		return fmt.Errorf("entry schema compile check failed: %w", err)
	}
	return nil
}

func validateLocalSchemaHints(gmlPath string, pairs [][2]string) error {
	baseDir := filepath.Dir(gmlPath)
	hasLocal := false
	for _, pair := range pairs {
		loc := pair[1]
		if isRemoteSchemaLocation(loc) {
			continue
		}
		hasLocal = true
		target := filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(loc)))
		if _, err := os.Stat(target); err != nil {
			return fmt.Errorf("missing local schema hint %s: %w", target, err)
		}
	}
	if !hasLocal {
		return fmt.Errorf("no local schema hints found")
	}
	return nil
}

func rebuildFromRoots(gml []byte, roots []string, cfg commonFlags, stdout io.Writer, deps prepareDeps) error {
	docs, crawlErr := deps.crawlSchemas(roots)
	if crawlErr != nil {
		return fmt.Errorf("crawl schemas: %w", crawlErr)
	}
	assignFlatNames(docs)
	applyCompatibilityPatches(docs)
	rewriteSchemaLocations(docs)
	if err := os.RemoveAll(cfg.xsdDir); err != nil {
		return fmt.Errorf("clean xsd dir: %w", err)
	}
	if err := os.MkdirAll(cfg.xsdDir, 0o755); err != nil {
		return fmt.Errorf("mkdir xsd dir: %w", err)
	}
	for _, d := range docs {
		if writeErr := os.WriteFile(filepath.Join(cfg.xsdDir, d.Name), d.Content, 0o644); writeErr != nil {
			return fmt.Errorf("write %s: %w", d.Name, writeErr)
		}
	}
	gmlUpdated := rewriteGMLSchemaLocation(string(gml), docs)
	if writeErr := os.WriteFile(cfg.gmlPath, []byte(gmlUpdated), 0o644); writeErr != nil {
		return fmt.Errorf("write gml: %w", writeErr)
	}
	_, _ = fmt.Fprintf(stdout, "schemas downloaded: %d\n", len(docs))
	_, _ = fmt.Fprintf(stdout, "gml updated: %s\n", cfg.gmlPath)
	_, _ = fmt.Fprintf(stdout, "xsd dir: %s\n", cfg.xsdDir)
	return nil
}

func isRemoteSchemaLocation(loc string) bool {
	return strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://")
}

func crawlSchemas(roots []string) ([]*doc, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	seen := map[string]bool{}
	queue := slices.Clone(roots)
	byURL := map[string]*doc{}

	for len(queue) > 0 {
		raw := queue[0]
		queue = queue[1:]
		if seen[raw] {
			continue
		}
		seen[raw] = true

		body, err := downloadWithFallback(client, raw)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", raw, err)
		}
		sum := sha256.Sum256(body)
		d := &doc{
			URL:     raw,
			Content: body,
			Hash:    hex.EncodeToString(sum[:]),
		}
		byURL[raw] = d

		for _, loc := range findSchemaLocations(body) {
			resolved, ok := resolveRef(raw, loc)
			if !ok || !strings.HasSuffix(strings.ToLower(path.Base(resolved)), ".xsd") {
				continue
			}
			if !seen[resolved] {
				queue = append(queue, resolved)
			}
		}
	}

	docs := slices.Collect(maps.Values(byURL))
	sort.Slice(docs, func(i, j int) bool { return docs[i].URL < docs[j].URL })
	return docs, nil
}

func downloadWithFallback(client *http.Client, raw string) ([]byte, error) {
	candidates := []string{raw}
	if raw == "http://portele.de/ShapeChangeAppinfo.xsd" {
		candidates = append(candidates,
			"https://portele.de/ShapeChangeAppinfo.xsd",
			"https://shapechange.net/resources/schema/ShapeChangeAppinfo.xsd",
		)
	}
	var lastErr error
	for _, c := range candidates {
		for i := range 3 {
			b, err := downloadSchema(client, c)
			if err == nil {
				return b, nil
			}
			lastErr = err
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return nil, lastErr
}

func downloadSchema(client *http.Client, raw string) ([]byte, error) {
	resp, err := client.Get(raw)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func findSchemaLocations(b []byte) []string {
	mm := schemaLocRe.FindAllSubmatch(b, -1)
	out := make([]string, 0, len(mm))
	for _, m := range mm {
		if len(m) >= 2 {
			out = append(out, string(m[1]))
		}
	}
	return out
}

func resolveRef(baseRaw, ref string) (string, bool) {
	bu, err := url.Parse(baseRaw)
	if err != nil {
		return "", false
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	if ru.Scheme != "" && ru.Scheme != "http" && ru.Scheme != "https" {
		return "", false
	}
	res := bu.ResolveReference(ru)
	res.Fragment = ""
	if res.Scheme != "http" && res.Scheme != "https" {
		return "", false
	}
	return res.String(), true
}

func assignFlatNames(docs []*doc) {
	used := map[string]string{}
	hashName := map[string]string{}
	for _, d := range docs {
		if n, ok := hashName[d.Hash]; ok {
			d.Name = n
			continue
		}
		base := path.Base(strings.TrimSpace(urlPath(d.URL)))
		if base == "" || !strings.HasSuffix(strings.ToLower(base), ".xsd") {
			base = "schema.xsd"
		}
		name := base
		if prevHash, exists := used[name]; exists && prevHash != d.Hash {
			ext := filepath.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			name = fmt.Sprintf("%s-%s%s", stem, d.Hash[:10], ext)
		}
		used[name] = d.Hash
		hashName[d.Hash] = name
		d.Name = name
	}
}

func rewriteSchemaLocations(docs []*doc) {
	byURL := map[string]*doc{}
	for _, d := range docs {
		byURL[d.URL] = d
	}
	for _, d := range docs {
		s := string(d.Content)
		s = schemaLocRe.ReplaceAllStringFunc(s, func(attr string) string {
			m := schemaLocRe.FindStringSubmatch(attr)
			if len(m) < 2 {
				return attr
			}
			old := m[1]
			resolved, ok := resolveRef(d.URL, old)
			if !ok {
				return attr
			}
			target, ok := byURL[resolved]
			if !ok {
				return attr
			}
			return strings.Replace(attr, old, target.Name, 1)
		})
		d.Content = []byte(s)
	}
}

func rewriteGMLSchemaLocation(gml string, docs []*doc) string {
	byURL := map[string]*doc{}
	for _, d := range docs {
		byURL[d.URL] = d
	}
	return schemaLocAttrRe.ReplaceAllStringFunc(gml, func(attr string) string {
		m := schemaLocAttrRe.FindStringSubmatch(attr)
		if len(m) < 2 {
			return attr
		}
		tok := strings.Fields(m[1])
		if len(tok) < 2 {
			return attr
		}
		out := make([]string, 0, len(tok))
		for i := 0; i+1 < len(tok); i += 2 {
			ns := tok[i]
			loc := tok[i+1]
			out = append(out, ns)
			if d, ok := byURL[loc]; ok {
				out = append(out, "xsd/"+d.Name)
			} else {
				out = append(out, "xsd/"+path.Base(loc))
			}
		}
		return `xsi:schemaLocation="` + strings.Join(out, " ") + `"`
	})
}

// applyCompatibilityPatches patches known upstream schemas that miss explicit imports.
func applyCompatibilityPatches(docs []*doc) {
	byURL := map[string]*doc{}
	nsToURL := map[string]string{}
	for _, d := range docs {
		byURL[d.URL] = d
		if m := targetNSRe.FindSubmatch(d.Content); len(m) >= 2 {
			ns := string(m[1])
			if ns != "" {
				if _, ok := nsToURL[ns]; !ok {
					nsToURL[ns] = d.URL
				}
			}
		}
	}
	addImport := func(schemaURL, ns, targetURL string) {
		sch, ok := byURL[schemaURL]
		if !ok {
			return
		}
		tgt, ok := byURL[targetURL]
		if !ok {
			return
		}
		txt := string(sch.Content)
		if strings.Contains(txt, `namespace="`+ns+`"`) {
			return
		}
		tag := "<import"
		if strings.Contains(txt, "<xs:import") {
			tag = "<xs:import"
		}
		line := fmt.Sprintf(`  %s namespace="%s" schemaLocation="%s"/>`+"\n", strings.TrimSuffix(tag, ""), ns, tgt.Name)
		idx := strings.Index(txt, tag)
		if idx == -1 {
			return
		}
		insAt := strings.Index(txt[idx:], "\n")
		if insAt == -1 {
			return
		}
		abs := idx + insAt + 1
		txt = txt[:abs] + line + txt[abs:]
		sch.Content = []byte(txt)
	}
	addImportNS := func(schemaURL, ns string) {
		u, ok := nsToURL[ns]
		if !ok {
			return
		}
		addImport(schemaURL, ns, u)
	}
	addConcreteSubstitution := func(schemaURL, elementLine string) {
		sch, ok := byURL[schemaURL]
		if !ok {
			return
		}
		txt := string(sch.Content)
		if strings.Contains(txt, elementLine) {
			return
		}
		idx := strings.LastIndex(txt, "</schema>")
		if idx == -1 {
			return
		}
		txt = txt[:idx] + "  " + elementLine + "\n" + txt[idx:]
		sch.Content = []byte(txt)
	}

	addImport("https://inspire.ec.europa.eu/schemas/base/4.0/BaseTypes.xsd",
		"http://www.isotc211.org/2005/gco",
		"http://schemas.opengis.net/iso/19139/20070417/gco/gco.xsd")
	addImport("https://inspire.ec.europa.eu/schemas/hy-p/5.0/HydroPhysicalWaters.xsd",
		"http://inspire.ec.europa.eu/schemas/omop/3.0",
		"https://inspire.ec.europa.eu/schemas/omop/3.0/ObservableProperties.xsd")
	addImport("https://inspire.ec.europa.eu/schemas/hy-p/5.0/HydroPhysicalWaters.xsd",
		"http://www.opengis.net/om/2.0",
		"http://schemas.opengis.net/om/2.0/observation.xsd")

	for _, u := range []string{
		"http://schemas.opengis.net/iso/19139/20070417/gmd/dataQuality.xsd",
		"http://schemas.opengis.net/iso/19139/20070417/gmd/extent.xsd",
		"http://schemas.opengis.net/iso/19139/20070417/gmd/content.xsd",
		"http://schemas.opengis.net/iso/19139/20070417/gmd/spatialRepresentation.xsd",
	} {
		addImport(u, "http://www.opengis.net/gml/3.2", "http://schemas.opengis.net/gml/3.2.1/gml.xsd")
	}

	for _, ns := range []string{
		"http://inspire.ec.europa.eu/schemas/ad/4.0",
		"http://inspire.ec.europa.eu/schemas/gn/4.0",
		"http://www.isotc211.org/2005/gmd",
		"http://inspire.ec.europa.eu/schemas/au/4.0",
		"http://inspire.ec.europa.eu/schemas/base2/2.0",
		"http://inspire.ec.europa.eu/schemas/bu-base/4.0",
	} {
		addImportNS("https://inspire.ec.europa.eu/schemas/lcn/5.0/LandCoverNomenclature.xsd", ns)
	}

	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/bu-base/4.0/BuildingsBase.xsd",
		`<element name="ConcreteBuildingPlaceholder" substitutionGroup="bu-base:AbstractBuilding" type="bu-base:AbstractBuildingType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteGeneralisedLinkPlaceholder" substitutionGroup="net:GeneralisedLink" type="net:GeneralisedLinkType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteLinkPlaceholder" substitutionGroup="net:Link" type="net:LinkType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteLinkSequencePlaceholder" substitutionGroup="net:LinkSequence" type="net:LinkSequenceType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteLinkSetPlaceholder" substitutionGroup="net:LinkSet" type="net:LinkSetType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteNetworkAreaPlaceholder" substitutionGroup="net:NetworkArea" type="net:NetworkAreaType"/>`,
	)
	addConcreteSubstitution(
		"https://inspire.ec.europa.eu/schemas/net/4.0/Network.xsd",
		`<element name="ConcreteNodePlaceholder" substitutionGroup="net:Node" type="net:NodeType"/>`,
	)
}

func urlPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.EscapedPath()
}
