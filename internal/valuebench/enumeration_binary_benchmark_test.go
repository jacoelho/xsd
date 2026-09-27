package valuebench

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkAuditStringEnumerationValidation(b *testing.B) {
	for _, size := range []int{8, 32, 64, 512, 1024} {
		for _, shape := range []string{"short", "commonprefix"} {
			program, id, values := auditStringEnumerationProgram(b, size, shape)
			for _, position := range []string{"first", "last", "miss"} {
				input := auditEnumerationInput(values, position, shape)
				raw := []byte(input)
				wantFacet := position == "miss"
				b.Run(fmt.Sprintf("n=%d/%s/%s", size, shape, position), func(b *testing.B) {
					b.Helper()
					b.ReportAllocs()
					b.SetBytes(int64(len(input)))
					for b.Loop() {
						_, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil)
						if (err != nil) != wantFacet || (wantFacet && !errors.Is(err, valuepkg.ErrFacet)) {
							b.Fatalf("ValidateBytes(%q) error = %v, want facet=%t", input, err, wantFacet)
						}
					}
				})
			}
		}
	}
}

func BenchmarkAuditBase64EnumerationValidation(b *testing.B) {
	for _, payloadSize := range []int{8, 64, 512} {
		for _, groupSize := range []int{8, 64, 512} {
			program, id, values := auditBase64EnumerationProgram(b, payloadSize, groupSize)
			for _, position := range []string{"first", "last", "miss"} {
				canonical := auditBase64EnumerationInput(payloadSize, groupSize, values, position)
				wantFacet := position == "miss"
				for _, spelling := range []string{"clean", "whitespace"} {
					input := canonical
					if spelling == "whitespace" {
						input = auditBase64Whitespace(canonical)
					}
					raw := []byte(input)
					for _, api := range []string{"string", "bytes"} {
						b.Run(fmt.Sprintf("bytes=%d/group=%d/%s/%s/%s", payloadSize, groupSize, spelling, api, position), func(b *testing.B) {
							b.Helper()
							b.ReportAllocs()
							b.SetBytes(int64(len(input)))
							for b.Loop() {
								var err error
								if api == "bytes" {
									_, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil)
								} else {
									_, err = program.Validate(id, input, valuepkg.Resolver{}, 0, 16<<20, nil)
								}
								if (err != nil) != wantFacet || (wantFacet && !errors.Is(err, valuepkg.ErrFacet)) {
									b.Fatalf("validation(%q) error = %v, want facet=%t", input, err, wantFacet)
								}
							}
						})
					}
				}
			}
		}
	}
}

func BenchmarkAuditBase64NonEnumeration(b *testing.B) {
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		b.Fatal(err)
	}
	id, ok := valuepkg.BuiltinTypeID("base64Binary")
	if !ok {
		b.Fatal("missing base64Binary builtin")
	}
	for _, payloadSize := range []int{8, 64, 512} {
		canonical := auditBase64Value(payloadSize, 7)
		for _, spelling := range []string{"clean", "whitespace"} {
			input := canonical
			if spelling == "whitespace" {
				input = auditBase64Whitespace(canonical)
			}
			raw := []byte(input)
			// The none rows measure the raw no-output control; canonical rows
			// force the parsed binary path changed by the candidate.
			for _, needs := range []struct {
				name  string
				value valuepkg.Needs
			}{
				{name: "none", value: 0},
				{name: "canonical", value: valuepkg.NeedCanonical},
			} {
				for _, api := range []string{"string", "bytes"} {
					b.Run(fmt.Sprintf("bytes=%d/%s/%s/%s", payloadSize, spelling, needs.name, api), func(b *testing.B) {
						b.Helper()
						b.ReportAllocs()
						b.SetBytes(int64(len(input)))
						for b.Loop() {
							var err error
							if api == "bytes" {
								_, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, needs.value, 16<<20, nil)
							} else {
								_, err = program.Validate(id, input, valuepkg.Resolver{}, needs.value, 16<<20, nil)
							}
							if err != nil {
								b.Fatal(err)
							}
						}
					})
				}
			}
		}
	}
}

func BenchmarkAuditStringEnumerationCompile(b *testing.B) {
	for _, size := range []int{32, 512, 1024} {
		for _, shape := range []string{"short", "commonprefix"} {
			literals := auditStringLiteralSpecs(auditStringEnumerationLiterals(size, shape))
			b.Run(fmt.Sprintf("n=%d/%s", size, shape), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
					if _, err := builder.Add(valuepkg.TypeSpec{
						Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
						Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
						Base: valuepkg.NoType, ListItem: valuepkg.NoType,
						Facets: valuepkg.FacetSpec{Enumeration: literals},
					}); err != nil {
						b.Fatal(err)
					}
					if _, err := builder.Seal(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func auditStringEnumerationProgram(tb testing.TB, size int, shape string) (*valuepkg.Program, valuepkg.TypeID, []string) {
	tb.Helper()
	values := auditStringEnumerationLiterals(size, shape)
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
		Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
		Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Facets: valuepkg.FacetSpec{Enumeration: auditStringLiteralSpecs(values)},
	})
	if err != nil {
		tb.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	return program, id, values
}

func auditBase64EnumerationProgram(tb testing.TB, payloadSize, groupSize int) (*valuepkg.Program, valuepkg.TypeID, []string) {
	tb.Helper()
	values := make([]string, groupSize)
	literals := make([]valuepkg.LiteralSpec, groupSize)
	for i := range values {
		values[i] = auditBase64Value(payloadSize, i)
		literals[i] = valuepkg.LiteralSpec{Lexical: values[i], Type: valuepkg.NoType}
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveBase64Binary,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
		Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Facets: valuepkg.FacetSpec{Enumeration: literals},
	})
	if err != nil {
		tb.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	return program, id, values
}

func auditStringEnumerationLiterals(size int, shape string) []string {
	values := make([]string, size)
	for i := range values {
		switch shape {
		case "short":
			values[i] = fmt.Sprintf("v%04d", i)
		case "commonprefix":
			values[i] = fmt.Sprintf("%s%04d", strings.Repeat("prefix-", 8), i)
		default:
			panic("unknown enumeration shape")
		}
	}
	return values
}

func auditStringLiteralSpecs(values []string) []valuepkg.LiteralSpec {
	literals := make([]valuepkg.LiteralSpec, len(values))
	for i, value := range values {
		literals[i] = valuepkg.LiteralSpec{Lexical: value, Type: valuepkg.NoType}
	}
	return literals
}

func auditBase64Value(size, index int) string {
	data := make([]byte, size)
	if len(data) > 0 {
		data[0] = byte(index & 0xff)
	}
	if len(data) > 1 {
		data[1] = byte((index >> 8) & 0xff)
	}
	for i := 2; i < len(data); i++ {
		data[i] = byte((index*37 + i*17) & 0xff)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func auditEnumerationInput(values []string, position, shape string) string {
	switch position {
	case "first":
		return values[0]
	case "last":
		return values[len(values)-1]
	case "miss":
		if shape == "commonprefix" {
			return fmt.Sprintf("%s9999", strings.Repeat("prefix-", 8))
		}
		if strings.HasPrefix(values[0], "v") {
			return "missing"
		}
		return values[len(values)-1] + "-miss"
	default:
		panic("unknown enumeration position")
	}
}

func auditBase64EnumerationInput(payloadSize, groupSize int, values []string, position string) string {
	switch position {
	case "first":
		return values[0]
	case "last":
		return values[len(values)-1]
	case "miss":
		return auditBase64Value(payloadSize, groupSize+1)
	default:
		panic("unknown enumeration position")
	}
}

func auditBase64Whitespace(input string) string {
	if len(input) < 2 {
		return " " + input + " "
	}
	mid := len(input) / 2
	return input[:mid] + " \n\t" + input[mid:]
}
