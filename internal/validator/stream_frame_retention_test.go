package validator

import "testing"

func TestReleaseRemainingFramesClearsBuffers(t *testing.T) {
	run := &streamRun{}
	run.frames = []streamFrame{{
		textBuf:       make([]byte, 1024),
		fieldCaptures: make([]fieldCapture, 1),
		listStream: listStreamState{
			itemBuf:      make([]byte, 512),
			collapsedBuf: make([]byte, 256),
		},
	}}

	frame := &run.frames[0]
	run.releaseRemainingFrames()

	if len(run.frames) != 0 {
		t.Fatalf("expected frames to be empty, got %d", len(run.frames))
	}
	if frame.textBuf != nil {
		t.Fatalf("expected textBuf to be cleared")
	}
	if frame.fieldCaptures != nil {
		t.Fatalf("expected fieldCaptures to be cleared")
	}
	if frame.listStream.itemBuf != nil || frame.listStream.collapsedBuf != nil {
		t.Fatalf("expected listStream buffers to be cleared")
	}
}
