package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeQNameSetsCanonicalAndKey(t *testing.T) {
	t.Parallel()

	var sess Session
	metrics := &ValueMetrics{}

	canonical, err := sess.canonicalizeQName(
		runtime.ValidatorMeta{Kind: runtime.VQName},
		[]byte("p:local"),
		mapResolver{"p": "urn:test"},
		true,
		metrics,
	)
	if err != nil {
		t.Fatalf("canonicalizeQName() error = %v", err)
	}
	if got := string(canonical); got != "urn:test\x00local" {
		t.Fatalf("canonicalizeQName() canonical = %q, want %q", got, "urn:test\x00local")
	}

	keyKind, keyBytes, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeQName() should set key state")
	}
	if keyKind != runtime.VKQName {
		t.Fatalf("canonicalizeQName() key kind = %v, want %v", keyKind, runtime.VKQName)
	}
	wantKey := runtime.QNameKeyCanonical(nil, 0, canonical)
	if !bytes.Equal(keyBytes, wantKey) {
		t.Fatalf("canonicalizeQName() key = %v, want %v", keyBytes, wantKey)
	}
	if !bytes.Equal(sess.valueScratch, canonical) {
		t.Fatalf("canonicalizeQName() session value scratch = %v, want %v", sess.valueScratch, canonical)
	}
	if !bytes.Equal(sess.keyTmp, keyBytes) {
		t.Fatalf("canonicalizeQName() session key scratch = %v, want %v", sess.keyTmp, keyBytes)
	}
}

func TestCanonicalizeQNameNotationRequiresDeclaration(t *testing.T) {
	t.Parallel()

	var sess Session

	_, err := sess.canonicalizeQName(
		runtime.ValidatorMeta{Kind: runtime.VNotation},
		[]byte("p:local"),
		mapResolver{"p": "urn:test"},
		false,
		nil,
	)
	if err == nil || err.Error() != "notation not declared" {
		t.Fatalf("canonicalizeQName() error = %v, want notation not declared", err)
	}
}

func TestCanonicalizeQNameNotationDeclaredSetsKey(t *testing.T) {
	t.Parallel()

	builder := runtime.NewBuilder()
	nsID := mustInternNamespace(t, builder, []byte("urn:test"))
	notationSym := mustInternSymbol(t, builder, nsID, []byte("local"))
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	schema.Notations = []runtime.SymbolID{notationSym}

	sess := NewSession(schema)
	metrics := &ValueMetrics{}

	canonical, err := sess.canonicalizeQName(
		runtime.ValidatorMeta{Kind: runtime.VNotation},
		[]byte("p:local"),
		mapResolver{"p": "urn:test"},
		true,
		metrics,
	)
	if err != nil {
		t.Fatalf("canonicalizeQName() error = %v", err)
	}

	keyKind, keyBytes, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeQName() should set key state for notation")
	}
	if keyKind != runtime.VKQName {
		t.Fatalf("canonicalizeQName() key kind = %v, want %v", keyKind, runtime.VKQName)
	}
	wantKey := runtime.QNameKeyCanonical(nil, 1, canonical)
	if !bytes.Equal(keyBytes, wantKey) {
		t.Fatalf("canonicalizeQName() notation key = %v, want %v", keyBytes, wantKey)
	}
}
