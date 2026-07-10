package stream

import (
	"bufio"
	"strings"
	"testing"
)

func TestPrepareXMLReaderWithBufferOwnsReturnedReader(t *testing.T) {
	callerReader := bufio.NewReaderSize(strings.NewReader("<root/>"), xmlReaderBufferSize*2)

	reader, err := PrepareXMLReaderWithBuffer(callerReader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reader == callerReader {
		t.Fatal("PrepareXMLReaderWithBuffer() returned the caller-owned buffered reader")
	}
}
