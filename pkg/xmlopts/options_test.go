package xmlopts

import "testing"

type fooOption int

type barOption string

func Foo(value int) Options {
	return New(fooOption(value))
}

func Bar(value string) Options {
	return New(barOption(value))
}

func TestJoinAndGetOption(t *testing.T) {
	opts := JoinOptions(Foo(1), Foo(2), Bar("a"))
	gotFoo, ok := GetOption(opts, Foo)
	if !ok {
		t.Fatalf("GetOption Foo ok = false, want true")
	}
	if gotFoo != 2 {
		t.Fatalf("GetOption Foo = %d, want 2", gotFoo)
	}
	gotBar, ok := GetOption(opts, Bar)
	if !ok {
		t.Fatalf("GetOption Bar ok = false, want true")
	}
	if gotBar != "a" {
		t.Fatalf("GetOption Bar = %q, want a", gotBar)
	}
	if _, ok := GetOption(opts, func(int64) Options { return New(int64(0)) }); ok {
		t.Fatalf("GetOption unknown ok = true, want false")
	}
}
