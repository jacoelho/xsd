package listtypes

import "testing"

func TestItemTypeName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		typeName   string
		wantItem   string
		shouldFind bool
	}{
		{name: "NMTOKENS", typeName: "NMTOKENS", wantItem: "NMTOKEN", shouldFind: true},
		{name: "IDREFS", typeName: "IDREFS", wantItem: "IDREF", shouldFind: true},
		{name: "ENTITIES", typeName: "ENTITIES", wantItem: "ENTITY", shouldFind: true},
		{name: "non-list", typeName: "string", shouldFind: false},
	}

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotItem, ok := ItemTypeName(tc.typeName)
			if ok != tc.shouldFind {
				t.Fatalf("ItemTypeName(%q) found=%v, want %v", tc.typeName, ok, tc.shouldFind)
			}
			if gotItem != tc.wantItem {
				t.Fatalf("ItemTypeName(%q) item=%q, want %q", tc.typeName, gotItem, tc.wantItem)
			}
			if IsTypeName(tc.typeName) != tc.shouldFind {
				t.Fatalf("IsTypeName(%q) mismatch", tc.typeName)
			}
		})
	}
}
