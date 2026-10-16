package value_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func constructionQName(lexical string) (value.ExpandedName, bool) {
	switch lexical {
	case "p:x":
		return value.ExpandedName{Namespace: "urn:construction", Local: "x"}, true
	case "p:y":
		return value.ExpandedName{Namespace: "urn:construction", Local: "y"}, true
	}
	return value.ExpandedName{}, false
}

func constructionQNameSpec(resolve value.QNameResolver) value.TypeSpec {
	return atomicFacetType(value.PrimitiveQName, value.FacetSpec{
		Enumeration: []value.LiteralSpec{{Type: value.NoType, Lexical: "p:x", Resolver: value.Resolver{QName: resolve}}},
	})
}

func requireConstructionEnumeration(t *testing.T, builder *value.Builder, id value.TypeID) {
	t.Helper()
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	resolver := value.Resolver{QName: constructionQName}
	if _, validationErr := program.Validate(id, "p:x", resolver, 0, 16<<20, nil); validationErr != nil {
		t.Fatalf("Validate(p:x) = %v", validationErr)
	}
	_, err = program.Validate(id, "p:y", resolver, 0, 16<<20, nil)
	if !errors.Is(err, value.ErrFacet) || err.Error() != "enumeration facet failed" {
		t.Fatalf("Validate(p:y) = %v; want enumeration rejection", err)
	}
}

func requireActiveCompletionSealFailure(t *testing.T, builder *value.Builder) {
	t.Helper()
	program, err := builder.Seal()
	if program != nil || !errors.Is(err, value.ErrMetadata) {
		t.Fatalf("Seal during Complete = %v, %v; want nil, metadata error", program, err)
	}
}

func TestBuilderCallbackCannotSealBeforeCompletion(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	observed := false
	spec := constructionQNameSpec(func(string) (value.ExpandedName, bool) {
		observed = true
		requireActiveCompletionSealFailure(t, builder)
		return value.ExpandedName{}, false
	})
	err = builder.Complete(id, spec)
	if err == nil || err.Error() != `enumeration "p:x": unresolved QName` || !observed {
		t.Fatalf("Complete = %v, callback observed = %t", err, observed)
	}
	if _, available := builder.TypeView(id); available {
		t.Fatal("failed completion exposed metadata")
	}
	spec = constructionQNameSpec(func(lexical string) (value.ExpandedName, bool) {
		requireActiveCompletionSealFailure(t, builder)
		return constructionQName(lexical)
	})
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatal(completionErr)
	}
	requireConstructionEnumeration(t, builder, id)
}

func TestBuilderCallbackGrowthPreservesEnumeration(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec := constructionQNameSpec(func(lexical string) (value.ExpandedName, bool) {
		reserved, reserveErr := builder.Reserve()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		program, sealErr := builder.Seal()
		want := fmt.Sprintf("type %d: value program metadata is invalid", reserved)
		if program != nil || !errors.Is(sealErr, value.ErrMetadata) || sealErr.Error() != want {
			t.Fatalf("Seal with reserved child = %v, %v; want %q", program, sealErr, want)
		}
		if completionErr := builder.Complete(reserved, atomicFacetType(value.PrimitiveString, value.FacetSpec{})); completionErr != nil {
			t.Fatal(completionErr)
		}
		for range 128 {
			child, reserveErr := builder.Reserve()
			if reserveErr != nil {
				t.Fatal(reserveErr)
			}
			if completionErr := builder.Complete(child, atomicFacetType(value.PrimitiveString, value.FacetSpec{})); completionErr != nil {
				t.Fatal(completionErr)
			}
		}
		requireActiveCompletionSealFailure(t, builder)
		return constructionQName(lexical)
	})
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatal(completionErr)
	}
	requireConstructionEnumeration(t, builder, id)
}

func TestBuilderCallbackCannotSpendParentStorage(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1024})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	observed := false
	spec := constructionQNameSpec(func(lexical string) (value.ExpandedName, bool) {
		observed = true
		child, reserveErr := builder.Reserve()
		if child != value.NoType || !errors.Is(reserveErr, value.ErrLimit) {
			t.Fatalf("nested Reserve = %v, %v; want storage limit", child, reserveErr)
		}
		return constructionQName(lexical)
	})
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatal(completionErr)
	}
	if !observed {
		t.Fatal("resolver was not called")
	}
	requireConstructionEnumeration(t, builder, id)
}

func TestBuilderCallbackFailureRefundPreservesChild(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1070})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	child := value.NoType
	spec := constructionQNameSpec(func(string) (value.ExpandedName, bool) {
		var reserveErr error
		child, reserveErr = builder.Reserve()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		if completionErr := builder.Complete(child, atomicFacetType(value.PrimitiveString, value.FacetSpec{})); completionErr != nil {
			t.Fatal(completionErr)
		}
		return value.ExpandedName{}, false
	})
	err = builder.Complete(id, spec)
	if err == nil || err.Error() != `enumeration "p:x": unresolved QName` {
		t.Fatalf("Complete = %v; want QName resolution failure", err)
	}
	if _, available := builder.InputRequirements(id); available {
		t.Fatal("failed parent exposed metadata")
	}
	if _, validationErr := builder.Validate(child, "child survives", value.Resolver{}, 0, nil); validationErr != nil {
		t.Fatalf("child validation after parent failure = %v", validationErr)
	}
	if completionErr := builder.Complete(id, constructionQNameSpec(constructionQName)); completionErr != nil {
		t.Fatalf("parent retry = %v", completionErr)
	}
	if extra, reserveErr := builder.Reserve(); extra != value.NoType || !errors.Is(reserveErr, value.ErrLimit) {
		t.Fatalf("Reserve after retry = %v, %v; want storage limit", extra, reserveErr)
	}
	requireConstructionEnumeration(t, builder, id)
}

func TestBuilderCallbackPanicRefundPreservesChild(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1070})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	token := &struct{ label string }{"resolver panic"}
	child := value.NoType
	spec := constructionQNameSpec(func(string) (value.ExpandedName, bool) {
		var reserveErr error
		child, reserveErr = builder.Reserve()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		if completionErr := builder.Complete(child, atomicFacetType(value.PrimitiveString, value.FacetSpec{})); completionErr != nil {
			t.Fatal(completionErr)
		}
		panic(token)
	})
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		if completionErr := builder.Complete(id, spec); completionErr != nil {
			t.Fatalf("Complete returned instead of propagating the panic: %v", completionErr)
		}
	}()
	if recovered != token {
		t.Fatalf("recovered = %v; want original resolver panic", recovered)
	}
	if _, available := builder.TypeView(id); available {
		t.Fatal("panicking completion exposed parent metadata")
	}
	if _, validationErr := builder.Validate(child, "child survives", value.Resolver{}, 0, nil); validationErr != nil {
		t.Fatalf("child validation after parent panic = %v", validationErr)
	}
	if completionErr := builder.Complete(id, constructionQNameSpec(constructionQName)); completionErr != nil {
		t.Fatalf("parent retry after panic = %v", completionErr)
	}
	if extra, reserveErr := builder.Reserve(); extra != value.NoType || !errors.Is(reserveErr, value.ErrLimit) {
		t.Fatalf("Reserve after retry = %v, %v; want storage limit", extra, reserveErr)
	}
	requireConstructionEnumeration(t, builder, id)
}
