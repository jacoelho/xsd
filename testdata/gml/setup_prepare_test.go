package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	testGMLSourceURL = "https://example.com/example.gml"
	testSchemaURL    = "https://example.com/root.xsd"
	testNamespace    = "urn:test"
	legacyMetaSuffix = ".source-url"
)

const testRootXSD = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:test"
    xmlns:t="urn:test"
    elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

type gmlFixture struct {
	t       *testing.T
	root    string
	gmlPath string
	xsdDir  string
}

func newGMLFixture(t *testing.T) gmlFixture {
	t.Helper()

	root := t.TempDir()
	gmlPath := filepath.Join(root, "example.gml")
	return gmlFixture{
		t:       t,
		root:    root,
		gmlPath: gmlPath,
		xsdDir:  filepath.Join(root, "xsd"),
	}
}

func (f gmlFixture) cfg() commonFlags {
	return commonFlags{
		gmlURL:  testGMLSourceURL,
		gmlPath: f.gmlPath,
		xsdDir:  f.xsdDir,
	}
}

func (f gmlFixture) xsdPath(name string) string {
	return filepath.Join(f.xsdDir, name)
}

func (f gmlFixture) legacyMetaPath() string {
	return f.gmlPath + legacyMetaSuffix
}

func (f gmlFixture) writeGML(content string) {
	f.writeFile(f.gmlPath, content)
}

func (f gmlFixture) writeXSD(name, content string) {
	f.mkdirAll(f.xsdDir)
	f.writeFile(f.xsdPath(name), content)
}

func (f gmlFixture) readGML() string {
	return f.readFile(f.gmlPath)
}

func (f gmlFixture) readFile(path string) string {
	f.t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func (f gmlFixture) writeFile(path, content string) {
	f.t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

func (f gmlFixture) mkdirAll(path string) {
	f.t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", path, err)
	}
}

func (f gmlFixture) requireFile(path string) {
	f.t.Helper()

	if _, err := os.Stat(path); err != nil {
		f.t.Fatalf("stat %s: %v", path, err)
	}
}

func (f gmlFixture) requireNoFile(path string) {
	f.t.Helper()

	if _, err := os.Stat(path); err == nil {
		f.t.Fatalf("expected %s to be absent", path)
	}
}

func TestShouldDownloadGML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(gmlFixture)
		mutateConfig    func(commonFlags) commonFlags
		wantDownload    bool
		wantErrContains string
	}{
		{
			name: "force download always wins",
			mutateConfig: func(cfg commonFlags) commonFlags {
				cfg.forceDownload = true
				return cfg
			},
			wantDownload: true,
		},
		{
			name: "missing gml with skip download returns error",
			mutateConfig: func(cfg commonFlags) commonFlags {
				cfg.skipDownload = true
				return cfg
			},
			wantErrContains: "does not exist and --skip-download is set",
		},
		{
			name: "existing gml skips download",
			setup: func(fx gmlFixture) {
				fx.writeGML("<xml/>")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fx := newGMLFixture(t)
			if tt.setup != nil {
				tt.setup(fx)
			}

			cfg := fx.cfg()
			if tt.mutateConfig != nil {
				cfg = tt.mutateConfig(cfg)
			}

			got, err := shouldDownloadGML(cfg)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("shouldDownloadGML() error = %v, want substring %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("shouldDownloadGML() error = %v", err)
			}
			if got != tt.wantDownload {
				t.Fatalf("shouldDownloadGML() = %v, want %v", got, tt.wantDownload)
			}
		})
	}
}

func TestValidateLocalPreparedState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(gmlFixture)
		wantErrContains string
	}{
		{
			name: "valid local hints and entry schema compile",
			setup: func(fx gmlFixture) {
				fx.writeXSD("root.xsd", testRootXSD)
				fx.writeGML(localRootGML("xsd/root.xsd"))
			},
		},
		{
			name: "missing local schema hint fails",
			setup: func(fx gmlFixture) {
				fx.writeGML(localRootGML("xsd/missing.xsd"))
			},
			wantErrContains: "missing local schema hint",
		},
		{
			name: "no local schema hints fails",
			setup: func(fx gmlFixture) {
				fx.writeGML(remoteRootGML(testSchemaURL))
			},
			wantErrContains: "no local schema hints found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fx := newGMLFixture(t)
			tt.setup(fx)

			err := validateLocalPreparedState(fx.gmlPath)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("validateLocalPreparedState() error = %v, want substring %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateLocalPreparedState() error = %v", err)
			}
		})
	}
}

func TestValidateExistingLocalSchemaSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(gmlFixture)
		wantErrContains string
	}{
		{
			name: "valid remote-root gml with local xsd dir",
			setup: func(fx gmlFixture) {
				fx.writeXSD("root.xsd", testRootXSD)
				fx.writeGML(remoteRootGML(testSchemaURL))
			},
		},
		{
			name: "missing local entry schema fails",
			setup: func(fx gmlFixture) {
				fx.writeGML(remoteRootGML(testSchemaURL))
			},
			wantErrContains: "missing local entry schema",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fx := newGMLFixture(t)
			tt.setup(fx)

			err := validateExistingLocalSchemaSet(fx.gmlPath, fx.xsdDir)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("validateExistingLocalSchemaSet() error = %v, want substring %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateExistingLocalSchemaSet() error = %v", err)
			}
		})
	}
}

func TestRunPrepare(t *testing.T) {
	t.Parallel()

	rebuildOutput := func(fx gmlFixture) []string {
		return []string{
			"schemas downloaded: 1",
			"gml updated: " + fx.gmlPath,
			"xsd dir: " + fx.xsdDir,
		}
	}

	tests := []struct {
		name              string
		setup             func(gmlFixture)
		mutateConfig      func(commonFlags) commonFlags
		downloadBody      string
		crawlDocs         []*doc
		wantErrContains   []string
		wantStdout        func(gmlFixture) []string
		wantDownloadCalls int
		wantCrawlCalls    int
		wantCrawlRoots    []string
		wantGMLContains   []string
		wantXSDNames      []string
	}{
		{
			name: "remote-root gml rebuilds localized schema set",
			setup: func(fx gmlFixture) {
				fx.writeGML(remoteRootGML(testSchemaURL))
			},
			crawlDocs:       []*doc{{URL: testSchemaURL, Content: []byte(testRootXSD)}},
			wantStdout:      rebuildOutput,
			wantCrawlCalls:  1,
			wantCrawlRoots:  []string{testSchemaURL},
			wantGMLContains: []string{`xsi:schemaLocation="` + testNamespace + ` xsd/root.xsd"`},
			wantXSDNames:    []string{"root.xsd"},
		},
		{
			name: "remote-root gml with valid local xsd dir skips refresh",
			setup: func(fx gmlFixture) {
				fx.writeXSD("root.xsd", testRootXSD)
				fx.writeGML(remoteRootGML(testSchemaURL))
			},
			wantStdout: func(gmlFixture) []string {
				return []string{"local xsd dir already valid; skipping schema refresh"}
			},
			wantGMLContains: []string{`xsi:schemaLocation="` + testNamespace + ` ` + testSchemaURL + `"`},
		},
		{
			name: "already localized valid gml skips refresh",
			setup: func(fx gmlFixture) {
				fx.writeXSD("root.xsd", testRootXSD)
				fx.writeGML(localRootGML("xsd/root.xsd"))
			},
			wantStdout: func(gmlFixture) []string {
				return []string{"gml schemaLocation already points to local xsd; skipping schema refresh"}
			},
			wantGMLContains: []string{`xsi:schemaLocation="` + testNamespace + ` xsd/root.xsd"`},
		},
		{
			name: "localized invalid gml with skip download returns explicit error",
			setup: func(fx gmlFixture) {
				fx.writeGML(localRootGML("xsd/missing.xsd"))
			},
			mutateConfig: func(cfg commonFlags) commonFlags {
				cfg.skipDownload = true
				return cfg
			},
			wantErrContains: []string{
				"local schema state invalid and --skip-download is set:",
				"missing local schema hint",
			},
		},
		{
			name: "localized invalid gml with download allowed redownloads and rebuilds",
			setup: func(fx gmlFixture) {
				fx.writeGML(localRootGML("xsd/missing.xsd"))
			},
			downloadBody:      remoteRootGML(testSchemaURL),
			crawlDocs:         []*doc{{URL: testSchemaURL, Content: []byte(testRootXSD)}},
			wantStdout:        rebuildOutput,
			wantDownloadCalls: 1,
			wantCrawlCalls:    1,
			wantCrawlRoots:    []string{testSchemaURL},
			wantGMLContains:   []string{`xsi:schemaLocation="` + testNamespace + ` xsd/root.xsd"`},
			wantXSDNames:      []string{"root.xsd"},
		},
		{
			name:              "missing gml downloads and rebuilds",
			downloadBody:      remoteRootGML(testSchemaURL),
			crawlDocs:         []*doc{{URL: testSchemaURL, Content: []byte(testRootXSD)}},
			wantStdout:        rebuildOutput,
			wantDownloadCalls: 1,
			wantCrawlCalls:    1,
			wantCrawlRoots:    []string{testSchemaURL},
			wantGMLContains:   []string{`xsi:schemaLocation="` + testNamespace + ` xsd/root.xsd"`},
			wantXSDNames:      []string{"root.xsd"},
		},
		{
			name: "force download refreshes even when localized gml is already valid",
			setup: func(fx gmlFixture) {
				fx.writeXSD("root.xsd", testRootXSD)
				fx.writeGML(localRootGML("xsd/root.xsd"))
			},
			mutateConfig: func(cfg commonFlags) commonFlags {
				cfg.forceDownload = true
				return cfg
			},
			downloadBody:      remoteRootGML(testSchemaURL),
			crawlDocs:         []*doc{{URL: testSchemaURL, Content: []byte(testRootXSD)}},
			wantStdout:        rebuildOutput,
			wantDownloadCalls: 1,
			wantCrawlCalls:    1,
			wantCrawlRoots:    []string{testSchemaURL},
			wantGMLContains:   []string{`xsi:schemaLocation="` + testNamespace + ` xsd/root.xsd"`},
			wantXSDNames:      []string{"root.xsd"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fx := newGMLFixture(t)
			if tt.setup != nil {
				tt.setup(fx)
			}

			cfg := fx.cfg()
			if tt.mutateConfig != nil {
				cfg = tt.mutateConfig(cfg)
			}

			var stdout bytes.Buffer
			downloadCalls := 0
			crawlCalls := 0
			deps := prepareDeps{
				downloadFile: func(rawURL, out string) error {
					downloadCalls++
					if tt.downloadBody == "" {
						t.Fatalf("unexpected download: %s -> %s", rawURL, out)
					}
					return os.WriteFile(out, []byte(tt.downloadBody), 0o644)
				},
				crawlSchemas: func(roots []string) ([]*doc, error) {
					crawlCalls++
					if tt.crawlDocs == nil {
						t.Fatalf("unexpected crawl: %v", roots)
					}
					if !slices.Equal(roots, tt.wantCrawlRoots) {
						t.Fatalf("crawl roots = %v, want %v", roots, tt.wantCrawlRoots)
					}
					return cloneDocs(tt.crawlDocs), nil
				},
			}

			err := runPrepareWithDeps(cfg, &stdout, deps)
			if len(tt.wantErrContains) > 0 {
				if err == nil {
					t.Fatalf("runPrepareWithDeps() error = nil, want non-nil")
				}
				for _, want := range tt.wantErrContains {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("runPrepareWithDeps() error = %q, want substring %q", err.Error(), want)
					}
				}
			} else if err != nil {
				t.Fatalf("runPrepareWithDeps() error = %v", err)
			}

			if downloadCalls != tt.wantDownloadCalls {
				t.Fatalf("download calls = %d, want %d", downloadCalls, tt.wantDownloadCalls)
			}
			if crawlCalls != tt.wantCrawlCalls {
				t.Fatalf("crawl calls = %d, want %d", crawlCalls, tt.wantCrawlCalls)
			}

			if tt.wantStdout != nil {
				for _, want := range tt.wantStdout(fx) {
					if !strings.Contains(stdout.String(), want) {
						t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
					}
				}
			}

			if len(tt.wantGMLContains) > 0 {
				gml := fx.readGML()
				for _, want := range tt.wantGMLContains {
					if !strings.Contains(gml, want) {
						t.Fatalf("gml = %q, want substring %q", gml, want)
					}
				}
			}

			for _, name := range tt.wantXSDNames {
				fx.requireFile(fx.xsdPath(name))
			}
			fx.requireNoFile(fx.legacyMetaPath())
		})
	}
}

func remoteRootGML(schemaURL string) string {
	return `<root xmlns="` + testNamespace + `" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="` + testNamespace + ` ` + schemaURL + `">ok</root>`
}

func localRootGML(schemaLocation string) string {
	return `<root xmlns="` + testNamespace + `" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="` + testNamespace + ` ` + schemaLocation + `">ok</root>`
}

func cloneDocs(docs []*doc) []*doc {
	cloned := make([]*doc, len(docs))
	for i, d := range docs {
		copyDoc := *d
		copyDoc.Content = bytes.Clone(d.Content)
		cloned[i] = &copyDoc
	}
	return cloned
}
