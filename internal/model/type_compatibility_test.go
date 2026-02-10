package model

import "testing"

func TestElementTypesCompatible(t *testing.T) {
	t.Parallel()

	stringType := GetBuiltin(TypeNameString)
	intType := GetBuiltin(TypeNameInt)
	if stringType == nil || intType == nil {
		t.Fatalf("missing builtin types")
	}

	anonA := &SimpleType{}
	anonB := &SimpleType{}

	tests := []struct {
		name string
		a    Type
		b    Type
		want bool
	}{
		{name: "nil and nil", a: nil, b: nil, want: true},
		{name: "nil and non-nil", a: nil, b: stringType, want: false},
		{name: "same named type", a: stringType, b: stringType, want: true},
		{name: "different named types", a: stringType, b: intType, want: false},
		{name: "same anonymous pointer", a: anonA, b: anonA, want: true},
		{name: "different anonymous pointers", a: anonA, b: anonB, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ElementTypesCompatible(tt.a, tt.b); got != tt.want {
				t.Fatalf("ElementTypesCompatible() = %v, want %v", got, tt.want)
			}
		})
	}
}
