package validate

import (
	"errors"
	"sync"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
)

type sessionXMLTestState struct {
	input testXMLInput
	ready bool
}

var sessionXMLTestStates struct {
	mu sync.Mutex

	values map[*session]*sessionXMLTestState
}

func sessionXMLState(s *session) *sessionXMLTestState {
	sessionXMLTestStates.mu.Lock()
	defer sessionXMLTestStates.mu.Unlock()
	if sessionXMLTestStates.values == nil {
		sessionXMLTestStates.values = make(map[*session]*sessionXMLTestState)
	}
	state := sessionXMLTestStates.values[s]
	if state == nil {
		state = new(sessionXMLTestState)
		sessionXMLTestStates.values[s] = state
	}
	return state
}

func startForTest(t *testing.T, s *session, line, col int, start xmlstream.StartElement) error {
	t.Helper()
	tok, err := nextStartForTest(t, s, start)
	if err != nil {
		return err
	}
	return s.start(line, col, tok.Start)
}

func nextStartForTest(t *testing.T, s *session, start xmlstream.StartElement) (*xmlstream.Token, error) {
	t.Helper()
	state := sessionXMLState(s)
	state.input.offset = 0
	state.input.data = appendTestXMLStartBytes(state.input.data[:0], start)
	if !state.ready && len(state.input.data) < xmlstream.XMLDeclarationPrefixLen {
		state.input.data = append(state.input.data, []byte("<!--x-->")...)
	}
	if !state.ready {
		if resetErr := resetSessionXMLReader(s, state); resetErr != nil {
			return nil, resetErr
		}
	}
	tok, err := s.reader.Next()
	if errors.Is(err, xmlstream.ErrReaderState) {
		state.ready = false
		if resetErr := resetSessionXMLReader(s, state); resetErr != nil {
			return nil, resetErr
		}
		tok, err = s.reader.Next()
	}
	if err != nil {
		return nil, err
	}
	if tok.Kind != xmlstream.KindStart {
		return nil, errors.New("test XML input did not produce a start token")
	}
	return tok, nil
}

func endForTest(t *testing.T, s *session, line, col int, end xmlstream.EndElement) error {
	t.Helper()
	state := sessionXMLState(s)
	state.input.offset = 0
	state.input.data = appendTestXMLEndBytes(state.input.data[:0], end)
	if !state.ready {
		return errors.New("test XML reader is not active")
	}
	tok, err := s.reader.Next()
	if err != nil {
		return err
	}
	if tok.Kind != xmlstream.KindEnd {
		return errors.New("test XML input did not produce an end token")
	}
	return s.end(line, col, tok.End)
}

func resetSessionXMLReader(s *session, state *sessionXMLTestState) error {
	if err := s.reader.Reset(&state.input, xmlstream.Config{
		Limits: xmlstream.Limits{
			MaxInputBytes: s.limits.InstanceBytes,
			MaxTokenBytes: s.limits.InstanceTokenBytes,
			MaxAttrs:      s.limits.InstanceAttributes,
			MaxDepth:      s.limits.InstanceDepth,
		},
		LazyAttrValues: true,
	}); err != nil {
		return err
	}
	state.ready = true
	return nil
}
