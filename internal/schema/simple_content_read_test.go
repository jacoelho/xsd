package schema

import "testing"

func TestSimpleContentTypeReadForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ct      ComplexType
		want    SimpleTypeID
		present bool
	}{
		{
			name: "simple",
			ct: ComplexType{
				TextType:    3,
				ContentKind: ContentSimple,
			},
			want:    3,
			present: true,
		},
		{
			name: "element only",
			ct: ComplexType{
				TextType:    5,
				ContentKind: ContentElementOnly,
			},
			want: NoSimpleType,
		},
		{
			name: "mixed",
			ct: ComplexType{
				TextType:    6,
				ContentKind: ContentMixed,
			},
			want: NoSimpleType,
		},
		{
			name: "empty",
			ct:   ComplexType{TextType: 7, ContentKind: ContentEmpty},
			want: NoSimpleType,
		},
		{
			name:    "present invalid",
			ct:      ComplexType{TextType: NoSimpleType, ContentKind: ContentSimple},
			want:    NoSimpleType,
			present: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := newComplexTypeRead(tt.ct)
			typ, present := read.simpleContent()
			if typ != tt.want || present != tt.present {
				t.Fatalf("simpleContent() = type %d present %v, want %d %v",
					typ, present, tt.want, tt.present)
			}
		})
	}
}
