//go:build !(386 || arm || mips || mipsle || wasm)

package tests_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// attributeDerivationSchema returns a base type with attributes attributes
// and derivations types that each extend it with nothing or restrict it by
// redeclaring every attribute.
func attributeDerivationSchema(method string, derivations, attributes int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">`)
	var uses strings.Builder
	for i := range attributes {
		fmt.Fprintf(&uses, `<xs:attribute name="a%d" type="xs:string"/>`, i)
	}
	b.WriteString(`<xs:complexType name="B">` + uses.String() + `</xs:complexType>`)
	for d := range derivations {
		body := ""
		if method == "restriction" {
			body = uses.String()
		}
		fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:complexContent><xs:%s base="t:B">%s</xs:%s></xs:complexContent></xs:complexType>`, d, method, body, method)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

// TestAttributeDerivationAuditDoesWorkInProportionToChargedSteps pins the
// publication audit of a derived attribute-use set to lookups through the
// set's index: the statements of internal/schema per charged step that each
// added derived type costs may grow by at most compileWorkGrowthLimit when the
// base grows from 128 to 2048 attributes. A lookup that scans the other set's
// uses executes a statement per use scanned, so its work per step grows with
// the attribute count. The measurement limits of
// TestSequenceRestrictionOfAnAllGroupDoesWorkInProportionToChargedSteps apply.
func TestAttributeDerivationAuditDoesWorkInProportionToChargedSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	driver := buildCompileWorkDriver(t, dir)
	const small, large = 128, 2048
	for _, method := range []string{"extension", "restriction"} {
		t.Run(method, func(t *testing.T) {
			sizes := [4][2]int{{1, small}, {3, small}, {1, large}, {3, large}}
			var samples [4]compileWorkSample
			var errs [4]error
			var wg sync.WaitGroup
			for i, size := range sizes {
				wg.Go(func() {
					schema := attributeDerivationSchema(method, size[0], size[1])
					samples[i], errs[i] = measureCompileWork(t.Context(), driver, filepath.Join(dir, fmt.Sprintf("%s-%d-%d", method, size[0], size[1])), schema)
				})
			}
			wg.Wait()
			if err := errors.Join(errs[:]...); err != nil {
				t.Fatal(err)
			}
			addedSmall := samples[1].minus(samples[0])
			addedLarge := samples[3].minus(samples[2])
			for _, added := range []compileWorkSample{addedSmall, addedLarge} {
				if added.work <= 0 || added.steps <= 0 {
					t.Fatalf("two added derivations measured work %d and steps %d, want both positive", added.work, added.steps)
				}
			}
			growth := addedLarge.perStep() / addedSmall.perStep()
			if !(growth <= compileWorkGrowthLimit) {
				t.Errorf("two added derivations: %d attributes: work %d / steps %d = %.2f; %d attributes: work %d / steps %d = %.2f; growth %.3f, want at most %.2f",
					small, addedSmall.work, addedSmall.steps, addedSmall.perStep(), large, addedLarge.work, addedLarge.steps, addedLarge.perStep(), growth, compileWorkGrowthLimit)
			}
		})
	}
}
