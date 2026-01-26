package xsd_test

import (
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/errors"
)

func TestEngineValidateFS(t *testing.T) {
	fsys := fstest.MapFS{
		"simple.xsd": &fstest.MapFile{Data: []byte(testSchema)},
	}

	engine, err := xsd.CompileFS(fsys, "simple.xsd")
	if err != nil {
		t.Fatalf("CompileFS() error = %v", err)
	}

	if err := engine.Validate(strings.NewReader(validPersonXML)); err != nil {
		t.Fatalf("Engine.Validate() err = %v, want nil", err)
	}
}

func TestEngineValidateNilEngine(t *testing.T) {
	var engine *xsd.Engine

	requireSingleViolation(t, engine.Validate(strings.NewReader("<root/>")), errors.ErrSchemaNotLoaded)
}

func TestEngineValidateNilReader(t *testing.T) {
	fsys := fstest.MapFS{
		"simple.xsd": &fstest.MapFile{Data: []byte(testSchema)},
	}

	engine, err := xsd.CompileFS(fsys, "simple.xsd")
	if err != nil {
		t.Fatalf("CompileFS() error = %v", err)
	}

	requireSingleViolation(t, engine.Validate(nil), errors.ErrXMLParse)
}

func TestCompileSchemaReader(t *testing.T) {
	engine, err := xsd.CompileSchema(strings.NewReader(testSchema))
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	if err := engine.Validate(strings.NewReader(validPersonXML)); err != nil {
		t.Fatalf("Engine.Validate() err = %v, want nil", err)
	}
}

func TestSessionValidate(t *testing.T) {
	engine, err := xsd.CompileSchema(strings.NewReader(testSchema))
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	session := engine.NewSession()
	if session == nil {
		t.Fatal("NewSession() returned nil")
	}
	if err := session.Validate(strings.NewReader(validPersonXML)); err != nil {
		t.Fatalf("Session.Validate() err = %v, want nil", err)
	}
}

func TestSessionConcurrency(t *testing.T) {
	engine, err := xsd.CompileSchema(strings.NewReader(testSchema))
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	const goroutines = 16
	const iterations = 25
	errCh := make(chan error, goroutines*iterations)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if err := engine.Validate(strings.NewReader(validPersonXML)); err != nil {
					errCh <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Engine.Validate() err = %v, want nil", err)
		}
	}
}
