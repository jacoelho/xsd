package corpus

import (
	"errors"
	"io/fs"
	"maps"
	"strings"
	"testing"
	"testing/fstest"
)

const testIndex = `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","w3c/msMeta/Regex_w3c"]}`

func inventoryFS(projectCases, w3cCases string) fstest.MapFS {
	return fstest.MapFS{
		"manifest/index.json":                {Data: []byte(testIndex)},
		"manifest/project/top-level.json":    {Data: []byte(`{"cases":[` + projectCases + `]}`)},
		"manifest/w3c/msMeta/Regex_w3c.json": {Data: []byte(`{"cases":[` + w3cCases + `]}`)},
	}
}

const validProjectCase = `{"id":"project/a","expectedSource":"project",
	"schema":{"expected":"valid","documents":[{"file":"corpus/a.xsd"}],"rule":{"specRef":"https://example.test/#r","clause":"r clause 1"},
		"differences":{"xerces":{"record":"I-001"},"libxml2":{"record":"limits"}}},
	"instances":[{"testName":"i","file":"corpus/i.xml","expected":"invalid","oracle":{"project":{"expected":"valid","reason":"corrected"}}}]}`

const validW3CCase = `{"id":"w3c/msMeta/Regex_w3c.xml/g","expectedSource":"w3c","schema":{"expected":"invalid","documents":[{"file":"corpus/w.xsd"}]}}`

func TestLoadInventoryConcatenatesShardsInIndexOrder(t *testing.T) {
	inv, err := LoadInventory(inventoryFS(validProjectCase, validW3CCase))
	if err != nil {
		t.Fatalf("LoadInventory() error = %v", err)
	}
	if len(inv.Cases) != 2 || inv.Cases[0].ID != "project/a" || inv.Cases[1].ID != "w3c/msMeta/Regex_w3c.xml/g" {
		t.Fatalf("cases = %+v, want project/a then the W3C case", inv.Cases)
	}
	phases := inv.Cases[0].Phases()
	if len(phases) != 2 || phases[0].Name != SchemaPhase || phases[1].Name != "i" || phases[1].Expected != ExpectedInvalid {
		t.Fatalf("Phases() = %+v, want schema then instance i", phases)
	}
	if got := phases[0].Differences[Xerces].Record; got != "I-001" {
		t.Fatalf("schema difference for xerces = %q, want I-001", got)
	}
	if got := phases[0].Rule.Clause; got != "r clause 1" {
		t.Fatalf("schema rule clause = %q, want %q", got, "r clause 1")
	}
}

func TestLoadInventoryRejectsInvalidCases(t *testing.T) {
	for _, tt := range []struct {
		name, project, w3c, want string
	}{
		{name: "case in another shard", project: `{"id":"project/area/a","expectedSource":"project"}`, want: `case "project/area/a" is in shard "project/top-level", want "project/area"`},
		{name: "duplicate case", project: `{"id":"project/a","expectedSource":"project"},{"id":"project/a","expectedSource":"project"}`, want: `case "project/a" is duplicated`},
		{name: "unknown expected source", project: `{"id":"project/a","expectedSource":"vendor"}`, want: `unknown expectedSource "vendor"`},
		{name: "id outside its source", w3c: `{"id":"w3c/msMeta/Regex_w3c.xml/g","expectedSource":"project"}`, want: `is not prefixed by its expectedSource "project"`},
		{name: "unknown schema expected", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"unknown"}}`, want: `phase "schema": unknown expected value "unknown"`},
		{name: "unknown instance expected", project: `{"id":"project/a","expectedSource":"project","instances":[{"file":"i.xml","testName":"i","expected":""}]}`, want: `phase "i": unknown expected value ""`},
		{name: "duplicate instance", project: `{"id":"project/a","expectedSource":"project","instances":[{"file":"i.xml","testName":"i","expected":"valid"},{"file":"i.xml","testName":"i","expected":"invalid"}]}`, want: `duplicates instance "i"`},
		{name: "instance named schema", project: `{"id":"project/a","expectedSource":"project","instances":[{"file":"i.xml","testName":"schema","expected":"valid"}]}`, want: `instance test name "schema"`},
		{name: "unnamed instance", project: `{"id":"project/a","expectedSource":"project","instances":[{"file":"i.xml","expected":"valid"}]}`, want: `instance test name ""`},
		{name: "duplicate schema document", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"s.xsd"},{"file":"s.xsd"}]}}`, want: `duplicates schema document "s.xsd"`},
		{name: "provider oracle entry", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","oracle":{"xerces":{"expected":"valid","reason":"r"}}}}`, want: `unknown oracle key "xerces"`},
		{name: "oracle without reason", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","oracle":{"w3c":{"expected":"invalid","reason":" "}}}}`, want: `oracle "w3c" has no reason`},
		{name: "w3c oracle equal to the effective expectation", w3c: `{"id":"w3c/msMeta/Regex_w3c.xml/g","expectedSource":"w3c","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","oracle":{"w3c":{"expected":"valid","reason":"retained"}}}}`, want: `phase "schema": oracle "w3c" expectation "valid" equals the effective expectation`},
		{name: "project oracle equal to the effective expectation", project: `{"id":"project/a","expectedSource":"project","instances":[{"file":"i.xml","testName":"i","expected":"invalid","oracle":{"project":{"expected":"invalid","reason":"r"}}}]}`, want: `phase "i": oracle "project" expectation "invalid" equals the effective expectation`},
		{name: "oracle status", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","oracle":{"project":{"status":"blocked","reason":"r"}}}}`, want: `unknown field "status"`},
		{name: "rule without specRef", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","rule":{"clause":"c"}}}`, want: "rule has no specRef"},
		{name: "difference for unknown provider", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","differences":{"dotNet":{"record":"I-001"}}}}`, want: `difference for unknown provider "dotNet"`},
		{name: "difference with only a note", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","differences":{"dotnet":{"note":"n"}}}}`, want: `unknown field "note"`},
		{name: "malformed record", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","differences":{"dotnet":{"record":"I-1"}}}}`, want: `names record "I-1", want I-NNN or a row anchor`},
		{name: "malformed row anchor", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","differences":{"dotnet":{"record":"Temporal boundaries"}}}}`, want: `names record "Temporal boundaries"`},
		{name: "empty difference", project: `{"id":"project/a","expectedSource":"project","schema":{"documents":[{"file":"s.xsd"}],"expected":"valid","differences":{"dotnet":{}}}}`, want: "difference for dotnet has no record"},
		{name: "schema without documents", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[]}}`, want: `case "project/a" has a schema phase without documents`},
		{name: "schema document without file", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":""}]}}`, want: `case "project/a" has a schema document without a file`},
		{name: "instance without file", project: `{"id":"project/a","expectedSource":"project","instances":[{"testName":"i","expected":"valid"}]}`, want: `case "project/a" instance "i" has no file`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadInventory(inventoryFS(tt.project, tt.w3c))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadInventory() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadInventoryRejectsUndeclaredKeys(t *testing.T) {
	for _, tt := range []struct {
		name, index, project, want string
	}{
		{name: "index key", index: `{"version":"2.0","specification":"XSD 1.0","shard":["project/top-level"],"shards":["project/top-level","w3c/msMeta/Regex_w3c"]}`, project: validProjectCase, want: `manifest/index.json: json: unknown field "shard"`},
		{name: "case key", project: `{"id":"project/a","expectedSource":"project","titel":"t"}`, want: `unknown field "titel"`},
		{name: "schema key", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"invalid","errrorCode":"schema.reference","documents":[{"file":"a.xsd"}]}}`, want: `unknown field "errrorCode"`},
		{name: "instance key", project: `{"id":"project/a","expectedSource":"project","instances":[{"testName":"i","file":"i.xml","expected":"valid","rules":{"specRef":"r"}}]}`, want: `unknown field "rules"`},
		{name: "provenance key", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"invalid","documents":[{"file":"a.xsd"}],"oracle":{"project":{"expected":"valid","reason":"r","specification":"s"}}}}`, want: `unknown field "specification"`},
		{name: "difference key", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}],"differences":{"xerces":{"records":"I-001"}}}}`, want: `unknown field "records"`},
		{name: "data after the index", index: testIndex + `{}`, project: validProjectCase, want: "manifest/index.json: data after the JSON value"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fsys := inventoryFS(tt.project, validW3CCase)
			if tt.index != "" {
				fsys["manifest/index.json"] = &fstest.MapFile{Data: []byte(tt.index)}
			}
			_, err := LoadInventory(fsys)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadInventory() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCaseMeasured(t *testing.T) {
	override := func(original Expectation) map[string]Provenance {
		return map[string]Provenance{SourceW3C: {Expected: original, Reason: "r"}}
	}
	for _, tt := range []struct {
		name string
		c    Case
		want bool
	}{
		{name: "project case", c: Case{ExpectedSource: SourceProject}, want: true},
		{name: "W3C case without override", c: Case{ExpectedSource: SourceW3C, Schema: &Schema{Expected: ExpectedValid}}, want: false},
		{name: "W3C schema override", c: Case{ExpectedSource: SourceW3C, Schema: &Schema{Expected: ExpectedInvalid, Oracle: override(ExpectedValid)}}, want: true},
		{name: "W3C instance override", c: Case{ExpectedSource: SourceW3C, Schema: &Schema{Expected: ExpectedValid}, Instances: []Instance{
			{TestName: "a", Expected: ExpectedValid},
			{TestName: "b", Expected: ExpectedInvalid, Oracle: override(ExpectedValid)},
		}}, want: true},
		{name: "W3C case with a project correction only", c: Case{ExpectedSource: SourceW3C, Schema: &Schema{Expected: ExpectedInvalid, Oracle: map[string]Provenance{SourceProject: {Expected: ExpectedValid, Reason: "r"}}}}, want: false},
		{name: "Xerces-J case with a W3C-keyed override", c: Case{ExpectedSource: SourceXercesJ, Schema: &Schema{Expected: ExpectedInvalid, Oracle: override(ExpectedValid)}}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Measured(); got != tt.want {
				t.Fatalf("Measured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadInventoryRejectsBadIndex(t *testing.T) {
	for _, tt := range []struct {
		name, index, want string
	}{
		{name: "no shards", index: `{"version":"2.0","specification":"XSD 1.0","shards":[]}`, want: "needs a version, a specification and shards"},
		{name: "repeated shard", index: `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/top-level"]}`, want: `lists shard "project/top-level" twice`},
		{name: "missing shard file", index: `{"version":"2.0","specification":"XSD 1.0","shards":["project/absent"]}`, want: "manifest/project/absent.json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fsys := inventoryFS(validProjectCase, validW3CCase)
			fsys["manifest/index.json"] = &fstest.MapFile{Data: []byte(tt.index)}
			_, err := LoadInventory(fsys)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadInventory() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestShardName(t *testing.T) {
	for _, tt := range []struct{ id, want string }{
		{"project/name", "project/top-level"},
		{"project/datatypes/numeric/x", "project/datatypes"},
		{"xerces-j/idc/idc_1", "xerces-j"},
		{"w3c/msMeta/Regex_w3c.xml/g", "w3c/msMeta/Regex_w3c"},
		{"w3c/sunMeta/ElemDecl.testSet/g", "w3c/sunMeta/ElemDecl"},
		{"w3c/nistMeta/NISTXMLSchemaDatatypes.testSet/list-ID-length-1", "w3c/nistMeta/NISTXMLSchemaDatatypes-list"},
	} {
		got, err := ShardName(tt.id)
		if err != nil || got != tt.want {
			t.Errorf("ShardName(%q) = %q, %v; want %q", tt.id, got, err, tt.want)
		}
	}
	for _, id := range []string{"project", "w3c/msMeta/Regex_w3c.xml", "vendor/a/b"} {
		if _, err := ShardName(id); err == nil {
			t.Errorf("ShardName(%q) error = nil, want an error", id)
		}
	}
}

const testProfile = `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"sha256:0"}`

func observationFS(rows string) fstest.MapFS {
	fsys := inventoryFS(validProjectCase, validW3CCase)
	fsys["oracles/profile.json"] = &fstest.MapFile{Data: []byte(testProfile)}
	fsys["oracles/top-level.tsv"] = &fstest.MapFile{Data: []byte(rows)}
	fsys["oracles/w3c.tsv"] = &fstest.MapFile{}
	return fsys
}

func TestLoadObservations(t *testing.T) {
	data := "project/a\ti\txerces\tinvalid\ti.xml:1: bad\nproject/a\tschema\tdotnet\terror\t\nproject/a\tschema\txerces\tschema-valid\t\n"
	fsys := observationFS(data)
	inv, err := LoadInventory(fsys)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := LoadObservations(fsys, &inv)
	if err != nil {
		t.Fatalf("LoadObservations() error = %v", err)
	}
	want := []Row{
		{CaseID: "project/a", Phase: "i", Provider: Xerces, Verdict: Invalid, Message: "i.xml:1: bad"},
		{CaseID: "project/a", Phase: "schema", Provider: DotNet, Verdict: Error},
		{CaseID: "project/a", Phase: "schema", Provider: Xerces, Verdict: SchemaValid},
	}
	if len(obs.Rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", obs.Rows, want)
	}
	for i := range want {
		if obs.Rows[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, obs.Rows[i], want[i])
		}
	}
	if obs.Profile.Providers[LibXML2] != "libxml2 2.9.13" || obs.Profile.Date != "2026-11-19" {
		t.Fatalf("profile = %+v", obs.Profile)
	}
	shuffled := []Row{want[2], want[0], want[1]}
	got, err := FormatRows(shuffled)
	if err != nil || string(got) != data {
		t.Fatalf("FormatRows() = %q, %v; want the sorted file %q", got, err, data)
	}
}

func TestFormatRowsRejectsRowsThatDoNotLoad(t *testing.T) {
	valid := Row{CaseID: "project/a", Phase: SchemaPhase, Provider: Xerces, Verdict: SchemaValid}
	for _, tt := range []struct {
		name string
		row  Row
		want string
	}{
		{name: "tab in message", row: Row{CaseID: "project/a", Phase: "i", Provider: Xerces, Verdict: Invalid, Message: "a\tb"}, want: "a tab or a line break in its message"},
		{name: "line break in message", row: Row{CaseID: "project/a", Phase: "i", Provider: Xerces, Verdict: Invalid, Message: "a\nb"}, want: "a tab or a line break in its message"},
		{name: "line break in phase", row: Row{CaseID: "project/a", Phase: "i\r", Provider: Xerces, Verdict: Invalid}, want: "holds a tab or a line break"},
		{name: "tab in case", row: Row{CaseID: "project/a\tb", Phase: "i", Provider: Xerces, Verdict: Invalid}, want: "holds a tab or a line break"},
		{name: "empty phase", row: Row{CaseID: "project/a", Provider: Xerces, Verdict: Invalid}, want: "empty case or phase"},
		{name: "unknown provider", row: Row{CaseID: "project/a", Phase: "i", Provider: "saxon", Verdict: Invalid}, want: `unknown provider "saxon"`},
		{name: "unknown verdict", row: Row{CaseID: "project/a", Phase: "i", Provider: Xerces, Verdict: "accepted"}, want: `unknown verdict "accepted"`},
		{name: "absolute path", row: Row{CaseID: "project/a", Phase: "i", Provider: Xerces, Verdict: Invalid, Message: "file:///x.xml: bad"}, want: "an absolute path in its message"},
		{name: "repeated provider", row: Row{CaseID: "project/a", Phase: SchemaPhase, Provider: Xerces, Verdict: SchemaInvalid}, want: `case "project/a" phase "schema" has two rows for xerces`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatRows([]Row{valid, tt.row})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("FormatRows() = %q, %v; want an error containing %q", got, err, tt.want)
			}
		})
	}
}

func TestFormatProfileLoadsBack(t *testing.T) {
	p := Profile{
		Providers: map[Provider]string{Xerces: "Xerces-J 2.12.2", DotNet: ".NET 10.0.14", XMLSchema: "xmlschema 4.3.2", LibXML2: "libxml2 2.9.13"},
		Date:      "2026-11-19",
		Image:     "sha256:0",
	}
	data, err := FormatProfile(p)
	if err != nil {
		t.Fatalf("FormatProfile() error = %v", err)
	}
	want := `{
  "providers": {
    "dotnet": ".NET 10.0.14",
    "libxml2": "libxml2 2.9.13",
    "xerces": "Xerces-J 2.12.2",
    "xmlschema": "xmlschema 4.3.2"
  },
  "date": "2026-11-19",
  "image": "sha256:0"
}
`
	if string(data) != want {
		t.Fatalf("FormatProfile() = %q, want %q", data, want)
	}
	fsys := observationFS("")
	fsys["oracles/profile.json"] = &fstest.MapFile{Data: data}
	inv, err := LoadInventory(fsys)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := LoadObservations(fsys, &inv)
	if err != nil {
		t.Fatalf("LoadObservations() error = %v", err)
	}
	if !maps.Equal(obs.Profile.Providers, p.Providers) || obs.Profile.Date != p.Date || obs.Profile.Image != p.Image {
		t.Fatalf("loaded profile = %+v, want %+v", obs.Profile, p)
	}
}

func TestFormatProfileRejectsAProfileThatDoesNotLoad(t *testing.T) {
	if data, err := FormatProfile(Profile{Providers: map[Provider]string{Xerces: "Xerces-J 2.12.2"}, Date: "2026-11-19", Image: "sha256:0"}); err == nil || !strings.Contains(err.Error(), "has no version for dotnet") {
		t.Fatalf("FormatProfile(only xerces) = %q, %v; want the missing dotnet version", data, err)
	}
}

func TestLoadObservationsRejectsAnAbsentShardFile(t *testing.T) {
	fsys := observationFS("")
	delete(fsys, "oracles/top-level.tsv")
	inv, err := LoadInventory(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadObservations(fsys, &inv); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadObservations() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadObservationsRejectsBadRows(t *testing.T) {
	for _, tt := range []struct {
		name, rows, want string
	}{
		{name: "unsorted", rows: "project/a\tschema\txerces\tschema-valid\t\nproject/a\tschema\tdotnet\tschema-valid\t\n", want: "line 2 is not sorted"},
		{name: "duplicate", rows: strings.Repeat("project/a\tschema\txerces\tschema-valid\t\n", 2), want: "line 2 is not sorted after line 1, or repeats"},
		{name: "other shard", rows: "project/area/a\tschema\txerces\tschema-valid\t\n", want: `case "project/area/a" belongs in "oracles/area.tsv"`},
		{name: "W3C case", rows: "w3c/msMeta/Regex_w3c.xml/g\tschema\txerces\tschema-valid\t\n", want: `case "w3c/msMeta/Regex_w3c.xml/g" belongs in "oracles/w3c.tsv"`},
		{name: "Xerces-J case", rows: "xerces-j/a\tschema\txerces\tschema-valid\t\n", want: `case "xerces-j/a" has no observation file: case "xerces-j/a" is neither a project nor a W3C case`},
		{name: "unknown verdict", rows: "project/a\tschema\txerces\taccepted\t\n", want: `unknown verdict "accepted"`},
		{name: "unknown provider", rows: "project/a\tschema\tsaxon\tvalid\t\n", want: `unknown provider "saxon"`},
		{name: "field count", rows: "project/a\tschema\txerces\tvalid\n", want: "row has 4 fields, want 5"},
		{name: "encoded file URI", rows: "project/a\tschema\txmlschema\tschema-invalid\tSchema URL: file:///home/u/xsd/tests/corpus/project/a%23b/schema.xsd\n", want: "an absolute path in its message"},
		{name: "absolute corpus path", rows: "project/a\tschema\txerces\tschema-invalid\tC:\\src\\xsd/tests/corpus/project/a/schema.xsd: bad\n", want: "an absolute path in its message"},
		{name: "no final line break", rows: "project/a\tschema\txerces\tvalid\t", want: "does not end with a line break"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fsys := observationFS(tt.rows)
			inv, err := LoadInventory(fsys)
			if err != nil {
				t.Fatal(err)
			}
			_, err = LoadObservations(fsys, &inv)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadObservations() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadObservationsRejectsInvalidProfile(t *testing.T) {
	const versions = `"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"`
	for _, tt := range []struct {
		name, profile, want string
	}{
		{name: "missing version", profile: `{"providers":{"xerces":"Xerces-J 2.12.2"},"date":"2026-11-19","image":"sha256:0"}`, want: "has no version for dotnet"},
		{name: "no date", profile: `{"providers":{` + versions + `},"image":"sha256:0"}`, want: "needs a date and an image"},
		{name: "no image", profile: `{"providers":{` + versions + `},"date":"2026-11-19","image":""}`, want: "needs a date and an image"},
		{name: "unknown provider", profile: `{"providers":{` + versions + `,"saxon":"Saxon 12"},"date":"2026-11-19","image":"sha256:0"}`, want: "lists an unknown provider"},
		{name: "misspelt key", profile: `{"providers":{` + versions + `},"date":"2026-11-19","imag":"sha256:0"}`, want: `unknown field "imag"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fsys := observationFS("")
			fsys["oracles/profile.json"] = &fstest.MapFile{Data: []byte(tt.profile)}
			inv, err := LoadInventory(fsys)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadObservations(fsys, &inv); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadObservations() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestVerdictOutcome(t *testing.T) {
	for _, tt := range []struct {
		verdict Verdict
		want    Outcome
	}{
		{SchemaValid, OutcomeValid},
		{Valid, OutcomeValid},
		{SchemaInvalid, OutcomeInvalid},
		{Invalid, OutcomeInvalid},
		{Unassessed, OutcomeUnassessed},
		{Blocked, OutcomeBlocked},
		{Error, OutcomeBlocked},
	} {
		if got, ok := tt.verdict.Outcome(); !ok || got != tt.want {
			t.Errorf("%s.Outcome() = %d, %v; want %d", tt.verdict, got, ok, tt.want)
		}
	}
	if _, ok := Verdict("warning").Outcome(); ok {
		t.Error(`Verdict("warning").Outcome() ok = true, want false`)
	}
}
