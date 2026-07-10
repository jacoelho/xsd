package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestSessionAppendTextLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		append  string
		max     int64
		want    string
		wantErr bool
	}{
		{
			name:    "unlimited",
			current: "0123456789",
			append:  "abcdefghij",
			want:    "0123456789abcdefghij",
		},
		{
			name:    "exact limit",
			current: "12",
			append:  "345",
			max:     5,
			want:    "12345",
		},
		{
			name:    "exceeds limit",
			current: "123",
			append:  "456",
			max:     5,
			want:    "123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := session{maxInstanceTextBytes: tt.max}
			s.doc.text = []byte(tt.current)
			err := s.appendText([]byte(tt.append), 8, 9)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("appendText() error = %v", err)
				}
			} else {
				requireCode(t, err, xsderrors.CodeValidationLimit)
				if !strings.Contains(err.Error(), "instance text byte limit exceeded") {
					t.Fatalf("appendText() error = %v", err)
				}
				expectXSDLocation(t, err, "/", 8, 9)
			}
			if got := string(s.doc.text); got != tt.want {
				t.Fatalf("text = %q, want %q", got, tt.want)
			}
		})
	}
}
