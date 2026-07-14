package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestNormalizeOptionsRejectsNegativeLimits(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "depth", opts: Options{MaxSchemaDepth: -1}},
		{name: "attributes", opts: Options{MaxSchemaAttributes: -1}},
		{name: "token bytes", opts: Options{MaxSchemaTokenBytes: -1}},
		{name: "source bytes", opts: Options{MaxSchemaSourceBytes: -1}},
		{name: "sources", opts: Options{MaxSchemaSources: -1}},
		{name: "total source bytes", opts: Options{MaxSchemaTotalBytes: -1}},
		{name: "references", opts: Options{MaxSchemaReferences: -1}},
		{name: "target contexts", opts: Options{MaxSchemaTargetContexts: -1}},
		{name: "instantiated nodes", opts: Options{MaxSchemaInstantiatedNodes: -1}},
		{name: "names", opts: Options{MaxSchemaNames: -1}},
		{name: "content model states", opts: Options{MaxContentModelStates: -1}},
		{name: "substitution closure entries", opts: Options{MaxSubstitutionClosureEntries: -1}},
		{name: "simple union member entries", opts: Options{MaxSimpleUnionMemberEntries: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeOptions(tt.opts)
			if err == nil {
				t.Fatal("NormalizeOptions() succeeded")
			}
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error type = %T, want *xsderrors.Error", err)
			}
			if xerr.Code != xsderrors.CodeSchemaLimit {
				t.Fatalf("code = %s, want %s", xerr.Code, xsderrors.CodeSchemaLimit)
			}
		})
	}
}

func TestNormalizeOptionsAppliesDefaultsAndCopiesLimits(t *testing.T) {
	limits, err := NormalizeOptions(Options{MaxSchemaNames: 7, MaxFiniteOccurs: 11})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	if limits.MaxSchemaDepth != defaultMaxSchemaDepth {
		t.Fatalf("MaxSchemaDepth = %d, want %d", limits.MaxSchemaDepth, defaultMaxSchemaDepth)
	}
	if limits.MaxSchemaAttributes != defaultMaxSchemaAttributes {
		t.Fatalf("MaxSchemaAttributes = %d, want %d", limits.MaxSchemaAttributes, defaultMaxSchemaAttributes)
	}
	if limits.MaxSchemaTokenBytes != defaultMaxSchemaTokenBytes {
		t.Fatalf("MaxSchemaTokenBytes = %d, want %d", limits.MaxSchemaTokenBytes, defaultMaxSchemaTokenBytes)
	}
	if limits.MaxSchemaSourceBytes != defaultMaxSchemaSourceBytes {
		t.Fatalf("MaxSchemaSourceBytes = %d, want %d", limits.MaxSchemaSourceBytes, defaultMaxSchemaSourceBytes)
	}
	if limits.MaxSchemaSources != defaultMaxSchemaSources {
		t.Fatalf("MaxSchemaSources = %d, want %d", limits.MaxSchemaSources, defaultMaxSchemaSources)
	}
	if limits.MaxSchemaTotalBytes != defaultMaxSchemaTotalBytes {
		t.Fatalf("MaxSchemaTotalBytes = %d, want %d", limits.MaxSchemaTotalBytes, defaultMaxSchemaTotalBytes)
	}
	if limits.MaxSchemaReferences != defaultMaxSchemaReferences {
		t.Fatalf("MaxSchemaReferences = %d, want %d", limits.MaxSchemaReferences, defaultMaxSchemaReferences)
	}
	if limits.MaxSchemaTargetContexts != defaultMaxSchemaTargetContexts {
		t.Fatalf("MaxSchemaTargetContexts = %d, want %d", limits.MaxSchemaTargetContexts, defaultMaxSchemaTargetContexts)
	}
	if limits.MaxSchemaInstantiatedNodes != defaultMaxSchemaInstantiatedNodes {
		t.Fatalf("MaxSchemaInstantiatedNodes = %d, want %d", limits.MaxSchemaInstantiatedNodes, defaultMaxSchemaInstantiatedNodes)
	}
	if limits.MaxContentModelStates != defaultMaxContentModelStates {
		t.Fatalf("MaxContentModelStates = %d, want %d", limits.MaxContentModelStates, defaultMaxContentModelStates)
	}
	if limits.MaxSubstitutionClosureEntries != defaultMaxSubstitutionClosureEntries {
		t.Fatalf("MaxSubstitutionClosureEntries = %d, want %d", limits.MaxSubstitutionClosureEntries, defaultMaxSubstitutionClosureEntries)
	}
	if limits.MaxSimpleUnionMemberEntries != defaultMaxSimpleUnionMemberEntries {
		t.Fatalf("MaxSimpleUnionMemberEntries = %d, want %d", limits.MaxSimpleUnionMemberEntries, defaultMaxSimpleUnionMemberEntries)
	}
	if limits.MaxSchemaNames != 7 {
		t.Fatalf("MaxSchemaNames = %d, want 7", limits.MaxSchemaNames)
	}
	if limits.MaxFiniteOccurs != 11 {
		t.Fatalf("MaxFiniteOccurs = %d, want 11", limits.MaxFiniteOccurs)
	}
}
