package model

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

type parserRegistryTestValue struct {
	lexical string
}

func (v parserRegistryTestValue) Type() Type { return nil }

func (v parserRegistryTestValue) Lexical() string { return v.lexical }

func (v parserRegistryTestValue) Native() any { return v.lexical }

func (v parserRegistryTestValue) String() string { return v.lexical }

func TestParseValueForTypeDispatchesParser(t *testing.T) {
	t.Parallel()

	got, err := parseValueForTypeWithParsers("v", TypeNameString, nil, map[TypeName]ValueParserFunc{
		TypeNameString: func(lexical string, _ Type) (TypedValue, error) {
			return parserRegistryTestValue{lexical: lexical + "-ok"}, nil
		},
	})
	if err != nil {
		t.Fatalf("parseValueForTypeWithParsers() error = %v", err)
	}
	if got.String() != "v-ok" {
		t.Fatalf("parseValueForTypeWithParsers() = %q, want %q", got.String(), "v-ok")
	}
}

func TestParseValueForTypeUnknownType(t *testing.T) {
	t.Parallel()

	_, err := parseValueForTypeWithParsers("v", TypeName("unknown"), nil, map[TypeName]ValueParserFunc{})
	if err == nil {
		t.Fatalf("expected error for unknown parser")
	}
	if !strings.Contains(err.Error(), "no parser for type unknown") {
		t.Fatalf("parseValueForTypeWithParsers() error = %v", err)
	}
}

func TestParseValueForTypePropagatesParserError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("parse failed")
	_, err := parseValueForTypeWithParsers("v", TypeNameString, nil, map[TypeName]ValueParserFunc{
		TypeNameString: func(string, Type) (TypedValue, error) {
			return nil, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("parseValueForTypeWithParsers() error = %v, want %v", err, wantErr)
	}
}

func TestBuiltinParseValueConcurrent(t *testing.T) {
	bt := GetBuiltin(TypeNameString)
	if bt == nil {
		t.Fatal("GetBuiltin(string) returned nil")
	}

	const workers = 32
	errCh := make(chan error, workers)
	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			if _, err := bt.ParseValue("hello"); err != nil {
				errCh <- err
			}
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("ParseValue error: %v", err)
	}
}
