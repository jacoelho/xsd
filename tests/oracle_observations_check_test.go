package tests_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/corpus"
)

func row(caseID, phase string, p corpus.Provider, v corpus.Verdict) corpus.Row {
	return corpus.Row{CaseID: caseID, Phase: phase, Provider: p, Verdict: v}
}

// coverageCase has a schema that Xerces-J and xmlschema accept, so its
// instance phase needs rows from exactly those two providers.
func coverageCase(id string) (corpus.Case, []corpus.Row) {
	c := corpus.Case{
		ID: id, ExpectedSource: "project",
		Schema:    &corpus.Schema{Expected: corpus.ExpectedValid},
		Instances: []corpus.Instance{{TestName: "i1", Expected: corpus.ExpectedValid}},
	}
	rows := []corpus.Row{
		row(id, "schema", corpus.Xerces, corpus.SchemaValid),
		row(id, "schema", corpus.DotNet, corpus.SchemaInvalid),
		row(id, "schema", corpus.XMLSchema, corpus.SchemaValid),
		row(id, "schema", corpus.LibXML2, corpus.Error),
		row(id, "i1", corpus.Xerces, corpus.Valid),
		row(id, "i1", corpus.XMLSchema, corpus.Unassessed),
	}
	return c, rows
}

func TestCheckObservationCoverage(t *testing.T) {
	measured, base := coverageCase("project/measured")
	w3c := corpus.Case{ID: "w3c/a/b/c", ExpectedSource: "w3c", Schema: &corpus.Schema{Expected: corpus.ExpectedValid}}
	inv := corpus.Inventory{Cases: []corpus.Case{measured, w3c}}
	if err := checkObservationCoverage(&inv, base); err != nil {
		t.Fatalf("checkObservationCoverage() error = %v, want nil", err)
	}
	override := corpus.Case{ID: "w3c/a/b/override", ExpectedSource: "w3c", Schema: &corpus.Schema{
		Expected: corpus.ExpectedInvalid,
		Oracle:   map[string]corpus.Provenance{"w3c": {Expected: corpus.ExpectedValid, Reason: "r"}},
	}}
	overridden := corpus.Inventory{Cases: []corpus.Case{measured, w3c, override}}
	if err := checkObservationCoverage(&overridden, base); err == nil || !strings.Contains(err.Error(), `measured case "w3c/a/b/override" has no row for phase "schema" provider xerces`) {
		t.Fatalf("checkObservationCoverage() with a W3C override and no rows error = %v, want its schema rows required", err)
	}
	with := func(r corpus.Row) []corpus.Row { return append(append([]corpus.Row(nil), base...), r) }
	for _, tt := range []struct {
		name string
		rows []corpus.Row
		want string
	}{
		{name: "missing schema row", rows: base[1:], want: `measured case "project/measured" has no row for phase "schema" provider xerces`},
		{name: "missing instance row", rows: base[:5], want: `measured case "project/measured" has no row for phase "i1" provider xmlschema`},
		{name: "instance row of a provider that rejected the schema", rows: with(row("project/measured", "i1", corpus.DotNet, corpus.Valid)), want: "row project/measured\ti1\tdotnet names no phase of a measured case measured with that provider"},
		{name: "instance row of a provider that errored on the schema", rows: with(row("project/measured", "i1", corpus.LibXML2, corpus.Valid)), want: "row project/measured\ti1\tlibxml2 names no phase of a measured case"},
		{name: "unknown phase", rows: with(row("project/measured", "i2", corpus.Xerces, corpus.Valid)), want: "row project/measured\ti2\txerces names no phase of a measured case"},
		{name: "unknown case", rows: with(row("project/other", "schema", corpus.Xerces, corpus.SchemaValid)), want: "row project/other\tschema\txerces names no phase of a measured case"},
		{name: "w3c case", rows: with(row("w3c/a/b/c", "schema", corpus.Xerces, corpus.SchemaValid)), want: "row w3c/a/b/c\tschema\txerces names no phase of a measured case"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := checkObservationCoverage(&inv, tt.rows)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkObservationCoverage() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckDifferences(t *testing.T) {
	records := parseRecords("## I-001 — first\n\nXerces-J rejects; .NET too.\n\nCases: `project/diff*`.\n## I-002 — second\n\nCases: `project/other`.\n\n## Qualifications\n\n" +
		"| <a id=\"row-q\"></a>Row | libxml2 accepts `project/diff`. |\n| <a id=\"row-r\"></a>Other | libxml2 accepts `project/other`. |\n")
	schemaDiff := func(d map[corpus.Provider]corpus.Difference) corpus.Case {
		return corpus.Case{ID: "project/diff", ExpectedSource: "project", Schema: &corpus.Schema{Expected: corpus.ExpectedValid, Differences: d}}
	}
	differing := []corpus.Row{row("project/diff", "schema", corpus.Xerces, corpus.SchemaInvalid)}
	for _, tt := range []struct {
		name    string
		c       corpus.Case
		rows    []corpus.Row
		backlog []string
		want    string
	}{
		{name: "anchored row", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.LibXML2: {Record: "row-q"}}), rows: []corpus.Row{row("project/diff", "schema", corpus.LibXML2, corpus.SchemaInvalid)}},
		{name: "anchored row without the case", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.LibXML2: {Record: "row-r"}}), rows: []corpus.Row{row("project/diff", "schema", corpus.LibXML2, corpus.SchemaInvalid)}, want: "cites row-r, which does not list the case"},
		{name: "anchored row without the provider", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "row-q"}}), rows: differing, want: "cites row-q, which does not name Xerces"},
		{name: "backlog case with an unowned difference", c: schemaDiff(nil), rows: differing, backlog: []string{"project/diff"}},
		{name: "backlog case whose differences are owned", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}), rows: differing, backlog: []string{"project/diff"}, want: `oracle backlog lists "project/diff", whose differences are all owned; remove it`},
		{name: "backlog case without differences", c: schemaDiff(nil), rows: []corpus.Row{row("project/diff", "schema", corpus.Xerces, corpus.SchemaValid)}, backlog: []string{"project/diff"}, want: "whose differences are all owned"},
		{name: "backlog lists an unknown case", c: schemaDiff(nil), rows: differing, backlog: []string{"project/diff", "project/zz"}, want: `oracle backlog lists "project/zz", which is not a measured case`},
		{
			name: "W3C override case with an undiagnosed row",
			c: corpus.Case{ID: "w3c/a/b/override", ExpectedSource: "w3c", Schema: &corpus.Schema{
				Expected: corpus.ExpectedInvalid,
				Oracle:   map[string]corpus.Provenance{"w3c": {Expected: corpus.ExpectedValid, Reason: "r"}},
			}},
			rows: []corpus.Row{row("w3c/a/b/override", "schema", corpus.DotNet, corpus.SchemaValid)},
			want: "1 rows differ from the project expectation without a differences entry:\nw3c/a/b/override\tschema\tdotnet\tschema-valid (expected invalid)",
		},
		{name: "record", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}), rows: differing},
		{name: "agreeing, unassessed and error rows need none", c: schemaDiff(nil), rows: []corpus.Row{
			row("project/diff", "schema", corpus.Xerces, corpus.SchemaValid), row("project/diff", "schema", corpus.DotNet, corpus.Error),
		}},
		{name: "undiagnosed", c: schemaDiff(nil), rows: differing, want: "1 rows differ from the project expectation without a differences entry:\nproject/diff\tschema\txerces\tschema-invalid (expected valid)"},
		{name: "undefined record", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-009"}}), rows: differing, want: "cites I-009, which docs/spec/INTERPRETATIONS.md does not define"},
		{name: "record without the case", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-002"}}), rows: differing, want: "cites I-002, which does not list the case"},
		{name: "record without the provider", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.LibXML2: {Record: "I-001"}}), rows: []corpus.Row{row("project/diff", "schema", corpus.LibXML2, corpus.SchemaInvalid)}, want: "cites I-001, which does not name libxml2"},
		{name: "difference for a blocked row", c: schemaDiff(map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}), rows: []corpus.Row{row("project/diff", "schema", corpus.Xerces, corpus.Blocked)}, want: "has a difference but no row whose validity differs"},
		{
			name: "instances of an invalid schema have no expectation to differ from",
			c: corpus.Case{ID: "project/diff", ExpectedSource: "project", Schema: &corpus.Schema{Expected: corpus.ExpectedInvalid, Differences: map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}},
				Instances: []corpus.Instance{{TestName: "i", Expected: corpus.ExpectedInvalid}}},
			rows: []corpus.Row{row("project/diff", "schema", corpus.Xerces, corpus.SchemaValid), row("project/diff", "i", corpus.Xerces, corpus.Valid)},
		},
		{
			name: "difference on an instance of an invalid schema",
			c: corpus.Case{ID: "project/diff", ExpectedSource: "project", Schema: &corpus.Schema{Expected: corpus.ExpectedInvalid, Differences: map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}},
				Instances: []corpus.Instance{{TestName: "i", Expected: corpus.ExpectedInvalid, Differences: map[corpus.Provider]corpus.Difference{corpus.Xerces: {Record: "I-001"}}}}},
			rows: []corpus.Row{row("project/diff", "schema", corpus.Xerces, corpus.SchemaValid), row("project/diff", "i", corpus.Xerces, corpus.Valid)},
			want: `case "project/diff" instance "i" has differences, but its schema is expected invalid`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inv := corpus.Inventory{Cases: []corpus.Case{tt.c}}
			err := checkDifferences(&inv, tt.rows, oracleEvidence{records: records, backlog: tt.backlog})
			if tt.want == "" {
				if err != nil {
					t.Fatalf("checkDifferences() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkDifferences() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRecordListsCase(t *testing.T) {
	text := "Cases: `project/a/exact`, `project/b/*-tail` and `project/c*mid*end`."
	for _, tt := range []struct {
		id   string
		want bool
	}{
		{"project/a/exact", true},
		{"project/a/exact-more", false},
		{"project/b/x/y-tail", true},
		{"project/b/x-tail-not", false},
		{"project/c/1/mid/2/end", true},
		{"project/c/end/mid", false},
		{"Cases: ", false},
	} {
		if got := recordListsCase(text, tt.id); got != tt.want {
			t.Errorf("recordListsCase(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestParseOracleBacklogRejectsBadLines(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want string
	}{
		{name: "unsorted", data: "project/b\nproject/a\n", want: `line 2 is not sorted after "project/b"`},
		{name: "duplicate", data: strings.Repeat("project/a\n", 2), want: `line 2 is not sorted after "project/a"`},
		{name: "empty line", data: "project/a\n\nproject/b\n", want: "line 2 is empty"},
		{name: "surrounding space", data: "project/a \n", want: "line 1 has surrounding space"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOracleBacklog(tt.data)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseOracleBacklog() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
