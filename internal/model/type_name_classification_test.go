package model

import "testing"

func TestIsIntegerTypeName(t *testing.T) {
	t.Parallel()

	integerTypeNames := []string{
		"integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	for _, typeName := range integerTypeNames {
		t.Run(typeName, func(t *testing.T) {
			t.Parallel()
			if !IsIntegerTypeName(typeName) {
				t.Fatalf("IsIntegerTypeName(%q) = false, want true", typeName)
			}
		})
	}

	nonIntegerTypeNames := []string{"decimal", "float", "double", "string", "date"}
	for _, typeName := range nonIntegerTypeNames {
		t.Run("not_"+typeName, func(t *testing.T) {
			t.Parallel()
			if IsIntegerTypeName(typeName) {
				t.Fatalf("IsIntegerTypeName(%q) = true, want false", typeName)
			}
		})
	}
}

func TestIsNumericTypeName(t *testing.T) {
	t.Parallel()

	numericTypeNames := []string{
		"decimal", "float", "double",
		"integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	for _, typeName := range numericTypeNames {
		t.Run(typeName, func(t *testing.T) {
			t.Parallel()
			if !IsNumericTypeName(typeName) {
				t.Fatalf("IsNumericTypeName(%q) = false, want true", typeName)
			}
		})
	}

	nonNumericTypeNames := []string{"string", "date", "duration", "boolean"}
	for _, typeName := range nonNumericTypeNames {
		t.Run("not_"+typeName, func(t *testing.T) {
			t.Parallel()
			if IsNumericTypeName(typeName) {
				t.Fatalf("IsNumericTypeName(%q) = true, want false", typeName)
			}
		})
	}
}
