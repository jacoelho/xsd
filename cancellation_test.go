package xsd_test

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const cancellationTestSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`

type cancelingReader struct {
	cancel func(error)
	cause  error
	err    error
	data   string
	done   bool
}

func (r *cancelingReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	n := copy(p, r.data)
	r.cancel(r.cause)
	return n, r.err
}

type trackedReadCloser struct {
	io.Reader
	err    error
	closed bool
}

type dataAndErrorReader struct {
	data string
	err  error
	done bool
}

func (r *dataAndErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func (r *trackedReadCloser) Close() error {
	r.closed = true
	return r.err
}

func TestCompileCancellationDoesNotInvokeOpener(t *testing.T) {
	cause := errors.New("compile stopped")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	calls := 0
	_, err := xsd.Compile(ctx, xsd.Open("schema.xsd", func(context.Context) (io.ReadCloser, error) {
		calls++
		return io.NopCloser(strings.NewReader(cancellationTestSchema)), nil
	}))
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeCompileCanceled)
	if !errors.Is(err, cause) || calls != 0 {
		t.Fatalf("Compile() error = %v, opener calls = %d; want cause and no calls", err, calls)
	}
}

func TestCompileOpenerCancellationPreservesCallbackAndCloseErrors(t *testing.T) {
	cause := errors.New("compile stopped")
	openErr := errors.New("open failed")
	closeErr := errors.New("close failed")
	ctx, cancel := context.WithCancelCause(context.Background())
	reader := &trackedReadCloser{Reader: strings.NewReader(cancellationTestSchema), err: closeErr}
	_, err := xsd.Compile(ctx, xsd.Open("schema.xsd", func(got context.Context) (io.ReadCloser, error) {
		if got != ctx {
			t.Fatalf("Open() context differs from Compile context")
		}
		cancel(cause)
		return reader, openErr //nolint:nilnil // Verify cleanup and error preservation for both return values.
	}))
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeCompileCanceled)
	if !errors.Is(err, cause) || !errors.Is(err, openErr) || !errors.Is(err, closeErr) || !reader.closed {
		t.Fatalf("Compile() error = %v, closed = %v; want all causes and cleanup", err, reader.closed)
	}
}

func TestCompileResolverCancellationPreservesCallbackError(t *testing.T) {
	cause := errors.New("compile stopped")
	resolveErr := errors.New("resolver failed")
	ctx, cancel := context.WithCancelCause(context.Background())
	root := xsd.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`))
	root = root.WithResolver(xsd.ResolverFunc(func(got context.Context, _, _ string) (xsd.SchemaSource, error) {
		if got != ctx {
			t.Fatalf("Resolver context differs from Compile context")
		}
		cancel(cause)
		return xsd.Bytes("child.xsd", []byte(cancellationTestSchema)), resolveErr
	}))
	_, err := xsd.Compile(ctx, root)
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeCompileCanceled)
	if !errors.Is(err, cause) || !errors.Is(err, resolveErr) {
		t.Fatalf("Compile() error = %v, want cancellation and resolver causes", err)
	}
}

func TestValidateCancellationPreservesReaderErrorAndSessionRecovers(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("validation stopped")
	readErr := errors.New("read failed")
	ctx, cancel := context.WithCancelCause(context.Background())
	err = session.Validate(ctx, &cancelingReader{
		cancel: cancel,
		cause:  cause,
		err:    readErr,
		data:   `<r/>`,
	})
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeValidationCanceled)
	if !errors.Is(err, cause) || !errors.Is(err, readErr) {
		t.Fatalf("Session.Validate() error = %v, want cancellation and reader causes", err)
	}
	if err := session.Validate(context.Background(), strings.NewReader(`<r/>`)); err != nil {
		t.Fatalf("Session.Validate() after cancellation error = %v", err)
	}
}

func TestValidateCancellationDoesNotRead(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := 0
	err = engine.Validate(ctx, countingReader{read: func([]byte) (int, error) {
		reads++
		return 0, io.EOF
	}})
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeValidationCanceled)
	if !errors.Is(err, context.Canceled) || reads != 0 {
		t.Fatalf("Validate() error = %v, reads = %d; want cancellation without reads", err, reads)
	}
}

func TestValidateCancellationTakesPrecedenceOverSimultaneousByteLimit(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("validation stopped")
	ctx, cancel := context.WithCancelCause(context.Background())
	err = engine.ValidateWithOptions(ctx, &cancelingReader{
		cancel: cancel,
		cause:  cause,
		data:   `<r/>`,
	}, xsd.ValidateOptions{MaxInstanceBytes: 3})
	expectCategoryCode(t, err, xsderrors.CategoryCanceled, xsderrors.CodeValidationCanceled)
	if !errors.Is(err, cause) {
		t.Fatalf("Validate() error = %v, want cancellation cause", err)
	}
}

func TestMaxInstanceBytesBoundaryAndSessionRecovery(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `<r/>`
	if validationErr := engine.ValidateWithOptions(context.Background(), strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: math.MaxInt64}); validationErr != nil {
		t.Fatalf("Validate(MaxInt64 byte limit) error = %v", validationErr)
	}
	if validationErr := engine.ValidateWithOptions(context.Background(), strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}); validationErr != nil {
		t.Fatalf("Validate(exact byte limit) error = %v", validationErr)
	}
	err = engine.ValidateWithOptions(context.Background(), strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc) - 1)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)

	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(context.Background(), strings.NewReader(`<r> </r>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := session.Validate(context.Background(), strings.NewReader(doc)); err != nil {
		t.Fatalf("Session.Validate() after byte limit error = %v", err)
	}
}

func TestMaxInstanceBytesPreservesSimultaneousReaderError(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	doc := `<r/>`
	err = engine.ValidateWithOptions(context.Background(), &dataAndErrorReader{data: doc, err: sentinel}, xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc) - 1)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Validate() error = %v, want reader cause", err)
	}
}

func TestMaxInstanceBytesRejectsLimitJoinedWithEOFAndSessionRecovers(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `<r/>`
	if validationErr := engine.ValidateWithOptions(context.Background(), &dataAndErrorReader{data: doc, err: io.EOF}, xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}); validationErr != nil {
		t.Fatalf("Validate(exact data with EOF) error = %v", validationErr)
	}

	opts := xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}
	session, err := engine.NewSession(opts)
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(context.Background(), &dataAndErrorReader{data: doc + "X", err: io.EOF})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Session.Validate() error = %v, want EOF reader cause", err)
	}
	if err := session.Validate(context.Background(), strings.NewReader(doc)); err != nil {
		t.Fatalf("Session.Validate() after joined limit/EOF error = %v", err)
	}
}

func TestValidationPreservesJoinedReaderErrorAfterBareCR(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	err = session.Validate(context.Background(), &dataAndErrorReader{data: `<r/>\r`, err: errors.Join(io.EOF, sentinel)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Session.Validate() error = %v, want reader cause", err)
	}
	if err := session.Validate(context.Background(), strings.NewReader(`<r/>`)); err != nil {
		t.Fatalf("Session.Validate() after reader error = %v", err)
	}
}

func TestValidationPreflightReaderErrorIsStructured(t *testing.T) {
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(cancellationTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	err = engine.Validate(context.Background(), &dataAndErrorReader{data: `<r`, err: errors.Join(io.EOF, sentinel)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if !errors.Is(err, sentinel) || !errors.Is(err, io.EOF) {
		t.Fatalf("Validate() error = %v, want joined reader causes", err)
	}
}
