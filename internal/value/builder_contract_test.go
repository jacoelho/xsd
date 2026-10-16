package value_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuilderCompleteChargesFacetLiteralEvaluationWork(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{
		MaxConstructionWork: 1,
		MaxStorageBytes:     1024,
	})
	reservedID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatalf("Reserve() error = %v", reserveErr)
	}
	if err := builder.Complete(reservedID, value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{
			{Lexical: "x", Type: value.NoType},
		}},
	}); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Complete() error = %v, want ErrLimit", err)
	}
}

func TestBuilderReserveChargesTypeStorage(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1})
	if _, err := builder.Reserve(); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Reserve() error = %v, want ErrLimit", err)
	}
}

func TestBuilderCoreReferenceAdmission(t *testing.T) {
	tests := []struct {
		name    string
		variety value.Variety
		late    string
	}{
		{name: "atomic self base", variety: value.Atomic, late: "self"},
		{name: "list self item", variety: value.List, late: "self"},
		{name: "atomic unreserved base", variety: value.Atomic, late: "unreserved"},
		{name: "list unreserved item", variety: value.List, late: "unreserved"},
		{name: "deep union then absent", variety: value.Union, late: "absent"},
		{name: "deep union then unreserved", variety: value.Union, late: "unreserved"},
		{name: "deep union then incomplete", variety: value.Union, late: "incomplete"},
		{name: "deep union then self", variety: value.Union, late: "self"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{MaxDepth: 1})
			completed, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			atomic := value.TypeSpec{
				Variety: value.Atomic, Primitive: value.PrimitiveString,
				Base: value.NoType, ListItem: value.NoType,
			}
			if completionErr := builder.Complete(completed, atomic); completionErr != nil {
				t.Fatal(completionErr)
			}
			target, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			incomplete, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			ref := target
			switch test.late {
			case "absent":
				ref = value.NoType
			case "unreserved":
				ref = incomplete + 1
			case "incomplete":
				ref = incomplete
			}
			spec := value.TypeSpec{
				Variety: test.variety, Primitive: value.PrimitiveString,
				Base: value.NoType, ListItem: value.NoType,
			}
			switch test.variety {
			case value.Atomic:
				spec.Base = ref
			case value.List:
				spec.ListItem = ref
			case value.Union:
				spec.Union = []value.TypeID{completed, ref}
			}
			if err := builder.Complete(target, spec); !errors.Is(err, value.ErrMetadata) {
				t.Fatalf("Complete() error = %v, want ErrMetadata before any depth error", err)
			}
			if _, available := builder.InputRequirements(target); available {
				t.Fatal("rejected type became available")
			}
			if test.name == "atomic self base" {
				if err := builder.Complete(target, atomic); err != nil {
					t.Fatalf("Complete() retry error = %v", err)
				}
				got, err := builder.Validate(target, "x", value.Resolver{}, value.NeedCanonical, nil)
				if err != nil || got.CanonicalText() != "x" {
					t.Fatalf("Validate() after retry = %q, %v, want x, nil", got.CanonicalText(), err)
				}
			}
		})
	}
}

func TestBuilderCoreDepthPrecedesFacetReferenceAdmission(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxDepth: 1})
	base, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	decimal := atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{})
	if completionErr := builder.Complete(base, decimal); completionErr != nil {
		t.Fatal(completionErr)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	decimal.Base = base
	decimal.Facets.MinInclusive = value.BoundFacet{
		Present: true, Type: derived + 1, Lexical: "1",
	}
	if completionErr := builder.Complete(derived, decimal); !errors.Is(completionErr, value.ErrLimit) {
		t.Fatalf("Complete = %v; want core depth limit before invalid facet reference", completionErr)
	}
	if _, available := builder.TypeView(derived); available {
		t.Fatal("rejected type exposed metadata")
	}
}

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

func TestBuilderMetadataProvisionalFailureAndRetryContract(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	observed := false
	resolver := value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		observed = true
		view, available := builder.TypeView(id)
		if !available || view.Variety != value.Atomic || view.Primitive != value.PrimitiveQName {
			t.Fatalf("callback TypeView() = %#v, %t, want atomic QName", view, available)
		}
		if _, available := builder.InputRequirements(id); !available {
			t.Fatal("callback cannot query provisional input requirements")
		}
		return value.ExpandedName{}, false
	}}
	spec := atomicFacetType(value.PrimitiveQName, value.FacetSpec{
		Enumeration: []value.LiteralSpec{{Type: value.NoType, Lexical: "p:item", Resolver: resolver}},
	})
	err = builder.Complete(id, spec)
	if err == nil || err.Error() != `enumeration "p:item": unresolved QName` {
		t.Fatalf("Complete() = %v, want enumeration QName resolution failure", err)
	}
	if !observed {
		t.Fatal("resolver never observed the provisional type")
	}
	if got, available := builder.InputRequirements(id); available || got != (value.InputRequirements{}) {
		t.Fatalf("failed InputRequirements() = %#v, %t, want zero, false", got, available)
	}
	if got, available := builder.TypeView(id); available || !reflect.DeepEqual(got, value.TypeView{}) {
		t.Fatalf("failed TypeView() = %#v, %t, want zero, false", got, available)
	}
	resolved := value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		return value.ExpandedName{Namespace: "urn:test", Local: "item"}, true
	}}
	spec.Facets.Enumeration[0].Resolver = resolved
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatalf("Complete() retry = %v", completionErr)
	}
	if got, available := builder.InputRequirements(id); !available || !got.NeedsQName {
		t.Fatalf("retried InputRequirements() = %#v, %t, want QName requirement", got, available)
	}
	if got, available := builder.TypeView(id); !available || !got.HasEnumeration {
		t.Fatalf("retried TypeView() = %#v, %t, want enumeration", got, available)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(id, "p:item", resolved, 0, 16<<20, nil); err != nil {
		t.Fatalf("sealed retried QName validation = %v", err)
	}
}

func TestBuilderRejectsMissingFacetPayloads(t *testing.T) {
	for _, facet := range []value.FacetMask{value.FacetEnumeration, value.FacetPattern, value.FacetMinInclusive, value.FacetTotalDigits} {
		t.Run(fmt.Sprint(facet), func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.Complete(id, atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{Present: facet})); !errors.Is(err, value.ErrMetadata) {
				t.Fatalf("Complete missing facet payload = %v, want ErrMetadata", err)
			}
		})
	}
}
