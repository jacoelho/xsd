package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaflow"
	"github.com/jacoelho/xsd/internal/semantic"
)

func TestValidateSchemaOwnedPathAllocatesLessThanLegacyClonePath(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(allocationHeavySchema(80)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	legacyAllocs := testing.AllocsPerRun(8, func() {
		if _, _, allocErr := validateSchemaLegacyClonePath(sch); allocErr != nil {
			panic(allocErr)
		}
	})
	ownedAllocs := testing.AllocsPerRun(8, func() {
		if _, _, allocErr := validateSchema(sch); allocErr != nil {
			panic(allocErr)
		}
	})

	if ownedAllocs >= legacyAllocs {
		t.Fatalf("owned validateSchema allocations = %.2f, want < legacy clone path %.2f", ownedAllocs, legacyAllocs)
	}
}

func validateSchemaLegacyClonePath(sch *parser.Schema) (*parser.Schema, *semantic.Registry, error) {
	if sch == nil {
		return nil, nil, fmt.Errorf("prepare schema: schema is nil")
	}
	cloned, err := loadmerge.CloneSchemaDeep(sch)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare schema: clone schema: %w", err)
	}
	resolvedSchema, err := schemaflow.ResolveAndValidate(cloned)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare schema: %w", err)
	}
	reg, err := semantic.AssignIDs(resolvedSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	if cycleErr := semantic.DetectCycles(resolvedSchema); cycleErr != nil {
		return nil, nil, fmt.Errorf("prepare schema: detect cycles: %w", cycleErr)
	}
	if upaErr := schemaflow.ValidateUPA(resolvedSchema, reg); upaErr != nil {
		return nil, nil, fmt.Errorf("prepare schema: validate UPA: %w", upaErr)
	}
	return resolvedSchema, reg, nil
}

func allocationHeavySchema(typeCount int) string {
	var b strings.Builder
	b.Grow(1024 + typeCount*256)
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"`)
	b.WriteString(` targetNamespace="urn:alloc" xmlns:tns="urn:alloc"`)
	b.WriteString(` elementFormDefault="qualified">`)
	for i := range typeCount {
		b.WriteString(`<xs:simpleType name="T`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"><xs:restriction base="xs:string"><xs:minLength value="1"/>`)
		b.WriteString(`</xs:restriction></xs:simpleType>`)
		b.WriteString(`<xs:complexType name="C`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"><xs:sequence><xs:element name="v" type="tns:T`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"/></xs:sequence></xs:complexType>`)
		b.WriteString(`<xs:element name="E`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`" type="tns:C`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"/>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}
