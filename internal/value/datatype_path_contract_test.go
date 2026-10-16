package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuiltinPrimitiveMappings(t *testing.T) {
	for _, tc := range []struct {
		kind value.PrimitiveKind
		id   value.TypeID
	}{
		{value.PrimitiveString, 1},
		{value.PrimitiveBoolean, 7},
		{value.PrimitiveDecimal, 8},
		{value.PrimitiveFloat, 22},
		{value.PrimitiveDouble, 23},
		{value.PrimitiveDuration, 24},
		{value.PrimitiveDateTime, 26},
		{value.PrimitiveTime, 27},
		{value.PrimitiveDate, 25},
		{value.PrimitiveGYearMonth, 28},
		{value.PrimitiveGYear, 29},
		{value.PrimitiveGMonthDay, 30},
		{value.PrimitiveGDay, 31},
		{value.PrimitiveGMonth, 32},
		{value.PrimitiveAnyURI, 33},
		{value.PrimitiveHexBinary, 34},
		{value.PrimitiveBase64Binary, 35},
		{value.PrimitiveQName, 36},
		{value.PrimitiveNotation, 37},
		{value.PrimitiveKind(19), value.NoType},
		{value.PrimitiveKind(255), value.NoType},
	} {
		if got := value.BuiltinType(tc.kind); got != tc.id {
			t.Fatalf("BuiltinType(%d) = %d, want %d", tc.kind, got, tc.id)
		}
	}
}

func TestBorrowedRawListBudgetRouting(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	listID, found := value.BuiltinTypeID("NMTOKENS")
	if !found {
		t.Fatal("missing NMTOKENS")
	}
	unionID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatal(reserveErr)
	}
	if completeErr := builder.Complete(unionID, value.TypeSpec{
		Variety: value.Union, Union: []value.TypeID{listID, value.BuiltinType(value.PrimitiveString)},
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.NoType, ListItem: value.NoType,
	}); completeErr != nil {
		t.Fatal(completeErr)
	}
	program, sealErr := builder.Seal()
	if sealErr != nil {
		t.Fatal(sealErr)
	}
	projected, projectionErr := program.Validate(listID, "one", value.Resolver{}, value.NeedIdentity, 16<<20, nil)
	if projectionErr != nil {
		t.Fatal(projectionErr)
	}
	for _, tc := range []struct {
		name     string
		id       value.TypeID
		selected value.TypeID
		limit    uint64
		wantErr  error
	}{
		{"direct below", listID, value.NoType, 7, value.ErrLimit},
		{"direct at", listID, value.NoType, 8, nil},
		{"direct above", listID, value.NoType, 9, nil},
		{"union below", unionID, listID, 11, value.ErrLimit},
		{"union at", unionID, listID, 12, nil},
		{"union above", unionID, listID, 13, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte("one")
			got, validationErr := program.ValidateBytes(tc.id, raw, value.Resolver{}, 0, tc.limit, nil)
			clear(raw)
			if !errors.Is(validationErr, tc.wantErr) {
				t.Fatalf("budget %d error = %v, want %v", tc.limit, validationErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			if got.Type() != tc.id || got.SelectedType() != tc.selected {
				t.Fatalf("result type/selection = %d/%d, want %d/%d", got.Type(), got.SelectedType(), tc.id, tc.selected)
			}
			if got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
				t.Fatalf("unprojected result = %#v", got)
			}
			if got.Equal(projected) {
				t.Fatal("unprojected value compared equal to an identity-bearing value")
			}
		})
	}
}
