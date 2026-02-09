package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseWithImportsAllocationScalesLinearly(t *testing.T) {
	smallSchema := allocationScaleSchema(10)
	largeSchema := allocationScaleSchema(100)

	smallAllocs := testing.AllocsPerRun(20, func() {
		if _, err := ParseWithImportsOptions(strings.NewReader(smallSchema)); err != nil {
			panic(err)
		}
	})
	largeAllocs := testing.AllocsPerRun(10, func() {
		if _, err := ParseWithImportsOptions(strings.NewReader(largeSchema)); err != nil {
			panic(err)
		}
	})

	// 100 components should stay close to linear relative to 10 components.
	// Keep slack to avoid flakes across Go versions while still catching
	// accidental superlinear regressions.
	if largeAllocs > smallAllocs*12 {
		t.Fatalf("ParseWithImports allocations grew too fast: small=%.2f large=%.2f ratio=%.2f", smallAllocs, largeAllocs, largeAllocs/smallAllocs)
	}
}

func allocationScaleSchema(componentPairs int) string {
	var b strings.Builder
	b.Grow(1024 + componentPairs*192)
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"`)
	b.WriteString(` targetNamespace="urn:alloc" xmlns:tns="urn:alloc"`)
	b.WriteString(` elementFormDefault="qualified">`)
	b.WriteString(`<xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>`)
	b.WriteString(`<xs:include schemaLocation="inc.xsd"/>`)
	for i := range componentPairs {
		b.WriteString(`<xs:simpleType name="T`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"><xs:restriction base="xs:string"><xs:minLength value="1"/>`)
		b.WriteString(`</xs:restriction></xs:simpleType>`)
		b.WriteString(`<xs:element name="E`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`" type="tns:T`)
		b.WriteString(fmt.Sprintf("%d", i))
		b.WriteString(`"/>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}
