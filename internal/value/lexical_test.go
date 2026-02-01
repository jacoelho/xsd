package value

import "testing"

func TestValidateNameAllowsColon(t *testing.T) {
	if err := ValidateName([]byte("a:b")); err != nil {
		t.Fatalf("ValidateName(a:b) error = %v", err)
	}
	if err := ValidateName([]byte(":name")); err != nil {
		t.Fatalf("ValidateName(:name) error = %v", err)
	}
}

func TestValidateNMTOKENAllowsColon(t *testing.T) {
	if err := ValidateNMTOKEN([]byte("a:b")); err != nil {
		t.Fatalf("ValidateNMTOKEN(a:b) error = %v", err)
	}
}

func TestValidateNCNameRejectsColon(t *testing.T) {
	if err := ValidateNCName([]byte("a:b")); err == nil {
		t.Fatalf("expected NCName to reject colon")
	}
}
