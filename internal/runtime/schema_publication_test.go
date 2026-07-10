package runtime

import (
	"reflect"
	"strings"
	"testing"
)

func TestPublishSchemaRejectsRawCorruptionWithoutMutation(t *testing.T) {
	badName := QName{Local: 1}
	build := SchemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}
	want := SchemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}

	_, err := PublishSchema(&build)
	if err == nil {
		t.Fatal("PublishSchema() succeeded for invalid name references")
	}
	if !reflect.DeepEqual(build, want) {
		t.Fatalf("PublishSchema() mutated failed build: got %#v want %#v", build, want)
	}
}

func TestProjectionAuditRejectsCorruption(t *testing.T) {
	audit := schemaAudit{
		build: SchemaBuild{Attributes: []AttributeDecl{{}}},
	}
	err := validateRuntimeReadProjections(&audit)
	if err == nil || !strings.Contains(err.Error(), "attribute declaration read projection count does not match declarations") {
		t.Fatalf("validateRuntimeReadProjections() error = %v", err)
	}
}
