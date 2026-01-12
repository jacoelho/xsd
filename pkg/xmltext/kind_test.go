package xmltext

import "testing"

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindNone, "None"},
		{KindStartElement, "StartElement"},
		{KindEndElement, "EndElement"},
		{KindCharData, "CharData"},
		{KindComment, "Comment"},
		{KindPI, "PI"},
		{KindDirective, "Directive"},
		{KindCDATA, "CDATA"},
		{Kind(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Fatalf("Kind(%d) = %q, want %s", tt.kind, got, tt.want)
		}
	}
}
