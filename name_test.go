package xsd_test

import (
	"testing"

	"github.com/jacoelho/xsd"
)

func TestNameString(t *testing.T) {
	if got := (xsd.Name{Local: "root"}).String(); got != "root" {
		t.Fatalf("Name.String() = %q, want root", got)
	}
	if got := (xsd.Name{Namespace: "urn:test", Local: "root"}).String(); got != "{urn:test}root" {
		t.Fatalf("Name.String() = %q, want {urn:test}root", got)
	}
	if !(xsd.Name{}).IsZero() {
		t.Fatal("zero Name IsZero() = false")
	}
}
