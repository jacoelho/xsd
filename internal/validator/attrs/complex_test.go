package attrs

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func TestValidateComplexUsesDeclaredAttribute(t *testing.T) {
	t.Parallel()

	rt := &runtime.Schema{
		AttrIndex: runtime.ComplexAttrIndex{
			Uses: []runtime.AttrUse{{Name: 11, Validator: 23}},
		},
	}
	ct := &runtime.ComplexType{Attrs: runtime.AttrIndexRef{Off: 0, Len: 1}}
	present := make([]bool, 1)
	attr := Start{Sym: 11, Value: []byte("v")}

	called := false
	validated, seenID, err := ValidateComplex(
		rt,
		ct,
		present,
		[]Start{attr},
		[]Class{ClassOther},
		true,
		nil,
		ComplexCallbacks{
			AppendRaw: func(validated []Start, attr Start, storeAttrs bool) []Start {
				t.Fatal("AppendRaw should not be called for a declared attribute")
				return nil
			},
			ValidateUse: func(validated []Start, attr Start, use runtime.AttrUse, storeAttrs bool, seenID *bool) ([]Start, error) {
				called = true
				*seenID = true
				if use.Name != 11 || use.Validator != 23 || !storeAttrs {
					t.Fatalf("unexpected use = %+v, storeAttrs = %v", use, storeAttrs)
				}
				return append(validated, attr), nil
			},
			ValidateWildcard: func([]Start, Start, runtime.WildcardID, bool, *bool) ([]Start, error) {
				t.Fatal("ValidateWildcard should not be called for a declared attribute")
				return nil, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateComplex() error = %v", err)
	}
	if !called {
		t.Fatal("ValidateUse was not called")
	}
	if !seenID {
		t.Fatal("seenID = false, want true")
	}
	if !present[0] {
		t.Fatal("present[0] = false, want true")
	}
	if len(validated) != 1 || validated[0].Sym != 11 {
		t.Fatalf("validated = %#v, want one declared attribute", validated)
	}
}

func TestValidateComplexProhibitedAttribute(t *testing.T) {
	t.Parallel()

	rt := &runtime.Schema{
		AttrIndex: runtime.ComplexAttrIndex{
			Uses: []runtime.AttrUse{{Name: 11, Use: runtime.AttrProhibited}},
		},
	}
	ct := &runtime.ComplexType{Attrs: runtime.AttrIndexRef{Off: 0, Len: 1}}

	_, _, err := ValidateComplex(
		rt,
		ct,
		nil,
		[]Start{{Sym: 11}},
		[]Class{ClassOther},
		false,
		nil,
		ComplexCallbacks{
			AppendRaw:        func(validated []Start, attr Start, storeAttrs bool) []Start { return validated },
			ValidateUse:      func([]Start, Start, runtime.AttrUse, bool, *bool) ([]Start, error) { return nil, nil },
			ValidateWildcard: func([]Start, Start, runtime.WildcardID, bool, *bool) ([]Start, error) { return nil, nil },
		},
	)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrAttributeProhibited {
		t.Fatalf("ValidateComplex() error = %v, want %s", err, xsderrors.ErrAttributeProhibited)
	}
}

func TestValidateComplexUsesWildcardFallback(t *testing.T) {
	t.Parallel()

	ct := &runtime.ComplexType{AnyAttr: 7}
	attr := Start{Sym: 99}

	var got runtime.WildcardID
	validated, seenID, err := ValidateComplex(
		&runtime.Schema{},
		ct,
		nil,
		[]Start{attr},
		[]Class{ClassOther},
		false,
		nil,
		ComplexCallbacks{
			AppendRaw:   func(validated []Start, attr Start, storeAttrs bool) []Start { return validated },
			ValidateUse: func([]Start, Start, runtime.AttrUse, bool, *bool) ([]Start, error) { return nil, nil },
			ValidateWildcard: func(validated []Start, attr Start, wildcard runtime.WildcardID, storeAttrs bool, seenID *bool) ([]Start, error) {
				got = wildcard
				*seenID = true
				return append(validated, attr), nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateComplex() error = %v", err)
	}
	if got != 7 {
		t.Fatalf("wildcard = %d, want 7", got)
	}
	if !seenID {
		t.Fatal("seenID = false, want true")
	}
	if len(validated) != 1 || validated[0].Sym != 99 {
		t.Fatalf("validated = %#v, want wildcard attribute", validated)
	}
}

func TestValidateComplexRejectsUnknownAttributeWithoutWildcard(t *testing.T) {
	t.Parallel()

	_, _, err := ValidateComplex(
		&runtime.Schema{},
		&runtime.ComplexType{},
		nil,
		[]Start{{Sym: 44}},
		[]Class{ClassOther},
		false,
		nil,
		ComplexCallbacks{
			AppendRaw:        func(validated []Start, attr Start, storeAttrs bool) []Start { return validated },
			ValidateUse:      func([]Start, Start, runtime.AttrUse, bool, *bool) ([]Start, error) { return nil, nil },
			ValidateWildcard: func([]Start, Start, runtime.WildcardID, bool, *bool) ([]Start, error) { return nil, nil },
		},
	)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrAttributeNotDeclared {
		t.Fatalf("ValidateComplex() error = %v, want %s", err, xsderrors.ErrAttributeNotDeclared)
	}
}

func TestValidateComplexPreservesKnownXSIAndXMLAttrs(t *testing.T) {
	t.Parallel()

	var got []Start
	validated, seenID, err := ValidateComplex(
		&runtime.Schema{},
		&runtime.ComplexType{},
		nil,
		[]Start{{Sym: 1}, {Sym: 2}},
		[]Class{ClassXSIKnown, ClassXML},
		true,
		nil,
		ComplexCallbacks{
			AppendRaw: func(validated []Start, attr Start, storeAttrs bool) []Start {
				got = append(got, attr)
				return append(validated, attr)
			},
			ValidateUse: func([]Start, Start, runtime.AttrUse, bool, *bool) ([]Start, error) {
				t.Fatal("ValidateUse should not be called for xsi/xml attrs")
				return nil, nil
			},
			ValidateWildcard: func([]Start, Start, runtime.WildcardID, bool, *bool) ([]Start, error) {
				t.Fatal("ValidateWildcard should not be called for xsi/xml attrs")
				return nil, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateComplex() error = %v", err)
	}
	if seenID {
		t.Fatal("seenID = true, want false")
	}
	if len(validated) != 2 || len(got) != 2 {
		t.Fatalf("validated = %#v, got = %#v, want both raw attrs preserved", validated, got)
	}
}
