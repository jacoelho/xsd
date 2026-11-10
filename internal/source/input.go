package source

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"math"

	"github.com/jacoelho/xsd/xsderrors"
)

const sourceInputBufferSize = 32 << 10

// Input is one bounded, forward-only view of a schema source. It retains
// reader state and digest state, never the source bytes. The owner must use
// the pointer linearly and must not copy or use it concurrently. The owner
// ends the input with exactly one of Finish or Discard. Discard is only for an
// input whose content and read failures are irrelevant because a limit has
// already ended the load; it is not a way around the Finish precedence rule
// for parse failures.
type Input struct {
	stream       io.ReadCloser
	digest       hash.Hash
	readErr      error
	name         string
	staticReader bytes.Reader
	final        ReadResult

	static      [sha256.Size]byte
	max         int64
	staticBytes int64
	bytes       int64
	emptyReads  int

	hasStaticDigest bool
	limitExceeded   bool
	phase           inputPhase
}

// inputPhase is the Input lifecycle. Legal progress:
//
//	invalid -> failed            first Read or Finish of a zero Input
//	reading -> ended | failed    end of input, or read error, no progress, limit
//	any unfinished -> finished   Finish or Discard
//
// Entering failed sets readErr. seal is the only entry to finished and
// freezes final, which every later Finish returns.
type inputPhase uint8

const (
	// inputInvalid is the zero value: an Input not built by OpenInput. Its
	// first use fails it.
	inputInvalid inputPhase = iota
	// inputReading may yield more bytes.
	inputReading
	// inputEnded saw end of input within the bound.
	inputEnded
	// inputFailed saw a read error, no progress, or the byte limit.
	inputFailed
	// inputFinished closed and detached the source; final is frozen.
	inputFinished
)

func newInput(name string, reader io.ReadCloser, maxBytes int64) *Input {
	return &Input{
		stream: reader,
		name:   name,
		max:    maxBytes,
		digest: sha256.New(),
		phase:  inputReading,
	}
}

func newBytesInput(name string, data []byte, digest [sha256.Size]byte, maxBytes int64) *Input {
	input := &Input{
		name:            name,
		max:             maxBytes,
		static:          digest,
		staticBytes:     int64(len(data)),
		hasStaticDigest: true,
		phase:           inputReading,
	}
	input.staticReader.Reset(data)
	return input
}

// Read implements io.Reader. It exposes at most the first maxBytes bytes plus
// one probe byte so callers can distinguish an exact boundary from overflow.
func (in *Input) Read(p []byte) (int, error) {
	if in == nil {
		return 0, io.ErrClosedPipe
	}
	switch in.phase {
	case inputFinished:
		return 0, io.EOF
	case inputInvalid:
		in.fail(invalidInputError())
		return 0, in.readErr
	case inputReading, inputEnded, inputFailed:
	}
	if len(p) == 0 {
		return 0, nil
	}
	switch in.phase {
	case inputFailed:
		return 0, in.readErr
	case inputEnded:
		return 0, io.EOF
	case inputInvalid, inputReading, inputFinished:
	}

	p = in.bound(p)
	n, err := in.readChunk(p)
	if n > 0 {
		in.record(p[:n])
	}
	return n, in.readResult(err)
}

// Finish drains unread input within the same bound, closes the source once,
// detaches it, and returns a stable result on every subsequent call.
func (in *Input) Finish() ReadResult {
	if in == nil {
		return ReadResult{Err: errors.New("nil schema input")}
	}
	switch in.phase {
	case inputFinished:
		return in.final
	case inputInvalid:
		in.fail(invalidInputError())
	case inputReading:
		if in.hasStaticDigest {
			// Byte-backed sources are immutable and were checked against max at
			// open time. Their complete identity is already known, so a partial
			// parser read needs no drain or allocation.
			in.bytes = in.staticBytes
			in.phase = inputEnded
		} else {
			in.drain()
		}
	case inputEnded, inputFailed:
	}
	return in.seal(in.closeSource())
}

// Discard closes the source without reading further, detaches it, and returns
// the close error. It ends an input whose content is not needed. The input is
// then finished: Read returns io.EOF and Finish returns the same terminal
// result, whose Bytes and Digest cover only what was consumed and are not a
// content identity. Discard after Finish or Discard returns nil.
func (in *Input) Discard() error {
	if in == nil || in.phase == inputFinished {
		return nil
	}
	closeErr := in.closeSource()
	in.seal(closeErr)
	return closeErr
}

// seal is the only transition to inputFinished: it detaches the source and
// freezes the result, including any close failure.
func (in *Input) seal(closeErr error) ReadResult {
	in.stream = nil
	if in.hasStaticDigest {
		in.staticReader.Reset(nil)
	}
	in.final = in.result(closeErr)
	in.phase = inputFinished
	return in.final
}

func (in *Input) fail(err error) {
	in.readErr = err
	in.phase = inputFailed
}

func (in *Input) drain() {
	buf := make([]byte, sourceInputBufferSize)
	for in.phase == inputReading {
		if _, err := in.Read(buf); err != nil {
			return
		}
	}
}

func (in *Input) closeSource() error {
	if in.stream == nil {
		return nil
	}
	return in.stream.Close()
}

func (in *Input) bound(p []byte) []byte {
	if in.max == math.MaxInt64 {
		return p
	}
	remaining := in.max - in.bytes
	if remaining < 0 {
		return p[:0]
	}
	// Permit one byte beyond the configured bound as the overflow probe.
	allowed := remaining + 1
	if allowed < int64(len(p)) {
		p = p[:allowed]
	}
	return p
}

func (in *Input) readChunk(p []byte) (int, error) {
	var n int
	var err error
	if in.hasStaticDigest {
		n, err = in.staticReader.Read(p)
	} else {
		n, err = in.stream.Read(p)
	}
	if n < 0 || n > len(p) {
		return 0, io.ErrShortBuffer
	}
	if n != 0 || err != nil {
		in.emptyReads = 0
		return n, err
	}
	in.emptyReads++
	if in.emptyReads >= maxConsecutiveEmptySchemaReads {
		return 0, io.ErrNoProgress
	}
	return 0, nil
}

func (in *Input) readResult(err error) error {
	if in.overLimit() {
		limitErr := schemaSourceLimitError(in.name)
		in.limitExceeded = true
		if err != nil && err != io.EOF { //nolint:errorlint // Only bare EOF is end of input.
			limitErr = errors.Join(limitErr, err)
		}
		in.fail(limitErr)
		return limitErr
	}
	if err == io.EOF { //nolint:errorlint // Wrapped EOF is a source read failure; match io.ReadAll.
		// Returning EOF with data is legal, and lets callers stop without
		// another read while Finish still observes ordinary EOF.
		in.phase = inputEnded
		return io.EOF
	}
	if err != nil {
		in.fail(err)
	}
	return err
}

func (in *Input) record(data []byte) {
	n := int64(len(data))
	if n > math.MaxInt64-in.bytes {
		in.bytes = math.MaxInt64
	} else {
		in.bytes += n
	}
	if !in.hasStaticDigest {
		_, _ = in.digest.Write(data)
	}
}

func (in *Input) overLimit() bool {
	return in.max != math.MaxInt64 && in.bytes > in.max
}

func (in *Input) result(closeErr error) ReadResult {
	result := ReadResult{
		Err:           in.readErr,
		Bytes:         in.bytes,
		Digest:        in.digestSum(),
		LimitExceeded: in.limitExceeded,
	}
	if closeErr != nil {
		if result.Err == nil {
			result.Err = closeErr
		} else {
			result.Err = errors.Join(result.Err, closeErr)
		}
	}
	return result
}

func (in *Input) digestSum() [sha256.Size]byte {
	if in.hasStaticDigest {
		return in.static
	}
	var result [sha256.Size]byte
	if in.digest == nil {
		return result
	}
	copy(result[:], in.digest.Sum(result[:0]))
	return result
}

func invalidInputError() error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema input is invalid")
}
