package tests_test

import (
	"strings"
	"testing"
)

func TestParseUnsupportedAllowlistAcceptsSchemaAndInstance(t *testing.T) {
	data := strings.Join([]string{
		"instance\tw3c\tcase-a\tcase-a.v\tunsupported.regex",
		"schema\tw3c\tcase-b\tunsupported.xsd_1_1",
		"",
	}, "\n")
	unsupported, err := parseUnsupportedAllowlist(data)
	if err != nil {
		t.Fatalf("parseUnsupportedAllowlist() error = %v", err)
	}
	if len(unsupported) != 2 {
		t.Fatalf("len(unsupported) = %d, want 2", len(unsupported))
	}
	if err := unsupported.use(unsupportedKey{kind: unsupportedInstance, source: "w3c", caseID: "case-a", instance: "case-a.v"}, "unsupported.regex"); err != nil {
		t.Fatalf("use() error = %v", err)
	}
	if err := unsupported.use(unsupportedKey{kind: unsupportedSchema, source: "w3c", caseID: "case-b"}, "unsupported.xsd_1_1"); err != nil {
		t.Fatalf("use() error = %v", err)
	}
}

func TestParseUnsupportedAllowlistRejectsBadLines(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "malformed schema",
			data: "schema\tw3c\tcase\n",
			want: "want 4",
		},
		{
			name: "unsorted",
			data: "schema\tw3c\tcase-b\tunsupported.regex\ninstance\tw3c\tcase-a\tcase-a.v\tunsupported.regex\n",
			want: "not sorted",
		},
		{
			name: "duplicate key",
			data: "schema\tw3c\tcase\tunsupported.regex\nschema\tw3c\tcase\tunsupported.xsd_1_1\n",
			want: "duplicates",
		},
		{
			name: "unknown kind",
			data: "document\tw3c\tcase\tunsupported.regex\n",
			want: "unknown kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnsupportedAllowlist(tt.data)
			if err == nil {
				t.Fatal("parseUnsupportedAllowlist() succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseUnsupportedAllowlist() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestUnsupportedAllowlistUseRequiresKnownCode(t *testing.T) {
	key := unsupportedKey{kind: unsupportedSchema, source: "w3c", caseID: "case"}
	unsupported := unsupportedAllowlist{
		key: {code: "unsupported.regex"},
	}
	if err := unsupported.use(key, "unsupported.xsd_1_1"); err == nil {
		t.Fatal("use() accepted wrong code")
	}
	if err := unsupported.use(unsupportedKey{kind: unsupportedSchema, source: "w3c", caseID: "other"}, "unsupported.regex"); err == nil {
		t.Fatal("use() accepted unlisted key")
	}
	if err := unsupported.use(key, "unsupported.regex"); err != nil {
		t.Fatalf("use() error = %v", err)
	}
	if !unsupported[key].used {
		t.Fatal("use() did not mark entry used")
	}
}

func TestHarnessRunCoverageRequiresFullCorpus(t *testing.T) {
	m := manifest{
		Cases: []manifestCase{
			{Schema: manifestSchema{Expected: "valid"}, Instances: []manifestInstance{{}, {}}},
			{Schema: manifestSchema{Expected: "valid"}, Instances: []manifestInstance{{}}},
			{Schema: manifestSchema{Expected: "invalid"}, Instances: []manifestInstance{{}}},
		},
	}
	if !(harnessRunCoverage{schemaCases: 3, instanceRuns: 3}).complete(m) {
		t.Fatal("complete run reported incomplete")
	}
	if (harnessRunCoverage{schemaCases: 2, instanceRuns: 3}).complete(m) {
		t.Fatal("partial schema run reported complete")
	}
	if (harnessRunCoverage{schemaCases: 3, instanceRuns: 2}).complete(m) {
		t.Fatal("partial instance run reported complete")
	}
}
