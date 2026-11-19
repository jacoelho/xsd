package corpus

import (
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
		"differences":{"xerces":{"record":"I-001"},"libxml2":{"note":"observed before records"}}},
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
	if len(phases) != 2 || phases[0].Name != SchemaPhase || phases[1].Name != "i" || phases[1].Expected != "invalid" {
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
		{name: "unknown schema expected", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"unknown"}}`, want: `phase "schema": unknown expected value "unknown"`},
		{name: "unknown instance expected", project: `{"id":"project/a","expectedSource":"project","instances":[{"testName":"i","expected":""}]}`, want: `phase "i": unknown expected value ""`},
		{name: "duplicate instance", project: `{"id":"project/a","expectedSource":"project","instances":[{"testName":"i","expected":"valid"},{"testName":"i","expected":"invalid"}]}`, want: `duplicates instance "i"`},
		{name: "instance named schema", project: `{"id":"project/a","expectedSource":"project","instances":[{"testName":"schema","expected":"valid"}]}`, want: `instance test name "schema"`},
		{name: "unnamed instance", project: `{"id":"project/a","expectedSource":"project","instances":[{"expected":"valid"}]}`, want: `instance test name ""`},
		{name: "duplicate schema document", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"s.xsd"},{"file":"s.xsd"}]}}`, want: `duplicates schema document "s.xsd"`},
		{name: "provider oracle entry", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","oracle":{"xerces":{"expected":"valid","reason":"r"}}}}`, want: `unknown oracle key "xerces"`},
		{name: "oracle without reason", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","oracle":{"w3c":{"expected":"invalid","reason":" "}}}}`, want: `oracle "w3c" has no reason`},
		{name: "oracle status", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","oracle":{"project":{"status":"blocked","reason":"r"}}}}`, want: `oracle "project" has unknown expected value ""`},
		{name: "rule without specRef", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","rule":{"clause":"c"}}}`, want: "rule has no specRef"},
		{name: "difference for unknown provider", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","differences":{"dotNet":{"record":"I-001"}}}}`, want: `difference for unknown provider "dotNet"`},
		{name: "difference with record and note", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","differences":{"dotnet":{"record":"I-001","note":"n"}}}}`, want: "has both record and note"},
		{name: "malformed record", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","differences":{"dotnet":{"record":"I-1"}}}}`, want: `names record "I-1", want I-NNN or a row anchor`},
		{name: "malformed row anchor", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","differences":{"dotnet":{"record":"Temporal boundaries"}}}}`, want: `names record "Temporal boundaries"`},
		{name: "empty difference", project: `{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","differences":{"dotnet":{}}}}`, want: "has neither record nor note"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadInventory(inventoryFS(tt.project, tt.w3c))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadInventory() error = %v, want containing %q", err, tt.want)
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
	if got := string(FormatRows(shuffled)); got != data {
		t.Fatalf("FormatRows() = %q, want the sorted file %q", got, data)
	}
}

func TestLoadObservationsRejectsBadRows(t *testing.T) {
	for _, tt := range []struct {
		name, rows, want string
	}{
		{name: "unsorted", rows: "project/a\tschema\txerces\tschema-valid\t\nproject/a\tschema\tdotnet\tschema-valid\t\n", want: "line 2 is not sorted"},
		{name: "duplicate", rows: strings.Repeat("project/a\tschema\txerces\tschema-valid\t\n", 2), want: "line 2 is not sorted after line 1, or repeats"},
		{name: "other shard", rows: "project/area/a\tschema\txerces\tschema-valid\t\n", want: `case "project/area/a" belongs in "oracles/area.tsv"`},
		{name: "non-project case", rows: "xerces-j/a\tschema\txerces\tschema-valid\t\n", want: `case "xerces-j/a" has no observation file: case "xerces-j/a" is not a project case`},
		{name: "unknown verdict", rows: "project/a\tschema\txerces\taccepted\t\n", want: `unknown verdict "accepted"`},
		{name: "unknown provider", rows: "project/a\tschema\tsaxon\tvalid\t\n", want: `unknown provider "saxon"`},
		{name: "field count", rows: "project/a\tschema\txerces\tvalid\n", want: "row has 4 fields, want 5"},
		{name: "encoded file URI", rows: "project/a\tschema\txmlschema\tschema-invalid\tSchema URL: file:///home/u/xsd/tests/corpus/project/a%23b/schema.xsd\n", want: "has an absolute path in its message"},
		{name: "absolute corpus path", rows: "project/a\tschema\txerces\tschema-invalid\tC:\\src\\xsd/tests/corpus/project/a/schema.xsd: bad\n", want: "has an absolute path in its message"},
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

func TestLoadObservationsRejectsIncompleteProfile(t *testing.T) {
	fsys := observationFS("")
	fsys["oracles/profile.json"] = &fstest.MapFile{Data: []byte(`{"providers":{"xerces":"Xerces-J 2.12.2"},"date":"2026-11-19","image":"sha256:0"}`)}
	inv, err := LoadInventory(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadObservations(fsys, &inv); err == nil || !strings.Contains(err.Error(), "has no version for dotnet") {
		t.Fatalf("LoadObservations() error = %v, want the missing dotnet version", err)
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
