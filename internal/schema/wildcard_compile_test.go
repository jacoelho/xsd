package schema

import (
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attrs      WildcardAttrs
		wantMode   WildcardMode
		wantNS     []string
		wantOther  string
		wantProc   ProcessContents
		wantCode   xsderrors.Code
		wantNoName bool
	}{
		{
			name:       "defaults to any strict",
			wantMode:   WildcardAny,
			wantProc:   ProcessStrict,
			wantNoName: true,
		},
		{
			name: "other uses target namespace",
			attrs: WildcardAttrs{
				Namespace:       "##other",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode:  WildcardOther,
			wantOther: "urn:t",
			wantProc:  ProcessStrict,
		},
		{
			name: "local",
			attrs: WildcardAttrs{
				Namespace:    "##local",
				HasNamespace: true,
			},
			wantMode:   WildcardLocal,
			wantProc:   ProcessStrict,
			wantNoName: true,
		},
		{
			name: "target namespace",
			attrs: WildcardAttrs{
				Namespace:       "##targetNamespace",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode: WildcardTargetNamespace,
			wantNS:   []string{"urn:t"},
			wantProc: ProcessStrict,
		},
		{
			name: "list normalizes XML whitespace and duplicates",
			attrs: WildcardAttrs{
				Namespace:          "urn:b\t##local\nurn:a\rurn:b ##targetNamespace",
				ProcessContents:    "lax",
				TargetNamespace:    "urn:t",
				HasNamespace:       true,
				HasProcessContents: true,
			},
			wantMode: WildcardList,
			wantNS:   []string{"", "urn:b", "urn:a", "urn:t"},
			wantProc: ProcessLax,
		},
		{
			name: "invalid reserved namespace token",
			attrs: WildcardAttrs{
				Namespace:    "urn:a ##bogus",
				HasNamespace: true,
			},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid anyURI namespace",
			attrs: WildcardAttrs{
				Namespace:    "urn:a %zz",
				HasNamespace: true,
			},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid process contents",
			attrs: WildcardAttrs{
				ProcessContents:    "open",
				HasProcessContents: true,
			},
			wantCode:   xsderrors.CodeSchemaInvalidAttribute,
			wantNoName: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names, nameErr := NewNameTable(64, []string{EmptyNamespaceURI}, nil)
			if nameErr != nil {
				t.Fatal(nameErr)
			}
			interner := NewNameInterner(&names)
			if tt.wantNoName {
				interner = NameInterner{}
			}
			got, err := ParseWildcard(interner, tt.attrs)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("ParseWildcard() error = %v", err)
			}
			if got.Mode != tt.wantMode || got.Process != tt.wantProc {
				t.Fatalf("ParseWildcard() = %#v, want mode %d process %d", got, tt.wantMode, tt.wantProc)
			}
			if tt.wantOther != "" && got.OtherThan != wildcardNamespaceIDForTest(t, &names, tt.wantOther) {
				t.Fatalf("OtherThan = %d, want namespace %q", got.OtherThan, tt.wantOther)
			}
			if tt.wantNS != nil {
				var want []NamespaceID
				for _, uri := range tt.wantNS {
					want = append(want, wildcardNamespaceIDForTest(t, &names, uri))
				}
				if !slices.Equal(got.Namespaces, want) {
					t.Fatalf("Namespaces = %v, want %v", got.Namespaces, want)
				}
			}
		})
	}
}

func TestParseWildcardRequiresNamespaceInternerWhenInterning(t *testing.T) {
	t.Parallel()

	_, err := ParseWildcard(NameInterner{}, WildcardAttrs{Namespace: "##other", HasNamespace: true})
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func wildcardNamespaceIDForTest(t *testing.T, names *NameTable, uri string) NamespaceID {
	t.Helper()
	id, ok := names.LookupNamespace(uri)
	if !ok {
		t.Fatalf("namespace was not interned: %q", uri)
	}
	return id
}
func TestParseWildcardZeroInternerPreservesTokenErrors(t *testing.T) {
	for _, namespace := range []string{"##bogus", "%zz"} {
		_, err := ParseWildcard(NameInterner{}, WildcardAttrs{Namespace: namespace, HasNamespace: true})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	}
}
