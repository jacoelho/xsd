package types

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestTypeCacheConcurrency(t *testing.T) {
	const (
		iterations = 200
		workers    = 16
		timeout    = 3 * time.Second
	)

	result := make(chan string, 1)
	go func() {
		for i := 0; i < iterations; i++ {
			builtin := newBuiltin(TypeNameString, validateString, nil, WhiteSpacePreserve, unordered)
			simple := &SimpleType{
				QName: QName{Namespace: "http://example.com", Local: "MyInt"},
				Restriction: &Restriction{
					Base: QName{Namespace: XSDNamespace, Local: string(TypeNameInt)},
				},
			}

			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(workers * 4)
			for j := 0; j < workers; j++ {
				go func() {
					defer wg.Done()
					<-start
					_ = builtin.PrimitiveType()
				}()
				go func() {
					defer wg.Done()
					<-start
					_ = builtin.FundamentalFacets()
				}()
				go func() {
					defer wg.Done()
					<-start
					_ = simple.PrimitiveType()
				}()
				go func() {
					defer wg.Done()
					<-start
					_ = simple.FundamentalFacets()
				}()
			}
			close(start)
			wg.Wait()

			if builtin.PrimitiveType() == nil {
				result <- "PrimitiveType() returned nil for builtin type in iteration " + strconv.Itoa(i)
				return
			}
			if builtin.FundamentalFacets() == nil {
				result <- "FundamentalFacets() returned nil for builtin type in iteration " + strconv.Itoa(i)
				return
			}
			if simple.PrimitiveType() == nil {
				result <- "PrimitiveType() returned nil for simple type in iteration " + strconv.Itoa(i)
				return
			}
			if simple.FundamentalFacets() == nil {
				result <- "FundamentalFacets() returned nil for simple type in iteration " + strconv.Itoa(i)
				return
			}
		}
		result <- ""
	}()

	select {
	case msg := <-result:
		if msg != "" {
			t.Fatal(msg)
		}
	case <-time.After(timeout):
		t.Fatalf("type cache concurrency test timed out after %s", timeout)
	}
}
