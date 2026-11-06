package schema

import "testing"

func TestSimpleContentTypeReadForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ct      complexTypeMetadata
		want    SimpleTypeID
		present bool
	}{
		{
			name: "simple",
			ct: complexTypeMetadata{
				TextType:    3,
				ContentKind: contentSimple,
			},
			want:    3,
			present: true,
		},
		{
			name: "element only",
			ct: complexTypeMetadata{
				TextType:    5,
				ContentKind: contentElementOnly,
			},
			want: NoSimpleType,
		},
		{
			name: "mixed",
			ct: complexTypeMetadata{
				TextType:    6,
				ContentKind: contentMixed,
			},
			want: NoSimpleType,
		},
		{
			name: "empty",
			ct:   complexTypeMetadata{TextType: 7, ContentKind: contentEmpty},
			want: NoSimpleType,
		},
		{
			name:    "present invalid",
			ct:      complexTypeMetadata{TextType: NoSimpleType, ContentKind: contentSimple},
			want:    NoSimpleType,
			present: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := newComplexTypeRead(tt.ct, tt.ct.Content)
			typ, present := read.simpleContent()
			if typ != tt.want || present != tt.present {
				t.Fatalf("simpleContent() = type %d present %v, want %d %v",
					typ, present, tt.want, tt.present)
			}
		})
	}
}
