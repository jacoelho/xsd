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

// sharedAttributeGroupGrowthLimit lies above the measured growth of 1.00 and
// below the growths of 6.8 and 8.7 measured when the reachability walk or the
// attribute restriction check reads the shared set once per type.
const sharedAttributeGroupGrowthLimit = 1.4

// sharedAttributeGroupSchema returns an attribute group declaring the given
// number of attributes and the given number of global complex types, each
// referencing the group and nothing else, so every type shares one
// attribute-use set.
func sharedAttributeGroupSchema(types, attributes int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">`)
	b.WriteString(`<xs:attributeGroup name="G">`)
	for i := range attributes {
		fmt.Fprintf(&b, `<xs:attribute name="a%d" type="xs:string"/>`, i)
	}
	b.WriteString(`</xs:attributeGroup>`)
	for i := range types {
		fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:attributeGroup ref="t:G"/></xs:complexType>`, i)
	}
	b.WriteString(`<xs:element name="root" type="t:T0"/></xs:schema>`)
	return b.String()
}

// TestTypesSharingAnAttributeGroupReadItsSetOnce requires the statements of
// internal/schema per charged step of two added types that reference one
// shared attribute group to grow by at most sharedAttributeGroupGrowthLimit
// when the group grows from 128 to 2048 attributes. A type referencing the
// group is charged for the reference, not for the group's uses, so a pass
// that reads the shared set once per type, such as the reachability walk or
// the implicit restriction of xs:anyType, executes statements per use that
// no step pays for. The measurement limits of
// TestContentModelCompileWorkGrowsWithChargedSteps apply.
func TestTypesSharingAnAttributeGroupReadItsSetOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	driver := buildCompileWorkDriver(t, dir)
	const small, large = 128, 2048
	sizes := [4][2]int{{1, small}, {3, small}, {1, large}, {3, large}}
	var samples [4]compileWorkSample
	var errs [4]error
	var wg sync.WaitGroup
	for i, size := range sizes {
		wg.Go(func() {
			schema := sharedAttributeGroupSchema(size[0], size[1])
			samples[i], errs[i] = measureCompileWork(t.Context(), driver, filepath.Join(dir, fmt.Sprintf("shared-%d-%d", size[0], size[1])), schema, false)
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
			t.Fatalf("two added types measured work %d and steps %d, want both positive", added.work, added.steps)
		}
	}
	growth := addedLarge.perStep() / addedSmall.perStep()
	if !(growth <= sharedAttributeGroupGrowthLimit) {
		t.Errorf("two added types: %d attributes: work %d / steps %d = %.2f; %d attributes: work %d / steps %d = %.2f; growth %.3f, want at most %.2f",
			small, addedSmall.work, addedSmall.steps, addedSmall.perStep(), large, addedLarge.work, addedLarge.steps, addedLarge.perStep(), growth, sharedAttributeGroupGrowthLimit)
	}
}
