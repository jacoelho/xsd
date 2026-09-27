package valuebench

import (
	"fmt"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkAuditBase64FixedEquality(b *testing.B) {
	program, id := auditBase64BuiltinProgram(b)
	for _, payloadSize := range []int{8, 64, 512} {
		canonical := auditBase64Value(payloadSize, 7)
		for _, spelling := range []string{"clean", "whitespace"} {
			input := canonical
			if spelling == "whitespace" {
				input = auditBase64Whitespace(canonical)
			}
			inputBytes := []byte(input)
			literal, err := program.Validate(id, canonical, valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, 16<<20, nil)
			if err != nil {
				b.Fatalf("fixed literal validation bytes=%d/%s: %v", payloadSize, spelling, err)
			}
			for _, api := range []string{"string", "bytes"} {
				b.Run(fmt.Sprintf("bytes=%d/%s/%s", payloadSize, spelling, api), func(b *testing.B) {
					b.Helper()
					b.ReportAllocs()
					b.SetBytes(int64(len(input)))
					if got, err := validateBase64BenchmarkValue(program, id, input, inputBytes, api); err != nil {
						b.Fatalf("warmup validation bytes=%d/%s/%s: %v", payloadSize, spelling, api, err)
					} else if !got.Equal(literal) {
						b.Fatalf("warmup value bytes=%d/%s/%s did not equal fixed literal", payloadSize, spelling, api)
					}
					b.ResetTimer()
					for b.Loop() {
						got, err := validateBase64BenchmarkValue(program, id, input, inputBytes, api)
						if err != nil {
							b.Fatalf("validation bytes=%d/%s/%s: %v", payloadSize, spelling, api, err)
						}
						if !got.Equal(literal) {
							b.Fatalf("value bytes=%d/%s/%s did not equal fixed literal", payloadSize, spelling, api)
						}
					}
				})
			}
		}
	}
}

func BenchmarkAuditBase64EnumerationConstruction(b *testing.B) {
	for _, payloadSize := range []int{8, 512} {
		for _, groupSize := range []int{8, 512} {
			literals := auditBase64LiteralSpecs(payloadSize, groupSize)
			b.Run(fmt.Sprintf("bytes=%d/group=%d", payloadSize, groupSize), func(b *testing.B) {
				b.Helper()
				b.ReportAllocs()
				b.SetBytes(int64(payloadSize * groupSize))
				warmupBuilder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
				warmupID, err := warmupBuilder.Add(valuepkg.TypeSpec{
					Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveBase64Binary,
					Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
					Base: valuepkg.NoType, ListItem: valuepkg.NoType,
					Facets: valuepkg.FacetSpec{Enumeration: literals},
				})
				if err != nil {
					b.Fatal(err)
				}
				warmupProgram, err := warmupBuilder.Seal()
				if err != nil {
					b.Fatal(err)
				}
				if _, err := warmupProgram.Validate(warmupID, literals[0].Lexical, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
					b.Fatalf("warmup validation: %v", err)
				}
				b.ResetTimer()
				for b.Loop() {
					builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
					id, err := builder.Add(valuepkg.TypeSpec{
						Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveBase64Binary,
						Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
						Base: valuepkg.NoType, ListItem: valuepkg.NoType,
						Facets: valuepkg.FacetSpec{Enumeration: literals},
					})
					if err != nil || id == valuepkg.NoType {
						b.Fatalf("enumeration construction: id=%d err=%v", id, err)
					}
					program, err := builder.Seal()
					if err != nil || program == nil {
						b.Fatalf("enumeration sealing: program=%p err=%v", program, err)
					}
				}
			})
		}
	}
}

func auditBase64BuiltinProgram(tb testing.TB) (*valuepkg.Program, valuepkg.TypeID) {
	tb.Helper()
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		tb.Fatal(err)
	}
	id, ok := valuepkg.BuiltinTypeID("base64Binary")
	if !ok {
		tb.Fatal("missing base64Binary builtin")
	}
	return program, id
}

func validateBase64BenchmarkValue(program *valuepkg.Program, id valuepkg.TypeID, input string, inputBytes []byte, api string) (valuepkg.Value, error) {
	const needs = valuepkg.NeedCanonical | valuepkg.NeedIdentity
	if api == "bytes" {
		return program.ValidateBytes(id, inputBytes, valuepkg.Resolver{}, needs, 16<<20, nil)
	}
	return program.Validate(id, input, valuepkg.Resolver{}, needs, 16<<20, nil)
}

func auditBase64LiteralSpecs(payloadSize, groupSize int) []valuepkg.LiteralSpec {
	literals := make([]valuepkg.LiteralSpec, groupSize)
	for i := range literals {
		literals[i] = valuepkg.LiteralSpec{
			Lexical: auditBase64Value(payloadSize, i),
			Type:    valuepkg.NoType,
		}
	}
	return literals
}
