package validator

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestResolveWildcardSymbol(t *testing.T) {
	strictErr := errors.New("strict unresolved")
	tests := []struct {
		name        string
		pc          runtime.ProcessContents
		sym         runtime.SymbolID
		resolveOK   bool
		wantResolve bool
		wantErr     error
	}{
		{
			name:        "skip ignores resolution",
			pc:          runtime.PCSkip,
			sym:         7,
			resolveOK:   true,
			wantResolve: false,
		},
		{
			name:        "lax unresolved symbol",
			pc:          runtime.PCLax,
			sym:         0,
			resolveOK:   false,
			wantResolve: false,
		},
		{
			name:        "lax missing declaration",
			pc:          runtime.PCLax,
			sym:         9,
			resolveOK:   false,
			wantResolve: false,
		},
		{
			name:        "strict unresolved symbol",
			pc:          runtime.PCStrict,
			sym:         0,
			resolveOK:   false,
			wantResolve: false,
			wantErr:     strictErr,
		},
		{
			name:        "strict missing declaration",
			pc:          runtime.PCStrict,
			sym:         4,
			resolveOK:   false,
			wantResolve: false,
			wantErr:     strictErr,
		},
		{
			name:        "lax resolved declaration",
			pc:          runtime.PCLax,
			sym:         3,
			resolveOK:   true,
			wantResolve: true,
		},
		{
			name:        "strict resolved declaration",
			pc:          runtime.PCStrict,
			sym:         3,
			resolveOK:   true,
			wantResolve: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveCalls := 0
			resolved, err := resolveWildcardSymbol(tt.pc, tt.sym, func(runtime.SymbolID) bool {
				resolveCalls++
				return tt.resolveOK
			}, func() error {
				return strictErr
			})
			if resolved != tt.wantResolve {
				t.Fatalf("resolveWildcardSymbol() resolved = %v, want %v", resolved, tt.wantResolve)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("resolveWildcardSymbol() unexpected error: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolveWildcardSymbol() error = %v, want %v", err, tt.wantErr)
			}
			if tt.pc == runtime.PCSkip && resolveCalls != 0 {
				t.Fatalf("resolve function called for skip mode")
			}
		})
	}
}

func TestResolveWildcardSymbolUnknownProcessContents(t *testing.T) {
	_, err := resolveWildcardSymbol(runtime.ProcessContents(255), 1, func(runtime.SymbolID) bool {
		return true
	}, nil)
	if err == nil {
		t.Fatalf("expected unknown processContents error")
	}
}
