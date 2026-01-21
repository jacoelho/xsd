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

func TestPopFrameClearsBuffers(t *testing.T) {
	run := &streamRun{}
	run.frames = make([]streamFrame, 1)
	run.frames[0] = streamFrame{
		textBuf:       make([]byte, 256),
		fieldCaptures: make([]fieldCapture, 1),
		listStream: listStreamState{
			itemBuf:      make([]byte, 128),
			collapsedBuf: make([]byte, 64),
		},
	}

	frame := run.popFrame()
	if frame == nil {
		t.Fatalf("expected frame to be popped")
	}
	if len(run.frames) != 0 {
		t.Fatalf("expected frames to be empty, got %d", len(run.frames))
	}
	if cap(run.frames) == 0 {
		t.Fatalf("expected frame storage to remain allocated")
	}
	slot := run.frames[:1][0]
	if slot.textBuf != nil {
		t.Fatalf("expected popped frame textBuf to be cleared")
	}
	if slot.fieldCaptures != nil {
		t.Fatalf("expected popped frame fieldCaptures to be cleared")
	}
	if slot.listStream.itemBuf != nil || slot.listStream.collapsedBuf != nil {
		t.Fatalf("expected popped frame listStream buffers to be cleared")
	}
}
