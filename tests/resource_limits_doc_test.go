package tests_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/validate"
)

// xmlSpace is the XML lexical whitespace set.
const xmlSpace = " \t\r\n"

// parseDocDefault parses a RESOURCE_LIMITS.md default cell such as `4 MiB`,
// `64 KiB`, `16_384` or `0`.
func parseDocDefault(cell string) (uint64, error) {
	text := strings.Trim(cell, xmlSpace)
	if len(text) < 3 || text[0] != '`' || text[len(text)-1] != '`' {
		return 0, fmt.Errorf("default %q is not a code span", cell)
	}
	text = text[1 : len(text)-1]
	multiplier := uint64(1)
	for suffix, m := range map[string]uint64{" KiB": 1 << 10, " MiB": 1 << 20} {
		if trimmed, ok := strings.CutSuffix(text, suffix); ok {
			text, multiplier = trimmed, m
		}
	}
	if text == "" || strings.HasPrefix(text, "_") || strings.HasSuffix(text, "_") || strings.Contains(text, "__") {
		return 0, fmt.Errorf("default %q is malformed", cell)
	}
	n, err := strconv.ParseUint(strings.ReplaceAll(text, "_", ""), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("default %q: %w", cell, err)
	}
	return n * multiplier, nil
}

func TestParseDocDefault(t *testing.T) {
	for _, test := range []struct {
		cell    string
		want    uint64
		wantErr bool
	}{
		{cell: "`4 MiB`", want: 4 << 20},
		{cell: " `64 KiB` ", want: 64 << 10},
		{cell: "`16_384`", want: 16384},
		{cell: "`0`", want: 0},
		{cell: "4 MiB", wantErr: true},
		{cell: "`4 GiB`", wantErr: true},
		{cell: "`1__0`", wantErr: true},
		{cell: "``", wantErr: true},
	} {
		got, err := parseDocDefault(test.cell)
		if (err != nil) != test.wantErr || got != test.want {
			t.Errorf("parseDocDefault(%q) = %d, %v; want %d, error=%v", test.cell, got, err, test.want, test.wantErr)
		}
	}
}

// docOptionTable returns option name to default cell for the table under the
// given level-two heading.
func docOptionTable(t *testing.T, doc, heading string) map[string]string {
	t.Helper()
	_, rest, ok := strings.Cut(doc, "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("heading %q not found", heading)
	}
	if section, _, more := strings.Cut(rest, "\n## "); more {
		rest = section
	}
	rows := map[string]string{}
	for line := range strings.SplitSeq(rest, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.Trim(cells[1], xmlSpace), "`Max") {
			continue
		}
		name := strings.Trim(strings.Trim(cells[1], xmlSpace), "`")
		if _, dup := rows[name]; dup {
			t.Fatalf("%s: option %s documented twice", heading, name)
		}
		rows[name] = cells[2]
	}
	return rows
}

func TestResourceLimitsDocMatchesCodeDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "RESOURCE_LIMITS.md"))
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	doc := string(data)

	validateLimits, err := validate.NormalizeOptions(validate.Options(xsd.ValidateOptions{}))
	if err != nil {
		t.Fatalf("validate.NormalizeOptions: %v", err)
	}
	compileLimits, err := schema.NormalizeOptions(schema.Options(xsd.CompileOptions{}))
	if err != nil {
		t.Fatalf("schema.NormalizeOptions: %v", err)
	}

	tests := []struct {
		heading string
		options reflect.Type
		limits  reflect.Value
		// limitField maps a public option name to its normalized Limits field.
		limitField func(string) string
	}{
		{"Compilation", reflect.TypeFor[xsd.CompileOptions](), reflect.ValueOf(compileLimits), func(name string) string { return name }},
		{"Validation", reflect.TypeFor[xsd.ValidateOptions](), reflect.ValueOf(validateLimits), func(name string) string { return strings.TrimPrefix(name, "Max") }},
	}
	for _, test := range tests {
		t.Run(test.heading, func(t *testing.T) {
			rows := docOptionTable(t, doc, test.heading)
			for field := range test.options.Fields() {
				name := field.Name
				if _, ok := rows[name]; !ok {
					t.Errorf("public option %s is not documented", name)
				}
			}
			for name, cell := range rows {
				if _, ok := test.options.FieldByName(name); !ok {
					t.Errorf("documented option %s is not a public option", name)
					continue
				}
				documented, err := parseDocDefault(cell)
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				field := test.limits.FieldByName(test.limitField(name))
				if !field.IsValid() {
					t.Errorf("%s: no normalized limit field %s", name, test.limitField(name))
					continue
				}
				var actual uint64
				if field.CanInt() {
					actual = uint64(max(field.Int(), 0))
				} else {
					actual = field.Uint()
				}
				if actual != documented {
					t.Errorf("%s: doc default %d, code default %d", name, documented, actual)
				}
			}
			if test.heading == "Compilation" {
				for _, name := range []string{"MaxSchemaNames", "MaxFiniteOccurs"} {
					if got, err := parseDocDefault(rows[name]); err != nil || got != 0 {
						t.Errorf("%s: doc default must be `0` (special meaning), got %q", name, rows[name])
					}
				}
			}
		})
	}
}
