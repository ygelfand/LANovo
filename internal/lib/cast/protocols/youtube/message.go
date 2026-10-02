package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
)

// Message is one lounge event: its array id, its name and its payload.
type Message struct {
	AID     int
	Name    string
	Payload json.RawMessage
}

func Frames(r io.Reader, got func([]Message)) error {
	dec := json.NewDecoder(r)
	for {
		var chunk json.RawMessage
		if err := dec.Decode(&chunk); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("lounge: a chunk: %w", err)
		}
		if len(chunk) == 0 || chunk[0] != '[' {
			continue
		}
		msgs, err := messages(chunk)
		if err != nil {
			return err
		}
		got(msgs)
	}
}

func Parse(data string) ([]Message, error) {
	var out []Message
	err := Frames(strings.NewReader(data), func(msgs []Message) { out = append(out, msgs...) })
	return out, err
}

func messages(chunk json.RawMessage) ([]Message, error) {
	var entries [][]json.RawMessage
	if err := json.Unmarshal(chunk, &entries); err != nil {
		return nil, fmt.Errorf("lounge: a chunk: %w", err)
	}
	out := make([]Message, 0, len(entries))
	for _, e := range entries {
		if len(e) != 2 {
			return nil, fmt.Errorf("lounge: a message of %d parts", len(e))
		}
		var msg Message
		if err := json.Unmarshal(e[0], &msg.AID); err != nil {
			return nil, fmt.Errorf("lounge: a message id: %w", err)
		}
		var body []json.RawMessage
		if err := json.Unmarshal(e[1], &body); err != nil || len(body) == 0 {
			return nil, fmt.Errorf("lounge: message %d has no name", msg.AID)
		}
		if err := json.Unmarshal(body[0], &msg.Name); err != nil {
			return nil, fmt.Errorf("lounge: message %d has no name: %w", msg.AID, err)
		}
		switch args := body[1:]; len(args) {
		case 0:
		case 1:
			msg.Payload = args[0]
		default:
			msg.Payload, _ = json.Marshal(args)
		}
		out = append(out, msg)
	}
	return out, nil
}

// String reads a payload that is a plain string.
func (m Message) String() string {
	var s string
	json.Unmarshal(m.Payload, &s)
	return s
}

// First reads the first element of a payload that is an array of strings, which is how the
// session id arrives.
func (m Message) First() string {
	var parts []json.RawMessage
	if json.Unmarshal(m.Payload, &parts) != nil || len(parts) == 0 {
		return m.String()
	}
	var s string
	json.Unmarshal(parts[0], &s)
	return s
}

// Out is a message a screen sends.
type Out struct {
	Name   string
	Fields map[string]string
}

// form is messages as a bind POST carries them, numbered from ofs.
func form(ofs int, msgs []Out) url.Values {
	v := url.Values{}
	v.Set("count", fmt.Sprint(len(msgs)))
	v.Set("ofs", fmt.Sprint(ofs))
	for i, m := range msgs {
		prefix := fmt.Sprintf("req%d_", i)
		v.Set(prefix+"_sc", m.Name)

		keys := make([]string, 0, len(m.Fields))
		for k := range m.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v.Set(prefix+k, m.Fields[k])
		}
	}
	return v
}
