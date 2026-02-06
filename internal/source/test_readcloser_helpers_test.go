package source

import (
	"fmt"
	"io"
)

type testReadCloser struct {
	reader io.Reader

	closeErr          error
	closed            bool
	closeCount        int
	failOnSecondClose bool
}

func (r *testReadCloser) Read(p []byte) (int, error) {
	if r.reader == nil {
		return 0, io.EOF
	}
	return r.reader.Read(p)
}

func (r *testReadCloser) Close() error {
	r.closeCount++
	if r.failOnSecondClose && r.closeCount > 1 {
		return fmt.Errorf("closed twice")
	}
	r.closed = true
	return r.closeErr
}
