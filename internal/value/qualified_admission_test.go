package value

import (
	"errors"
	"testing"
)

type qualifiedValueAPI struct {
	name       string
	workFactor uint64
	validate   func(TypeID, string, Resolver, Needs, uint64) (Value, error)
}

func qualifiedValueAPIs(p *Program) []qualifiedValueAPI {
	return []qualifiedValueAPI{
		{"Validate", 1, func(id TypeID, text string, resolver Resolver, needs Needs, work uint64) (Value, error) {
			return p.Validate(id, text, resolver, needs, work, nil)
		}},
		{"Admitted", 1, func(id TypeID, text string, resolver Resolver, needs Needs, work uint64) (Value, error) {
			return p.ValidateAdmitted(id, text, resolver, needs, work, nil)
		}},
		{"Bytes", 2, func(id TypeID, text string, resolver Resolver, needs Needs, work uint64) (Value, error) {
			return p.ValidateBytes(id, []byte(text), resolver, needs, work, nil)
		}},
	}
}

func assertUnprojectedQualifiedValue(t *testing.T, got Value, id TypeID) {
	t.Helper()
	if got.Type() != id || got.SelectedType() != NoType || !got.HasQualifiedNames() || got.HasIdentity() ||
		got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" {
		t.Fatalf("unprojected qualified value = %#v, want type %d, NoType selection and only qualified-name metadata", got, id)
	}
}

func TestUnprojectedQualifiedValueAdmission(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxDepth: 1})
	custom, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if completionErr := builder.Complete(custom, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveQName, Whitespace: WhitespaceCollapse,
		WhitespacePresent: true, Base: NoType, ListItem: NoType,
	}); completionErr != nil {
		t.Fatal(completionErr)
	}
	p, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range qualifiedValueAPIs(p) {
		t.Run(api.name, func(t *testing.T) {
			for _, shape := range []struct {
				name     string
				id       TypeID
				notation bool
			}{
				{"QName", builtinQName, false},
				{"NOTATION", builtinNotation, true},
				{"custom QName", custom, false},
			} {
				t.Run(shape.name, func(t *testing.T) {
					for _, input := range []struct {
						text  string
						exact uint64
					}{{"p:item", 7}, {" p:item ", 9}} {
						qnames, notations := 0, 0
						resolver := Resolver{
							QName: func(text string) (ExpandedName, bool) {
								qnames++
								if text != "p:item" {
									t.Errorf("QName callback input = %q, want p:item", text)
								}
								return ExpandedName{Namespace: "urn:test", Local: "item"}, true
							},
							Notation: func(namespace, local string) bool {
								notations++
								if qnames != 1 || namespace != "urn:test" || local != "item" {
									t.Errorf("NOTATION callback = (%q,%q), QName calls %d", namespace, local, qnames)
								}
								return true
							},
						}
						exact := input.exact * api.workFactor
						if _, err := api.validate(shape.id, input.text, resolver, 0, exact-1); !errors.Is(err, ErrLimit) {
							t.Fatalf("%q below work boundary = %v, want ErrLimit", input.text, err)
						}
						if qnames != 0 || notations != 0 {
							t.Fatal("resolver ran before work admission")
						}
						for range 2 {
							qnames, notations = 0, 0
							got, err := api.validate(shape.id, input.text, resolver, 0, exact)
							if err != nil {
								t.Fatal(err)
							}
							assertUnprojectedQualifiedValue(t, got, shape.id)
							wantNotation := 0
							if shape.notation {
								wantNotation = 1
							}
							if qnames != 1 || notations != wantNotation {
								t.Fatalf("resolver calls = (%d,%d), want (1,%d)", qnames, notations, wantNotation)
							}
						}
					}
				})
			}
		})
	}
}

func TestUnprojectedNotationWithoutQNameResolver(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range qualifiedValueAPIs(p) {
		t.Run(api.name, func(t *testing.T) {
			calls := 0
			resolver := Resolver{Notation: func(namespace, local string) bool {
				calls++
				if namespace != "" || local != "item" {
					t.Errorf("NOTATION lookup = (%q,%q), want ('',item)", namespace, local)
				}
				return true
			}}
			got, err := api.validate(builtinNotation, "item", resolver, 0, 10)
			if err != nil {
				t.Fatal(err)
			}
			assertUnprojectedQualifiedValue(t, got, builtinNotation)
			if calls != 1 {
				t.Fatalf("NOTATION calls = %d, want 1", calls)
			}
			calls = 0
			if _, err := api.validate(builtinNotation, "p:item", resolver, 0, 14); err == nil {
				t.Fatal("prefixed NOTATION without QName resolver succeeded")
			}
			if calls != 0 {
				t.Fatal("declaration lookup ran after invalid QName")
			}
		})
	}
}

func TestQualifiedValueExternalAdmissionPrecedesResolution(t *testing.T) {
	for _, input := range []struct{ text, reason string }{
		{"p:it\x00m", "invalid XML character"},
		{"p:it\xffm", "invalid UTF-8 value"},
	} {
		for _, builderAPI := range []bool{false, true} {
			for _, work := range []uint64{6, 7} {
				builder := NewBuilder(BuilderOptions{MaxConstructionWork: work})
				calls := 0
				resolver := Resolver{QName: func(string) (ExpandedName, bool) { calls++; return ExpandedName{Local: "item"}, true }}
				var err error
				if builderAPI {
					_, err = builder.Validate(builtinQName, input.text, resolver, 0, nil)
				} else {
					p, sealErr := builder.Seal()
					if sealErr != nil {
						t.Fatal(sealErr)
					}
					_, err = p.Validate(builtinQName, input.text, resolver, 0, work, nil)
				}
				if work == 6 {
					if !errors.Is(err, ErrLimit) {
						t.Fatalf("below admission boundary = %v, want ErrLimit", err)
					}
				} else if cause := errors.Unwrap(err); cause == nil || cause.Error() != input.reason {
					t.Fatalf("external admission error = %v, underlying %v; want %q", err, cause, input.reason)
				}
				if calls != 0 {
					t.Fatal("resolver ran before external admission")
				}
			}
		}
	}
}

func TestBuilderUnprojectedQualifiedValueBudget(t *testing.T) {
	for _, work := range []uint64{6, 7} {
		builder := NewBuilder(BuilderOptions{MaxConstructionWork: work})
		calls := 0
		resolver := Resolver{QName: func(string) (ExpandedName, bool) {
			calls++
			return ExpandedName{Namespace: "urn:test", Local: "item"}, true
		}}
		for range 2 {
			got, err := builder.Validate(builtinQName, "p:item", resolver, 0, nil)
			if work == 6 {
				if !errors.Is(err, ErrLimit) {
					t.Fatalf("builder below boundary = %v, want ErrLimit", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertUnprojectedQualifiedValue(t, got, builtinQName)
			}
		}
		wantCalls := 0
		if work == 7 {
			wantCalls = 2
		}
		if calls != wantCalls {
			t.Fatalf("builder resolver calls = %d, want %d", calls, wantCalls)
		}
	}
}

func TestUnprojectedQNameFailureKeepsOriginalSpelling(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range qualifiedValueAPIs(p) {
		t.Run(api.name, func(t *testing.T) {
			resolver := Resolver{QName: func(text string) (ExpandedName, bool) {
				if text != "bad::name" {
					t.Errorf("resolver input = %q, want bad::name", text)
				}
				return ExpandedName{}, false
			}}
			_, err := api.validate(builtinQName, " bad::name ", resolver, 0, 24)
			const want = `value " bad::name " is not valid for xs:QName: invalid QName syntax; expected a local name or prefix:local name`
			if err == nil || err.Error() != want {
				t.Fatalf("QName diagnostic = %v, want %s", err, want)
			}
			if cause := errors.Unwrap(err); cause == nil || cause.Error() != "unresolved QName" {
				t.Fatalf("QName cause = %v, want unresolved QName", cause)
			}
		})
	}
}
