package xsd

import "testing"

func TestSplitASCIIQNameBytes(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		prefix    string
		local     string
		ascii     bool
		ok        bool
		hasPrefix bool
	}{
		{name: "local", in: "row", local: "row", ascii: true, ok: true},
		{name: "prefixed", in: "xs:int", prefix: "xs", local: "int", ascii: true, ok: true, hasPrefix: true},
		{name: "leading_colon", in: ":bad", ascii: true},
		{name: "trailing_colon", in: "bad:", ascii: true},
		{name: "duplicate_colon", in: "a:b:c", ascii: true},
		{name: "invalid_char", in: "bad@name", ascii: true},
		{name: "unicode", in: "\u00e9", ascii: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, local, ascii, ok := splitASCIIQNameBytes([]byte(tt.in))
			if ascii != tt.ascii || ok != tt.ok {
				t.Fatalf("splitASCIIQNameBytes(%q) ascii=%v ok=%v, want ascii=%v ok=%v", tt.in, ascii, ok, tt.ascii, tt.ok)
			}
			if !ok {
				return
			}
			if got := prefix != nil; got != tt.hasPrefix {
				t.Fatalf("prefix presence = %v, want %v", got, tt.hasPrefix)
			}
			if string(prefix) != tt.prefix || string(local) != tt.local {
				t.Fatalf("splitASCIIQNameBytes(%q) prefix=%q local=%q, want prefix=%q local=%q", tt.in, prefix, local, tt.prefix, tt.local)
			}
		})
	}
}

func TestNameTableLimitStopsGrowthAfterFirstFailure(t *testing.T) {
	names, err := newNameTable(0)
	if err != nil {
		t.Fatalf("newNameTable() error = %v", err)
	}
	base := len(names.namespaces) + len(names.locals)
	names.maxNames = base + 1

	if _, err := names.InternQName("urn:new", "new"); err == nil {
		t.Fatal("InternQName() succeeded")
	}
	if got := len(names.namespaces) + len(names.locals); got != base {
		t.Fatalf("name count after first failure = %d, want %d", got, base)
	}

	if _, err := names.InternQName("urn:other", "other"); err == nil {
		t.Fatal("second InternQName() succeeded")
	}
	if got := len(names.namespaces) + len(names.locals); got != base {
		t.Fatalf("name count after second failure = %d, want %d", got, base)
	}
}
