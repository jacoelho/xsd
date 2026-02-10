package value

import "testing"

func TestUpperHex(t *testing.T) {
	got := UpperHex(nil, []byte{0x00, 0xab, 0xcd, 0xef})
	if string(got) != "00ABCDEF" {
		t.Fatalf("UpperHex() = %q, want %q", got, "00ABCDEF")
	}
}

func TestUpperHexReusesDestinationBuffer(t *testing.T) {
	backing := make([]byte, 16)
	dst := backing[:0]
	got := UpperHex(dst, []byte{0xde, 0xad})
	if string(got) != "DEAD" {
		t.Fatalf("UpperHex() = %q, want %q", got, "DEAD")
	}
	if len(got) > 0 && &got[0] != &backing[0] {
		t.Fatalf("UpperHex() did not reuse destination buffer")
	}
}
