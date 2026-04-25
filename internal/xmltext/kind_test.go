package xmltext

import "testing"

func TestKindString(t *testing.T) {
	kinds := []Kind{
		KindNone,
		KindStartElement,
		KindEndElement,
		KindCharData,
		KindComment,
		KindPI,
		KindDirective,
		KindCDATA,
		Kind(99),
	}
	wants := []string{
		"None",
		"StartElement",
		"EndElement",
		"CharData",
		"Comment",
		"PI",
		"Directive",
		"CDATA",
		"Unknown",
	}

	for i, kind := range kinds {
		if got := kind.String(); got != wants[i] {
			t.Fatalf("Kind(%d) = %q, want %s", kind, got, wants[i])
		}
	}
}
