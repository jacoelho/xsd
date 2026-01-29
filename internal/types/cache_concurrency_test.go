package types

import (
	"sync"
	"testing"
)

func TestTypeCacheConcurrency(t *testing.T) {
	builtin := GetBuiltin(TypeNameInteger)
	if builtin == nil {
		t.Fatal("expected builtin integer type")
	}

	simple := &SimpleType{
		QName: QName{Namespace: "http://example.com", Local: "MyInt"},
		Restriction: &Restriction{
			Base: QName{Namespace: XSDNamespace, Local: string(TypeNameInt)},
		},
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers * 3)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_ = builtin.PrimitiveType()
		}()
		go func() {
			defer wg.Done()
			_ = builtin.FundamentalFacets()
		}()
		go func() {
			defer wg.Done()
			_ = simple.PrimitiveType()
			_ = simple.FundamentalFacets()
			_ = IdentityNormalizable(simple)
		}()
	}
	wg.Wait()

	if builtin.PrimitiveType() == nil {
		t.Fatal("PrimitiveType() returned nil for builtin integer")
	}
	if builtin.FundamentalFacets() == nil {
		t.Fatal("FundamentalFacets() returned nil for builtin integer")
	}
	if simple.PrimitiveType() == nil {
		t.Fatal("PrimitiveType() returned nil for simple type")
	}
	if simple.FundamentalFacets() == nil {
		t.Fatal("FundamentalFacets() returned nil for simple type")
	}
	if !IdentityNormalizable(simple) {
		t.Fatal("IdentityNormalizable() returned false for atomic simple type")
	}
}
