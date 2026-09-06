package xsd_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	streamingSchemaPrefix = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo>`
	streamingSchemaSuffix = `</xs:appinfo></xs:annotation><xs:element name="root" type="xs:int"/></xs:schema>`
	streamingComment      = `<!--x-->`
)

var errStreamingReadBufferTooLarge = errors.New("streaming source received an oversized read buffer")

// TestCompileOpenSourceUsesBoundedReadRequests makes the opener fail
// when its caller asks for a buffer larger than a bounded stream window. The
// reader itself synthesizes the document, so the test does not retain a copy
// of the source either.
func TestCompileOpenSourceUsesBoundedReadRequests(t *testing.T) {
	t.Parallel()

	const (
		schemaSize = 1 << 20
		maxRead    = 128 << 10
	)
	state := new(streamingReaderState)
	source := xsd.Open("streaming.xsd", func() (io.ReadCloser, error) {
		return io.NopCloser(newStreamingSchemaReader(schemaSize, maxRead, state)), nil
	})

	if _, err := xsd.Compile(source); err != nil {
		t.Fatalf("Compile() error = %v; source read window exceeded %d bytes", err, maxRead)
	}
	if state.oversized {
		t.Fatalf("source requested a read buffer larger than %d bytes", maxRead)
	}
	if state.maxRead == 0 {
		t.Fatal("source was never read")
	}
}

func TestCompileSourceAcquisitionErrorsPrecedeEarlyXMLParseErrors(t *testing.T) {
	t.Parallel()

	malformed := []byte(`<xs:schema><broken></xs:schema>`)
	readErr := errors.New("source read failed")
	closeErr := errors.New("source close failed")
	tests := []struct {
		name      string
		readErr   error
		closeErr  error
		wantCause error
		wantCode  xsderrors.Code
	}{
		{
			name:      "read error",
			readErr:   readErr,
			wantCause: readErr,
			wantCode:  xsderrors.CodeSchemaRead,
		},
		{
			name:      "close error",
			closeErr:  closeErr,
			wantCause: closeErr,
			wantCode:  xsderrors.CodeSchemaRead,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			reader := &stagedSchemaReadCloser{
				data:     malformed,
				readErr:  test.readErr,
				closeErr: test.closeErr,
			}
			_, err := xsd.Compile(xsd.Open("broken.xsd", func() (io.ReadCloser, error) {
				return reader, nil
			}))

			if err == nil {
				t.Fatal("Compile() succeeded for malformed schema")
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %v, want structured diagnostic", err)
			}
			if diagnostic.Code() != test.wantCode {
				t.Fatalf("Compile() code = %q, want %q; error = %v", diagnostic.Code(), test.wantCode, err)
			}
			if !errors.Is(err, test.wantCause) {
				t.Fatalf("Compile() error = %v, want cause %v", err, test.wantCause)
			}
			if !reader.closed {
				t.Fatal("Compile() did not close the source after the parser failed")
			}
		})
	}
}

func TestCompileSourceLimitPrecedesEarlyXMLParseError(t *testing.T) {
	t.Parallel()

	malformed := `<xs:schema><broken></xs:schema>`
	reader := &stagedSchemaReadCloser{
		data:    []byte(malformed + strings.Repeat("x", 1024)),
		readErr: io.EOF,
	}
	_, err := xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaSourceBytes: int64(len(malformed))},
		xsd.Open("limited.xsd", func() (io.ReadCloser, error) { return reader, nil }),
	)

	if err == nil {
		t.Fatal("CompileWithOptions() succeeded beyond the source byte limit")
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CompileWithOptions() error = %v, want structured diagnostic", err)
	}
	if diagnostic.Code() != xsderrors.CodeSchemaLimit {
		t.Fatalf("CompileWithOptions() code = %q, want %q; error = %v", diagnostic.Code(), xsderrors.CodeSchemaLimit, err)
	}
	if !reader.closed {
		t.Fatal("CompileWithOptions() did not close the limited source")
	}
}

func TestCompileRejectsDifferentRawContentForOneSourceIdentity(t *testing.T) {
	t.Parallel()

	const name = "shared.xsd"
	open := func(element string) xsd.SchemaSource {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="` + element + `" type="xs:string"/></xs:schema>`
		return xsd.Open(name, func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(schema)), nil
		})
	}

	_, err := xsd.Compile(open("first"), open("other"))
	if err == nil {
		t.Fatal("Compile() accepted different raw content for one source identity")
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("Compile() error = %v, want structured diagnostic", err)
	}
	if diagnostic.Code() != xsderrors.CodeSchemaReference {
		t.Fatalf("Compile() code = %q, want %q; error = %v", diagnostic.Code(), xsderrors.CodeSchemaReference, err)
	}
	if !strings.Contains(err.Error(), "different document content") {
		t.Fatalf("Compile() error = %v, want identity-content conflict", err)
	}
}

func TestCompileAcceptsIdenticalSeparateStreamsForOneSourceIdentity(t *testing.T) {
	t.Parallel()

	const (
		name   = "shared.xsd"
		schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`
	)
	openCalls := 0
	open := func() xsd.SchemaSource {
		return xsd.Open(name, func() (io.ReadCloser, error) {
			openCalls++
			return io.NopCloser(strings.NewReader(schema)), nil
		})
	}

	if _, err := xsd.Compile(open(), open()); err != nil {
		t.Fatalf("Compile() rejected identical separate streams: %v", err)
	}
	if openCalls != 2 {
		t.Fatalf("source opener calls = %d, want 2", openCalls)
	}
}

func BenchmarkCompileStreamingSource(b *testing.B) {
	for _, size := range []int{4 << 10, 1 << 20, 16 << 20} {
		b.Run(fmt.Sprintf("%dB", size), func(b *testing.B) {
			source := xsd.Open("streaming-benchmark.xsd", func() (io.ReadCloser, error) {
				return io.NopCloser(newStreamingSchemaReader(size, 0, nil)), nil
			})
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(source); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type streamingReaderState struct {
	maxRead   int
	oversized bool
}

type streamingSchemaReader struct {
	state       *streamingReaderState
	prefix      []byte
	suffix      []byte
	payloadSize int64
	commentSize int64
	payloadPos  int64
	position    int64
	total       int64
	maxRead     int
}

func newStreamingSchemaReader(size, maxRead int, state *streamingReaderState) *streamingSchemaReader {
	prefix := []byte(streamingSchemaPrefix)
	suffix := []byte(streamingSchemaSuffix)
	if size < len(prefix)+len(suffix) {
		panic("streaming schema size is smaller than its fixed syntax")
	}
	return &streamingSchemaReader{
		state:       state,
		prefix:      prefix,
		suffix:      suffix,
		payloadSize: int64(size - len(prefix) - len(suffix)),
		commentSize: int64(size-len(prefix)-len(suffix)) / int64(len(streamingComment)) * int64(len(streamingComment)),
		total:       int64(size),
		maxRead:     maxRead,
	}
}

func (r *streamingSchemaReader) Read(p []byte) (int, error) {
	if r.maxRead > 0 && len(p) > r.maxRead {
		if r.state != nil {
			r.state.oversized = true
		}
		return 0, errStreamingReadBufferTooLarge
	}
	if r.state != nil && len(p) > r.state.maxRead {
		r.state.maxRead = len(p)
	}
	if r.position >= r.total {
		return 0, io.EOF
	}

	read := 0
	for read < len(p) && r.position < r.total {
		switch {
		case r.position < int64(len(r.prefix)):
			n := copy(p[read:], r.prefix[r.position:])
			read += n
			r.position += int64(n)
		case r.position < int64(len(r.prefix))+r.payloadSize && r.payloadPos < r.commentSize:
			patternOffset := r.payloadPos % int64(len(streamingComment))
			patternRemaining := int64(len(streamingComment)) - patternOffset
			n := min(int64(len(p)-read), min(r.commentSize-r.payloadPos, patternRemaining))
			for i := range n {
				p[read+int(i)] = streamingComment[patternOffset+i]
			}
			read += int(n)
			r.position += n
			r.payloadPos += n
		case r.position < int64(len(r.prefix))+r.payloadSize:
			n := min(int64(len(p)-read), r.payloadSize-r.payloadPos)
			for i := range n {
				p[read+int(i)] = 'x'
			}
			read += int(n)
			r.position += n
			r.payloadPos += n
		default:
			n := copy(p[read:], r.suffix[r.position-int64(len(r.prefix))-r.payloadSize:])
			read += n
			r.position += int64(n)
		}
	}
	return read, nil
}

type stagedSchemaReadCloser struct {
	data     []byte
	readErr  error
	closeErr error
	closed   bool
}

func (r *stagedSchemaReadCloser) Read(p []byte) (int, error) {
	if len(r.data) != 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if r.readErr == nil {
		return 0, io.EOF
	}
	return 0, r.readErr
}

func (r *stagedSchemaReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}
