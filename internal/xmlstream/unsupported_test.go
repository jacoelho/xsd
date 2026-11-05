package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

func unsupportedFeatureIs(err error, want UnsupportedFeature) bool {
	got, ok := UnsupportedFeatureOf(err)
	return ok && got == want
}

// TestUnsupportedFeatureClassification pins the project contract (XML 1.0,
// UTF-8, no DTD, no general entities) and the invariant that a reader error
// has Kind ErrorUnsupported exactly when it classifies as a feature.
func TestUnsupportedFeatureClassification(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		limits  Limits
		feature UnsupportedFeature
	}{
		{name: "doctype", xml: `<!DOCTYPE r><r/>`, feature: UnsupportedDTD},
		{name: "doctype internal subset", xml: `<!DOCTYPE r [<!ENTITY x "y">]><r/>`, feature: UnsupportedDTD},
		{name: "declared latin1", xml: `<?xml version="1.0" encoding="ISO-8859-1"?><r/>`, feature: UnsupportedNonUTF8},
		{name: "utf16 bom", xml: "\xFE\xFF<r/>", feature: UnsupportedNonUTF8},
		{name: "xml 1.1", xml: `<?xml version="1.1"?><r/>`, feature: UnsupportedXMLVersion},
		{name: "text entity", xml: `<r>&x;</r>`, feature: UnsupportedEntityReference},
		{name: "attribute entity", xml: `<r a="&x;"/>`, feature: UnsupportedEntityReference},
		{name: "predefined entity", xml: `<r>&amp;</r>`},
		{name: "colon entity name", xml: `<r>&a:b;</r>`},
		{name: "colon PI target", xml: `<r><?a:b?></r>`},
		{name: "token limit", xml: `<r>text</r>`, limits: Limits{MaxTokenBytes: 1}},
		{name: "input limit", xml: `<r>text</r>`, limits: Limits{MaxInputBytes: 4}},
		{name: "malformed", xml: `<r></s>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader Reader
			err := reader.Reset(strings.NewReader(tt.xml), Config{Limits: tt.limits})
			if err == nil {
				err = drainReader(&reader)
			}
			if tt.feature == 0 && err == nil {
				return
			}
			if err == nil {
				t.Fatalf("document accepted, want feature %d", tt.feature)
			}
			feature, ok := UnsupportedFeatureOf(err)
			if feature != tt.feature || ok != (tt.feature != 0) {
				t.Fatalf("UnsupportedFeatureOf(%v) = %d, %t; want %d", err, feature, ok, tt.feature)
			}
			boundary, isBoundary := errors.AsType[*Error](err)
			if !isBoundary {
				t.Fatalf("error %T = %v, want *Error", err, err)
			}
			if (boundary.Kind == ErrorUnsupported) != ok {
				t.Fatalf("Kind = %d, classified = %t; want ErrorUnsupported exactly when classified", boundary.Kind, ok)
			}
		})
	}
}
