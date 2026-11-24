package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

func readJSON(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	if err := decodeJSON(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// decodeJSON decodes the one JSON value of data into v. encoding/json matches
// keys case-insensitively, keeps the last of repeated keys and ignores keys v
// does not declare; decodeJSON rejects all three, so every key of data names
// a declared field, or a map entry, with its exact spelling, and once.
func decodeJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("data after the JSON value")
	}
	keys, err := objectKeys(data)
	if err != nil {
		return err
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return err
	}
	declared, err := objectKeys(canonical)
	if err != nil {
		return err
	}
	for key := range keys {
		if !declared[key] {
			return fmt.Errorf("key %q differs in case from the field it sets", key)
		}
	}
	return nil
}

// objectKeys returns every object key of the JSON value data, and fails if
// one object holds two keys that encoding/json would match to one field:
// equal ones, or ones equal ignoring case.
func objectKeys(data []byte) (map[string]bool, error) {
	all := make(map[string]bool)
	var stack []keyFrame
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return all, nil
		}
		if err != nil {
			return nil, err
		}
		if key, ok := objectKey(stack, tok); ok {
			if err := stack[len(stack)-1].addKey(key); err != nil {
				return nil, fmt.Errorf("%w at byte %d", err, dec.InputOffset())
			}
			all[key] = true
			continue
		}
		stack = nextFrames(stack, tok)
	}
}

// keyFrame is an open object or array of the objectKeys walk. An object maps
// the lowercased form of each of its keys to the key.
type keyFrame struct {
	keys      map[string]string
	expectKey bool
}

// objectKey returns tok as a key if it is one: a string where the innermost
// open object expects a key.
func objectKey(stack []keyFrame, tok json.Token) (string, bool) {
	key, ok := tok.(string)
	if !ok || len(stack) == 0 || !stack[len(stack)-1].expectKey {
		return "", false
	}
	return key, true
}

func (f *keyFrame) addKey(key string) error {
	folded := strings.ToLower(key)
	if prev, seen := f.keys[folded]; seen {
		return fmt.Errorf("object repeats key %q as %q", prev, key)
	}
	f.keys[folded] = key
	f.expectKey = false
	return nil
}

// nextFrames returns stack after a token that is not an object key: the token
// opens or closes a frame, and a completed value in an object makes the object
// expect a key.
func nextFrames(stack []keyFrame, tok json.Token) []keyFrame {
	switch tok {
	case json.Delim('{'):
		return append(stack, keyFrame{keys: make(map[string]string), expectKey: true})
	case json.Delim('['):
		return append(stack, keyFrame{})
	case json.Delim('}'), json.Delim(']'):
		stack = stack[:len(stack)-1]
	}
	if top := len(stack) - 1; top >= 0 && stack[top].keys != nil {
		stack[top].expectKey = true
	}
	return stack
}
