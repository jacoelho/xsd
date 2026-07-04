package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestKeyCanonicalizesLoadedSourceNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "relative path",
			in:   "schemas/../types.xsd",
			want: filepath.Clean("types.xsd"),
		},
		{
			name: "file uri",
			in:   "file:///tmp/../tmp/types.xsd",
			want: filepath.Clean("/tmp/types.xsd"),
		},
		{
			name: "opaque uri",
			in:   "urn:types",
			want: "urn:types",
		},
		{
			name: "hierarchical uri",
			in:   "https://example.test/schemas/../types.xsd",
			want: "https://example.test/types.xsd",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := Key(tc.in); got != tc.want {
				t.Fatalf("Key(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCheckSourceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		wantIssue SourceNameIssue
	}{
		{name: "present", in: "schema.xsd"},
		{name: "missing", wantIssue: SourceNameMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			issue := CheckSourceName(tt.in)
			if issue != tt.wantIssue {
				t.Fatalf("CheckSourceName(%q) = %v, want %v", tt.in, issue, tt.wantIssue)
			}
			if issue == SourceNameOK {
				if issue.Code() != "" || issue.Message() != "" {
					t.Fatalf("OK issue has code/message: %q %q", issue.Code(), issue.Message())
				}
				return
			}
			if issue.Code() == "" || issue.Message() == "" {
				t.Fatalf("issue %v missing code/message", issue)
			}
		})
	}
}

func TestIsSchemaLimitError(t *testing.T) {
	t.Parallel()

	if !IsSchemaLimitError(fmt.Errorf("wrapped: %w", schemaSourceLimitError("schema.xsd"))) {
		t.Fatal("IsSchemaLimitError(wrapped source limit) = false, want true")
	}
	if IsSchemaLimitError(xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "read failed")) {
		t.Fatal("IsSchemaLimitError(schema read) = true, want false")
	}
	if IsSchemaLimitError(errors.New("plain error")) {
		t.Fatal("IsSchemaLimitError(plain error) = true, want false")
	}
}

func TestBytesNilIsEmptySource(t *testing.T) {
	t.Parallel()

	data, err := Bytes("empty.xsd", nil).Read(1)
	if err != nil {
		t.Fatalf("Bytes(nil).Read() error = %v", err)
	}
	if data == nil || len(data) != 0 {
		t.Fatalf("Bytes(nil).Read() = %#v, want non-nil empty slice", data)
	}
}

func TestCheckImportNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		namespace    string
		hasNamespace bool
		want         string
		wantIssue    ImportNamespaceIssue
	}{
		{
			name:         "explicit foreign namespace",
			target:       "urn:a",
			namespace:    "urn:b",
			hasNamespace: true,
			want:         "urn:b",
		},
		{
			name:      "absent namespace imports no namespace from target schema",
			target:    "urn:a",
			wantIssue: ImportNamespaceOK,
		},
		{
			name:         "empty explicit namespace",
			target:       "urn:a",
			hasNamespace: true,
			wantIssue:    ImportNamespaceEmpty,
		},
		{
			name:      "missing namespace from no-target schema",
			wantIssue: ImportNamespaceMissingTarget,
		},
		{
			name:         "namespace matches enclosing target",
			target:       "urn:a",
			namespace:    "urn:a",
			hasNamespace: true,
			wantIssue:    ImportNamespaceMatchesTarget,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, issue := CheckImportNamespace(tt.target, tt.namespace, tt.hasNamespace)
			if got != tt.want || issue != tt.wantIssue {
				t.Fatalf("CheckImportNamespace() = (%q, %v), want (%q, %v)", got, issue, tt.want, tt.wantIssue)
			}
			if issue == ImportNamespaceOK {
				if issue.Code() != "" || issue.Message() != "" {
					t.Fatalf("OK issue has code/message: %q %q", issue.Code(), issue.Message())
				}
				return
			}
			if issue.Code() == "" || issue.Message() == "" {
				t.Fatalf("issue %v missing code/message", issue)
			}
		})
	}
}

func TestCheckIncludeSchemaLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hasLocation bool
		wantIssue   IncludeLocationIssue
	}{
		{name: "present", hasLocation: true},
		{name: "missing", wantIssue: IncludeLocationMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			issue := CheckIncludeSchemaLocation(tt.hasLocation)
			if issue != tt.wantIssue {
				t.Fatalf("CheckIncludeSchemaLocation() = %v, want %v", issue, tt.wantIssue)
			}
			if issue == IncludeLocationOK {
				if issue.Code() != "" || issue.Message() != "" {
					t.Fatalf("OK issue has code/message: %q %q", issue.Code(), issue.Message())
				}
				return
			}
			if issue.Code() == "" || issue.Message() == "" {
				t.Fatalf("issue %v missing code/message", issue)
			}
		})
	}

	if _, ok := NormalizeSchemaLocation(" \t\r\n "); CheckIncludeSchemaLocation(ok) != IncludeLocationMissing {
		t.Fatal("CheckIncludeSchemaLocation accepted whitespace-normalized empty location")
	}
}

func TestSchemaDocumentReferences(t *testing.T) {
	t.Parallel()

	attrs := func(values map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		}
	}
	got := SchemaDocumentReferences([]SchemaReferenceElement{
		{Local: vocab.XSDElemElement, Attr: attrs(map[string]string{vocab.XSDAttrSchemaLocation: "ignored.xsd"})},
		{Local: vocab.XSDElemInclude, Attr: attrs(map[string]string{vocab.XSDAttrSchemaLocation: " \tcommon.xsd\n "}), Line: 12, Column: 3},
		{Local: vocab.XSDElemInclude, Attr: attrs(nil)},
		{Local: vocab.XSDElemImport, Attr: attrs(map[string]string{
			vocab.XSDAttrNamespace:      "urn:types",
			vocab.XSDAttrSchemaLocation: "types.xsd",
		}), Line: 14, Column: 5},
	})
	want := []SchemaDocumentReference{
		{Location: "common.xsd", Line: 12, Column: 3},
		{Kind: SchemaReferenceImport, Namespace: "urn:types", Location: "types.xsd", Line: 14, Column: 5},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SchemaDocumentReferences() = %#v, want %#v", got, want)
	}
}

func TestResolveSchemaReferences(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 4)
	resolver := Resolver(func(base, location string) (Source, error) {
		calls = append(calls, base+" "+location)
		switch location {
		case "missing.xsd":
			return Source{}, xsderrors.ErrSchemaNotFound
		case "bad.xsd":
			return Source{}, errors.New("boom")
		default:
			return Empty("resolved/" + location), nil
		}
	})
	base := Empty("base.xsd").WithResolver(resolver)
	loads, aliases, err := ResolveSchemaReferences(base, "base-key", []SchemaDocumentReference{
		{Namespace: vocab.XMLNamespaceURI, Location: "xml.xsd"},
		{Location: "common.xsd"},
		{Kind: SchemaReferenceImport, Namespace: "urn:missing", Location: "missing.xsd"},
	})
	if err != nil {
		t.Fatalf("ResolveSchemaReferences() error = %v", err)
	}
	if !slices.Equal(calls, []string{"base.xsd common.xsd", "base.xsd missing.xsd"}) {
		t.Fatalf("resolver calls = %#v", calls)
	}
	if len(loads) != 1 || loads[0].Source.Name() != filepath.Join("resolved", "common.xsd") || !loads[0].OptionalMissing {
		t.Fatalf("loads = %#v", loads)
	}
	if loads[0].Source.Resolver() == nil {
		t.Fatal("resolved source did not inherit resolver")
	}
	wantAlias := Key(filepath.Join("resolved", "common.xsd"))
	if got := aliases[ReferenceKey{Base: "base-key", Location: "common.xsd"}]; got != wantAlias {
		t.Fatalf("alias = %q, want %q", got, wantAlias)
	}

	_, aliases, err = ResolveSchemaReferences(base, "base-key", []SchemaDocumentReference{{Location: "bad.xsd"}})
	if aliases != nil {
		t.Fatalf("aliases after resolver error = %#v, want nil", aliases)
	}
	resolveErr, ok := errors.AsType[*ResolveReferenceError](err)
	if !ok {
		t.Fatalf("ResolveSchemaReferences() error = %T %v, want ResolveReferenceError", err, err)
	}
	if resolveErr.Location != "bad.xsd" || resolveErr.Err == nil {
		t.Fatalf("ResolveReferenceError = %#v", resolveErr)
	}

	loads, aliases, err = ResolveSchemaReferences(base, "base-key", []SchemaDocumentReference{{Location: "missing.xsd"}})
	if err != nil {
		t.Fatalf("missing include error = %v, want nil", err)
	}
	if loads != nil || aliases != nil {
		t.Fatalf("loads/aliases after missing include = %#v/%#v, want nil/nil", loads, aliases)
	}
}

func TestLoadSchemaDocumentsReadsResolverQueue(t *testing.T) {
	t.Parallel()

	resolver := Resolver(func(base, location string) (Source, error) {
		if base != "schemas/root.xsd" {
			t.Fatalf("resolver base = %q, want root source", base)
		}
		switch location {
		case "common.xsd":
			return Bytes("schemas/common.xsd", []byte("common")), nil
		case "missing.xsd":
			return Source{}, xsderrors.ErrSchemaNotFound
		default:
			return Source{}, fmt.Errorf("unexpected location %q", location)
		}
	})
	var parsed []string
	result, err := LoadSchemaDocuments(
		[]Source{
			Bytes("schemas/root.xsd", []byte("root")).WithResolver(resolver),
			Bytes("schemas/root.xsd", []byte("duplicate root")),
		},
		1024,
		func(loaded LoadedSource) (LoadedSchemaDocument, error) {
			parsed = append(parsed, loaded.Key+":"+string(loaded.Data))
			switch loaded.Name {
			case "schemas/root.xsd":
				return LoadedSchemaDocument{
					TargetNamespace: "urn:root",
					References: []SchemaDocumentReference{
						{Location: "common.xsd"},
						{Kind: SchemaReferenceImport, Namespace: "urn:missing", Location: "missing.xsd"},
					},
				}, nil
			case "schemas/common.xsd":
				return LoadedSchemaDocument{TargetNamespace: "urn:common"}, nil
			default:
				return LoadedSchemaDocument{}, fmt.Errorf("unexpected parse source %q", loaded.Name)
			}
		},
	)
	if err != nil {
		t.Fatalf("LoadSchemaDocuments() error = %v", err)
	}
	rootKey := Key("schemas/root.xsd")
	commonKey := Key("schemas/common.xsd")
	if want := []string{rootKey + ":root", commonKey + ":common"}; !slices.Equal(parsed, want) {
		t.Fatalf("parsed = %#v, want %#v", parsed, want)
	}
	if got := result.ReferenceAliases[ReferenceKey{Base: rootKey, Location: "common.xsd"}]; got != commonKey {
		t.Fatalf("common alias = %q, want %q", got, commonKey)
	}
	if want := []string{commonKey, rootKey}; !slices.Equal(result.SelectedKeys, want) {
		t.Fatalf("SelectedKeys = %#v, want %#v", result.SelectedKeys, want)
	}
}

func TestLoadSchemaDocumentsSkipsOptionalMissingRead(t *testing.T) {
	t.Parallel()

	resolver := Resolver(func(_, location string) (Source, error) {
		if location != "optional.xsd" {
			return Source{}, fmt.Errorf("unexpected location %q", location)
		}
		return Source{name: "optional.xsd", err: xsderrors.ErrSchemaNotFound}, nil
	})
	var parsed []string
	result, err := LoadSchemaDocuments(
		[]Source{Bytes("root.xsd", []byte("root")).WithResolver(resolver)},
		1024,
		func(loaded LoadedSource) (LoadedSchemaDocument, error) {
			parsed = append(parsed, loaded.Name)
			return LoadedSchemaDocument{References: []SchemaDocumentReference{{Kind: SchemaReferenceImport, Location: "optional.xsd"}}}, nil
		},
	)
	if err != nil {
		t.Fatalf("LoadSchemaDocuments() error = %v", err)
	}
	if want := []string{"root.xsd"}; !slices.Equal(parsed, want) {
		t.Fatalf("parsed = %#v, want %#v", parsed, want)
	}
	if want := []string{Key("root.xsd")}; !slices.Equal(result.SelectedKeys, want) {
		t.Fatalf("SelectedKeys = %#v, want %#v", result.SelectedKeys, want)
	}
}

func TestLoadSchemaDocumentsSkipsMissingIncludeRead(t *testing.T) {
	t.Parallel()

	resolver := Resolver(func(_, location string) (Source, error) {
		if location != "required.xsd" {
			return Source{}, fmt.Errorf("unexpected location %q", location)
		}
		return Source{name: "required.xsd", err: xsderrors.ErrSchemaNotFound}, nil
	})
	_, err := LoadSchemaDocuments(
		[]Source{Bytes("root.xsd", []byte("root")).WithResolver(resolver)},
		1024,
		func(loaded LoadedSource) (LoadedSchemaDocument, error) {
			return LoadedSchemaDocument{References: []SchemaDocumentReference{{Location: "required.xsd"}}}, nil
		},
	)
	if err != nil {
		t.Fatalf("LoadSchemaDocuments() error = %v, want nil", err)
	}
}

func TestCheckReferenceTargetNamespaces(t *testing.T) {
	t.Parallel()

	importTests := []struct {
		name             string
		namespace        string
		referencedTarget string
		wantIssue        TargetNamespaceIssue
	}{
		{name: "matching explicit namespace", namespace: "urn:a", referencedTarget: "urn:a"},
		{name: "matching absent namespace"},
		{name: "mismatch", namespace: "urn:a", referencedTarget: "urn:b", wantIssue: ImportTargetNamespaceMismatch},
	}
	for _, tt := range importTests {
		t.Run("import "+tt.name, func(t *testing.T) {
			t.Parallel()

			issue := CheckImportedTargetNamespace(tt.namespace, tt.referencedTarget)
			if issue != tt.wantIssue {
				t.Fatalf("CheckImportedTargetNamespace() = %v, want %v", issue, tt.wantIssue)
			}
			assertTargetNamespaceIssue(t, issue)
		})
	}

	includeTests := []struct {
		name             string
		target           string
		referencedTarget string
		wantIssue        TargetNamespaceIssue
	}{
		{name: "matching target", target: "urn:a", referencedTarget: "urn:a"},
		{name: "chameleon", target: "urn:a"},
		{name: "both no target"},
		{name: "mismatch", target: "urn:a", referencedTarget: "urn:b", wantIssue: IncludeTargetNamespaceMismatch},
		{name: "referenced target from no-target schema", referencedTarget: "urn:b", wantIssue: IncludeTargetNamespaceMismatch},
	}
	for _, tt := range includeTests {
		t.Run("include "+tt.name, func(t *testing.T) {
			t.Parallel()

			issue := CheckIncludedTargetNamespace(tt.target, tt.referencedTarget)
			if issue != tt.wantIssue {
				t.Fatalf("CheckIncludedTargetNamespace() = %v, want %v", issue, tt.wantIssue)
			}
			assertTargetNamespaceIssue(t, issue)
		})
	}
}

func TestCheckReferenceNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		visible   ReferenceNamespaces
		want      string
		wantIssue ReferenceNamespaceIssue
	}{
		{
			name:      "xsd namespace is always visible",
			namespace: "http://www.w3.org/2001/XMLSchema",
			want:      "http://www.w3.org/2001/XMLSchema",
		},
		{
			name:      "xml namespace is always visible",
			namespace: "http://www.w3.org/XML/1998/namespace",
			want:      "http://www.w3.org/XML/1998/namespace",
		},
		{
			name:      "target namespace is visible",
			namespace: "urn:target",
			visible:   ReferenceNamespaces{TargetNamespace: "urn:target"},
			want:      "urn:target",
		},
		{
			name:      "imported namespace is visible",
			namespace: "urn:imported",
			visible:   ReferenceNamespaces{Imports: map[string]bool{"urn:imported": true}},
			want:      "urn:imported",
		},
		{
			name:      "no-target schema can reference no namespace",
			namespace: "",
			want:      "",
		},
		{
			name:      "adopted chameleon reference resolves to target namespace",
			namespace: "",
			visible:   ReferenceNamespaces{TargetNamespace: "urn:target", AdoptedTarget: true},
			want:      "urn:target",
		},
		{
			name:      "unadopted target schema cannot reference no namespace",
			namespace: "",
			visible:   ReferenceNamespaces{TargetNamespace: "urn:target"},
			want:      "",
			wantIssue: ReferenceNamespaceNotImported,
		},
		{
			name:      "foreign namespace must be imported",
			namespace: "urn:foreign",
			visible:   ReferenceNamespaces{TargetNamespace: "urn:target"},
			want:      "urn:foreign",
			wantIssue: ReferenceNamespaceNotImported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, issue := CheckReferenceNamespace(tt.namespace, tt.visible)
			if got != tt.want || issue != tt.wantIssue {
				t.Fatalf("CheckReferenceNamespace() = (%q, %v), want (%q, %v)", got, issue, tt.want, tt.wantIssue)
			}
			if issue == ReferenceNamespaceOK {
				if issue.Code() != "" || issue.Message(got) != "" {
					t.Fatalf("OK issue has code/message: %q %q", issue.Code(), issue.Message(got))
				}
				return
			}
			if issue.Code() == "" || issue.Message(got) == "" {
				t.Fatalf("issue %v missing code/message", issue)
			}
		})
	}
}

func TestCheckChameleonIncludeAdoption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		target                   string
		referencedTarget         string
		resolvedKey              string
		existingTarget           string
		existingClones           map[string]bool
		wantAction               chameleonAdoptionAction
		wantCloneKey             string
		wantTargetNamespaceIssue TargetNamespaceIssue
	}{
		{
			name:           "adopt unresolved chameleon",
			target:         "urn:a",
			resolvedKey:    "common.xsd",
			wantAction:     chameleonAdoptSource,
			wantCloneKey:   "",
			existingClones: nil,
		},
		{
			name:           "same target already adopted",
			target:         "urn:a",
			resolvedKey:    "common.xsd",
			existingTarget: "urn:a",
		},
		{
			name:           "clone for different target",
			target:         "urn:b",
			resolvedKey:    "common.xsd",
			existingTarget: "urn:a",
			wantAction:     chameleonCloneSource,
			wantCloneKey:   "common.xsd\x00urn:b",
		},
		{
			name:           "clone already exists",
			target:         "urn:b",
			resolvedKey:    "common.xsd",
			existingTarget: "urn:a",
			existingClones: map[string]bool{"common.xsd\x00urn:b": true},
		},
		{
			name:             "non-chameleon include",
			target:           "urn:a",
			referencedTarget: "urn:a",
			resolvedKey:      "types.xsd",
		},
		{
			name:         "no target cannot adopt",
			resolvedKey:  "common.xsd",
			wantAction:   chameleonNoAdoption,
			wantCloneKey: "",
		},
		{
			name:                     "target mismatch",
			target:                   "urn:a",
			referencedTarget:         "urn:b",
			resolvedKey:              "types.xsd",
			wantTargetNamespaceIssue: IncludeTargetNamespaceMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := checkChameleonIncludeAdoption(chameleonAdoptionInput{
				CloneExists: func(key string) bool {
					return tt.existingClones[key]
				},
				TargetNamespace:           tt.target,
				ReferencedTargetNamespace: tt.referencedTarget,
				ResolvedKey:               tt.resolvedKey,
				ExistingTargetNamespace:   tt.existingTarget,
			})
			if got.Action != tt.wantAction || got.CloneKey != tt.wantCloneKey || got.Issue != tt.wantTargetNamespaceIssue {
				t.Fatalf("CheckChameleonIncludeAdoption() = %+v, want action %v clone %q issue %v", got, tt.wantAction, tt.wantCloneKey, tt.wantTargetNamespaceIssue)
			}
		})
	}
}

func TestChameleonCloneReferenceAliases(t *testing.T) {
	t.Parallel()

	got := chameleonCloneReferenceAliases("common.xsd\x00urn:b", []resolvedReference{
		{Location: "base.xsd", Key: "resolved/base.xsd"},
		{Location: "other.xsd", Key: "resolved/other.xsd"},
		{Location: "", Key: "ignored.xsd"},
		{Location: "ignored.xsd"},
	})
	want := map[ReferenceKey]string{
		{Base: "common.xsd\x00urn:b", Location: "base.xsd"}:  "resolved/base.xsd",
		{Base: "common.xsd\x00urn:b", Location: "other.xsd"}: "resolved/other.xsd",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChameleonCloneReferenceAliases() = %#v, want %#v", got, want)
	}
	if got := chameleonCloneReferenceAliases("clone", nil); got != nil {
		t.Fatalf("ChameleonCloneReferenceAliases(nil) = %#v, want nil", got)
	}
	if got := chameleonCloneReferenceAliases("clone", []resolvedReference{{Location: "missing"}}); got != nil {
		t.Fatalf("ChameleonCloneReferenceAliases(unresolved) = %#v, want nil", got)
	}
}

func TestPlanChameleonIncludesAdoptsAndClones(t *testing.T) {
	t.Parallel()

	docs := []ChameleonDocument{
		{
			Key:             "main-a.xsd",
			Name:            "main-a.xsd",
			TargetNamespace: "urn:a",
			References:      []SchemaDocumentReference{{Location: "common.xsd", Kind: SchemaReferenceInclude}},
			Loaded:          true,
		},
		{
			Key:             "main-b.xsd",
			Name:            "main-b.xsd",
			TargetNamespace: "urn:b",
			References:      []SchemaDocumentReference{{Location: "common.xsd", Kind: SchemaReferenceInclude}},
			Loaded:          true,
		},
		{
			Key:        "common.xsd",
			Name:       "common.xsd",
			References: []SchemaDocumentReference{{Location: "leaf.xsd", Kind: SchemaReferenceInclude}},
			Loaded:     true,
		},
		{
			Key:    "leaf.xsd",
			Name:   "leaf.xsd",
			Loaded: true,
		},
	}

	plan, issue := PlanChameleonIncludes(docs, nil, nil)
	if issue.Issue != TargetNamespaceOK {
		t.Fatalf("PlanChameleonIncludes() issue = %+v", issue)
	}
	if !plan.Changed() {
		t.Fatal("PlanChameleonIncludes() reported no changes")
	}
	cloneKey := "common.xsd\x00urn:b"
	wantTargets := map[string]string{
		"common.xsd": "urn:a",
		cloneKey:     "urn:b",
		"leaf.xsd":   "urn:a",
	}
	if !reflect.DeepEqual(plan.AdoptedTargets, wantTargets) {
		t.Fatalf("adopted targets = %#v, want %#v", plan.AdoptedTargets, wantTargets)
	}
	wantClones := []ChameleonClone{{SourceKey: "common.xsd", CloneKey: cloneKey, TargetNamespace: "urn:b"}}
	if !slices.Equal(plan.Clones, wantClones) {
		t.Fatalf("clones = %#v, want %#v", plan.Clones, wantClones)
	}
	wantAliases := map[ReferenceKey]string{
		{Base: cloneKey, Location: "leaf.xsd"}: "leaf.xsd",
	}
	if !reflect.DeepEqual(plan.ReferenceAliases, wantAliases) {
		t.Fatalf("reference aliases = %#v, want %#v", plan.ReferenceAliases, wantAliases)
	}
}

func TestPlanChameleonIncludesReportsTargetMismatch(t *testing.T) {
	t.Parallel()

	docs := []ChameleonDocument{
		{
			Key:        "main.xsd",
			Name:       "main.xsd",
			References: []SchemaDocumentReference{{Location: "types.xsd", Kind: SchemaReferenceInclude, Line: 7, Column: 9}},
			Loaded:     true,
		},
		{
			Key:             "types.xsd",
			Name:            "types.xsd",
			TargetNamespace: "urn:types",
			Loaded:          true,
		},
	}

	plan, issue := PlanChameleonIncludes(docs, nil, nil)
	if plan.Changed() {
		t.Fatalf("plan changed on mismatch: %+v", plan)
	}
	if issue.Issue != IncludeTargetNamespaceMismatch ||
		issue.DocumentKey != "main.xsd" ||
		issue.Location != "types.xsd" ||
		issue.Line != 7 ||
		issue.Column != 9 {
		t.Fatalf("issue = %+v, want include target mismatch at main.xsd:7:9", issue)
	}
}

func TestPlanChameleonIncludesResolvesLoadedOnlyDocuments(t *testing.T) {
	t.Parallel()

	docs := []ChameleonDocument{
		{
			Key:             "main.xsd",
			Name:            "main.xsd",
			TargetNamespace: "urn:main",
			References:      []SchemaDocumentReference{{Location: "deduped.xsd", Kind: SchemaReferenceInclude}},
			Loaded:          true,
		},
		{
			Key:        "deduped.xsd",
			Name:       "deduped.xsd",
			References: []SchemaDocumentReference{{Location: "leaf.xsd", Kind: SchemaReferenceInclude}},
			Loaded:     true,
			LoadedOnly: true,
		},
		{
			Key:    "leaf.xsd",
			Name:   "leaf.xsd",
			Loaded: true,
		},
	}

	plan, issue := PlanChameleonIncludes(docs, nil, nil)
	if issue.Issue != TargetNamespaceOK {
		t.Fatalf("PlanChameleonIncludes() issue = %+v", issue)
	}
	wantTargets := map[string]string{"deduped.xsd": "urn:main"}
	if !reflect.DeepEqual(plan.AdoptedTargets, wantTargets) {
		t.Fatalf("adopted targets = %#v, want %#v", plan.AdoptedTargets, wantTargets)
	}
}

func assertTargetNamespaceIssue(t *testing.T, issue TargetNamespaceIssue) {
	t.Helper()

	if issue == TargetNamespaceOK {
		if issue.Code() != "" || issue.Message() != "" {
			t.Fatalf("OK issue has code/message: %q %q", issue.Code(), issue.Message())
		}
		return
	}
	if issue.Code() == "" || issue.Message() == "" {
		t.Fatalf("issue %v missing code/message", issue)
	}
}

func TestLocationKeysReturnsUniqueResolutionCandidates(t *testing.T) {
	t.Parallel()

	got := LocationKeys("https://example.test/schemas/main.xsd", "", "../types.xsd")
	want := []string{"https://example.test/types.xsd", filepath.Clean("../types.xsd")}
	if !slices.Equal(got, want) {
		t.Fatalf("LocationKeys(url base) = %q, want %q", got, want)
	}

	got = LocationKeys("schemas/main.xsd", filepath.Clean("schemas/main.xsd"), "types.xsd")
	want = []string{filepath.Clean("schemas/types.xsd"), filepath.Clean("types.xsd")}
	if !slices.Equal(got, want) {
		t.Fatalf("LocationKeys(file base) = %q, want %q", got, want)
	}

	got = LocationKeys("main.xsd", filepath.Clean("main.xsd"), "types.xsd")
	want = []string{filepath.Clean("types.xsd")}
	if !slices.Equal(got, want) {
		t.Fatalf("LocationKeys(deduplicated) = %q, want %q", got, want)
	}
}

func TestNormalizeSchemaLocationCollapsesXMLWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "trim", in: "  types.xsd  ", want: "types.xsd", ok: true},
		{name: "collapse", in: "a\t\nb", want: "a b", ok: true},
		{name: "empty", in: " \t\r\n ", ok: false},
		{name: "nbsp", in: "\u00a0types.xsd\u00a0", want: "\u00a0types.xsd\u00a0", ok: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := NormalizeSchemaLocation(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("NormalizeSchemaLocation(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestSchemaLocationAttrReadsAndNormalizesXSDAttribute(t *testing.T) {
	t.Parallel()

	attrs := map[string]string{
		vocab.XSDAttrSchemaLocation: " types\tmain.xsd ",
	}
	got, ok := SchemaLocationAttr(func(local string) (string, bool) {
		value, ok := attrs[local]
		return value, ok
	})
	if !ok || got != "types main.xsd" {
		t.Fatalf("SchemaLocationAttr() = %q, %v; want %q, true", got, ok, "types main.xsd")
	}
	if got, ok := SchemaLocationAttr(nil); ok || got != "" {
		t.Fatalf("SchemaLocationAttr(nil) = %q, %v; want empty false", got, ok)
	}
}

func TestResolveLoadedSchemaLocationPrefersResolverAlias(t *testing.T) {
	t.Parallel()

	resolved := map[ReferenceKey]string{
		{Base: "main.xsd", Location: "mem:types"}: "resolved/types.xsd",
	}
	got, ok := ResolveLoadedSchemaLocation("main.xsd", "main.xsd", " mem:types ", resolved, func(key string) bool {
		return key == "resolved/types.xsd"
	})
	if !ok || got != "resolved/types.xsd" {
		t.Fatalf("ResolveLoadedSchemaLocation() = %q, %v, want resolved/types.xsd, true", got, ok)
	}
}

func TestResolveLoadedSchemaLocationFallsBackToCandidateKeys(t *testing.T) {
	t.Parallel()

	got, ok := ResolveLoadedSchemaLocation("https://example.test/schema/main.xsd", "", "types.xsd", nil, func(key string) bool {
		return key == "https://example.test/schema/types.xsd"
	})
	if !ok || got != "https://example.test/schema/types.xsd" {
		t.Fatalf("ResolveLoadedSchemaLocation(url) = %q, %v", got, ok)
	}

	want := filepath.Clean("schema/types.xsd")
	got, ok = ResolveLoadedSchemaLocation("schema/main.xsd", filepath.Clean("schema/main.xsd"), "types.xsd", nil, func(key string) bool {
		return key == want
	})
	if !ok || got != want {
		t.Fatalf("ResolveLoadedSchemaLocation(file) = %q, %v, want %q, true", got, ok, want)
	}
}

func TestResolveLoadedSchemaLocationRejectsEmptyLocation(t *testing.T) {
	t.Parallel()

	called := false
	got, ok := ResolveLoadedSchemaLocation("main.xsd", "main.xsd", " \t\r\n ", nil, func(string) bool {
		called = true
		return true
	})
	if ok || got != "" {
		t.Fatalf("ResolveLoadedSchemaLocation() = %q, %v, want empty false", got, ok)
	}
	if called {
		t.Fatal("ResolveLoadedSchemaLocation called loaded for empty location")
	}
}

func TestResolveLocalSchemaLocationFileURIHost(t *testing.T) {
	t.Parallel()

	if _, ok := ResolveLocalSchemaLocation("/tmp/main.xsd", "file://example.com/tmp/types.xsd"); ok {
		t.Fatal("ResolveLocalSchemaLocation() accepted non-local file URI host")
	}

	want := filepath.Clean(filepath.FromSlash("/tmp/types.xsd"))
	for _, location := range []string{"file:///tmp/types.xsd", "file://localhost/tmp/types.xsd"} {
		t.Run(location, func(t *testing.T) {
			t.Parallel()

			got, ok := ResolveLocalSchemaLocation("/tmp/main.xsd", location)
			if !ok {
				t.Fatalf("ResolveLocalSchemaLocation() ok = false")
			}
			if got != want {
				t.Fatalf("ResolveLocalSchemaLocation() = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveLocalSchemaLocationWindowsDrivePath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("drive-letter paths only resolve on windows")
	}
	got, ok := ResolveLocalSchemaLocation(`C:\schemas\main.xsd`, `C:\schemas\types.xsd`)
	if !ok {
		t.Fatal("ResolveLocalSchemaLocation() ok = false for drive-letter location")
	}
	if want := filepath.Clean(`C:\schemas\types.xsd`); got != want {
		t.Fatalf("ResolveLocalSchemaLocation() = %q, want %q", got, want)
	}
}

func TestSelectedLoadedDocumentKeysDropsDuplicateTargetContent(t *testing.T) {
	t.Parallel()

	docs := []LoadedDocument{
		{Key: "b.xsd", TargetNamespace: "urn:test", Data: []byte("same")},
		{Key: "a.xsd", TargetNamespace: "urn:test", Data: []byte("same")},
		{Key: "c.xsd", TargetNamespace: "urn:test", Data: []byte("different")},
		{Key: "empty-2.xsd", Data: []byte("same")},
		{Key: "empty-1.xsd", Data: []byte("same")},
		{Key: "other.xsd", TargetNamespace: "urn:other", Data: []byte("same")},
	}
	got := SelectedLoadedDocumentKeys(docs, nil)
	want := []string{"a.xsd", "c.xsd", "empty-1.xsd", "empty-2.xsd", "other.xsd"}
	if !slices.Equal(got, want) {
		t.Fatalf("SelectedLoadedDocumentKeys() = %q, want %q", got, want)
	}
}

func TestSelectedLoadedDocumentKeysKeepsSameContentAcrossTargets(t *testing.T) {
	t.Parallel()

	docs := []LoadedDocument{
		{Key: "b.xsd", TargetNamespace: "urn:b", Data: []byte("same")},
		{Key: "a.xsd", TargetNamespace: "urn:a", Data: []byte("same")},
	}
	got := SelectedLoadedDocumentKeys(docs, nil)
	want := []string{"a.xsd", "b.xsd"}
	if !slices.Equal(got, want) {
		t.Fatalf("SelectedLoadedDocumentKeys() = %q, want %q", got, want)
	}
}

func TestSelectedLoadedDocumentKeysKeepsSameContentWithDifferentResolvedGraphs(t *testing.T) {
	t.Parallel()

	aMain := filepath.Join("a", "main.xsd")
	aCommon := filepath.Join("a", "common.xsd")
	bMain := filepath.Join("b", "main.xsd")
	bCommon := filepath.Join("b", "common.xsd")
	docs := []LoadedDocument{
		{
			Name:            bMain,
			Key:             bMain,
			TargetNamespace: "urn:test",
			References:      []SchemaDocumentReference{{Kind: SchemaReferenceInclude, Location: "common.xsd"}},
			Data:            []byte("same"),
		},
		{
			Name:            aMain,
			Key:             aMain,
			TargetNamespace: "urn:test",
			References:      []SchemaDocumentReference{{Kind: SchemaReferenceInclude, Location: "common.xsd"}},
			Data:            []byte("same"),
		},
		{Name: aCommon, Key: aCommon, Data: []byte("a common")},
		{Name: bCommon, Key: bCommon, Data: []byte("b common")},
	}

	got := SelectedLoadedDocumentKeys(docs, nil)
	want := []string{aCommon, aMain, bCommon, bMain}
	if !slices.Equal(got, want) {
		t.Fatalf("SelectedLoadedDocumentKeys() = %q, want %q", got, want)
	}
}

func TestSelectedLoadedDocumentKeysDropsSameContentWithSameResolvedGraph(t *testing.T) {
	t.Parallel()

	aMain := filepath.Join("a", "main.xsd")
	bMain := filepath.Join("b", "main.xsd")
	shared := filepath.Join("shared", "common.xsd")
	docs := []LoadedDocument{
		{
			Name:            bMain,
			Key:             bMain,
			TargetNamespace: "urn:test",
			References:      []SchemaDocumentReference{{Kind: SchemaReferenceInclude, Location: "common.xsd"}},
			Data:            []byte("same"),
		},
		{
			Name:            aMain,
			Key:             aMain,
			TargetNamespace: "urn:test",
			References:      []SchemaDocumentReference{{Kind: SchemaReferenceInclude, Location: "common.xsd"}},
			Data:            []byte("same"),
		},
		{Name: shared, Key: shared, Data: []byte("shared")},
	}
	aliases := map[ReferenceKey]string{
		{Base: aMain, Location: "common.xsd"}: shared,
		{Base: bMain, Location: "common.xsd"}: shared,
	}

	got := SelectedLoadedDocumentKeys(docs, aliases)
	want := []string{aMain, shared}
	if !slices.Equal(got, want) {
		t.Fatalf("SelectedLoadedDocumentKeys() = %q, want %q", got, want)
	}
}
