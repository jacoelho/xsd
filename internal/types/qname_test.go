package types

import "testing"

func TestQName_String(t *testing.T) {
	tests := []struct {
		name    string
		qname   QName
		wantStr string
	}{
		{
			name:    "with namespace",
			qname:   QName{Namespace: "http://example.com", Local: "element"},
			wantStr: "{http://example.com}element",
		},
		{
			name:    "without namespace",
			qname:   QName{Local: "element"},
			wantStr: "element",
		},
		{
			name:    "zero value",
			qname:   QName{},
			wantStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.qname.String(); got != tt.wantStr {
				t.Errorf("QName.String() = %v, want %v", got, tt.wantStr)
			}
		})
	}
}

func TestQName_IsZero(t *testing.T) {
	tests := []struct {
		name  string
		qname QName
		want  bool
	}{
		{
			name:  "zero value",
			qname: QName{},
			want:  true,
		},
		{
			name:  "only namespace",
			qname: QName{Namespace: "http://example.com"},
			want:  false,
		},
		{
			name:  "only local",
			qname: QName{Local: "element"},
			want:  false,
		},
		{
			name:  "both set",
			qname: QName{Namespace: "http://example.com", Local: "element"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.qname.IsZero(); got != tt.want {
				t.Errorf("QName.IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQName_Equal(t *testing.T) {
	tests := []struct {
		name string
		q1   QName
		q2   QName
		want bool
	}{
		{
			name: "equal with namespace",
			q1:   QName{Namespace: "http://example.com", Local: "element"},
			q2:   QName{Namespace: "http://example.com", Local: "element"},
			want: true,
		},
		{
			name: "equal without namespace",
			q1:   QName{Local: "element"},
			q2:   QName{Local: "element"},
			want: true,
		},
		{
			name: "different namespace",
			q1:   QName{Namespace: "http://example.com", Local: "element"},
			q2:   QName{Namespace: "http://other.com", Local: "element"},
			want: false,
		},
		{
			name: "different local",
			q1:   QName{Namespace: "http://example.com", Local: "element"},
			q2:   QName{Namespace: "http://example.com", Local: "other"},
			want: false,
		},
		{
			name: "zero values",
			q1:   QName{},
			q2:   QName{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.q1.Equal(tt.q2); got != tt.want {
				t.Errorf("QName.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
