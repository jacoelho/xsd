package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
)

func TestValidateDefaultOrFixedValueWithContextQNameContextHandling(t *testing.T) {
	qnameType := builtins.Get(builtins.TypeNameQName)
	if qnameType == nil {
		t.Fatal("missing QName builtin")
	}

	if err := validateDefaultOrFixedValueWithContext(nil, "p:name", qnameType, nil); err != nil {
		t.Fatalf("expected QName validation defer with nil context, got %v", err)
	}

	err := validateDefaultOrFixedValueWithContext(nil, "p:name", qnameType, map[string]string{"q": "urn:test"})
	if err == nil || !strings.Contains(err.Error(), "prefix p not found") {
		t.Fatalf("expected missing QName prefix error, got %v", err)
	}
}

func TestValidateDefaultOrFixedValueWithContextRejectsIDBuiltin(t *testing.T) {
	idType := builtins.Get(builtins.TypeNameID)
	if idType == nil {
		t.Fatal("missing ID builtin")
	}

	err := validateDefaultOrFixedValueWithContext(nil, "abc", idType, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot have default or fixed values") {
		t.Fatalf("expected ID default/fixed error, got %v", err)
	}
}
