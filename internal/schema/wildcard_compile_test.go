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
		attrs      wildcardAttrs
		wantMode   wildcardMode
		wantNS     []string
		wantOther  string
		wantProc   ProcessContents
		wantCode   xsderrors.Code
		wantNoName bool
	}{
		{
			name:       "defaults to any strict",
			wantMode:   WildcardAny,
			wantProc:   processStrict,
			wantNoName: true,
		},
		{
			name: "other uses target namespace",
			attrs: wildcardAttrs{
				Namespace:       "##other",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode:  WildcardOther,
			wantOther: "urn:t",
			wantProc:  processStrict,
		},
		{
			name: "local",
			attrs: wildcardAttrs{
				Namespace:    "##local",
				HasNamespace: true,
			},
			wantMode:   WildcardLocal,
			wantProc:   processStrict,
			wantNoName: true,
		},
		{
			name: "target namespace",
			attrs: wildcardAttrs{
				Namespace:       "##targetNamespace",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode: WildcardTargetNamespace,
			wantNS:   []string{"urn:t"},
			wantProc: processStrict,
		},
		{
			name: "list normalizes XML whitespace and duplicates",
			attrs: wildcardAttrs{
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
			attrs: wildcardAttrs{
				Namespace:    "urn:a ##bogus",
				HasNamespace: true,
			},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid anyURI namespace",
			attrs: wildcardAttrs{
				Namespace:    "urn:a %zz",
				HasNamespace: true,
			},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid process contents",
			attrs: wildcardAttrs{
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

			names, nameErr := newNameTable(64, []string{EmptyNamespaceURI}, nil)
			if nameErr != nil {
				t.Fatal(nameErr)
			}
			interner := newNameInterner(&names)
			if tt.wantNoName {
				interner = nameInterner{}
			}
			got, err := parseWildcard(interner, tt.attrs)
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
				var want []namespaceID
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

	_, err := parseWildcard(nameInterner{}, wildcardAttrs{Namespace: "##other", HasNamespace: true})
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func wildcardNamespaceIDForTest(t *testing.T, names *nameTable, uri string) namespaceID {
	t.Helper()
	id, ok := names.LookupNamespace(uri)
	if !ok {
		t.Fatalf("namespace was not interned: %q", uri)
	}
	return id
}
func TestParseWildcardZeroInternerPreservesTokenErrors(t *testing.T) {
	for _, namespace := range []string{"##bogus", "%zz"} {
		_, err := parseWildcard(nameInterner{}, wildcardAttrs{Namespace: namespace, HasNamespace: true})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	}
}
