package cast

import "testing"

// Everything here reads bytes off a socket that anything on the network may open. A parser that
// panics on a malformed message is the whole daemon going down and the panel with it, so the bar is
// that nothing crashes — not that anything is understood.

func FuzzUnmarshal(f *testing.F) {
	f.Add(Message{Source: "sender-0", Destination: "receiver-0", Namespace: NSHeartbeat,
		Payload: Ping()}.Marshal())
	f.Add(Message{Namespace: NSDeviceAuth, Binary: []byte{1, 2, 3}}.Marshal())
	f.Add([]byte{})
	f.Add([]byte{0x08})

	f.Fuzz(func(t *testing.T, raw []byte) {
		m, err := Unmarshal(raw)
		if err != nil {
			return
		}

		// What comes back has to survive being written again, since the receiver echoes fields
		// from what it was sent.
		if _, err := Unmarshal(m.Marshal()); err != nil {
			t.Fatalf("a message this parsed would not parse again: %v", err)
		}
	})
}

func FuzzNext(f *testing.F) {
	f.Add(Frame(Message{Namespace: NSReceiver, Payload: `{"type":"GET_STATUS"}`}))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x00, 0x00, 0x00, 0x05, 0x01})

	f.Fuzz(func(t *testing.T, raw []byte) {
		m, n, err := Next(raw)
		if err != nil {
			return
		}
		if n <= 0 || n > len(raw) {
			t.Fatalf("a message of %d bytes was taken out of %d", n, len(raw))
		}

		_ = m
	})
}

func FuzzKind(f *testing.F) {
	f.Add(`{"type":"LAUNCH","requestId":1}`)
	f.Add(`{}`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, payload string) {
		Kind(payload)
	})
}

// The payloads a sender sends, each of which is parsed before anything is done with it.
func FuzzPayloads(f *testing.F) {
	f.Add(`{"type":"LAUNCH","requestId":1,"appId":"CC1AD845"}`)
	f.Add(`{"type":"SET_VOLUME","volume":{"level":0.5}}`)
	f.Add(`{"type":"LOAD","media":{"contentId":"http://x/a.mp3"}}`)
	f.Add(`{"type":"SEEK","currentTime":1e308}`)
	f.Add(`{"media":{"duration":-1}}`)

	f.Fuzz(func(t *testing.T, payload string) {
		ParseLaunch(payload)
		ParseStop(payload)
		ParseSetVolume(payload)
		ParseLoad(payload)
		ParseMediaRequest(payload)
	})
}

// The whole receiver, driven with whatever arrives. Nothing may panic however wrong the message is.
func FuzzReceive(f *testing.F) {
	f.Add(NSReceiver, `{"type":"GET_STATUS","requestId":1}`)
	f.Add(NSMedia, `{"type":"PLAY"}`)
	f.Add(NSConnection, `{"type":"CONNECT"}`)
	f.Add("", "")

	f.Fuzz(func(t *testing.T, namespace, payload string) {
		r := NewReceiver("Kitchen")

		// Launched first, so the media namespace is reachable rather than refused at the door.
		r.Receive(Message{Source: SenderID, Namespace: NSConnection, Payload: Connect()})
		r.Receive(Message{Source: SenderID, Namespace: NSReceiver,
			Payload: `{"type":"LAUNCH","requestId":1,"appId":"` + DefaultMediaReceiver + `"}`})

		r.Receive(Message{Source: SenderID, Namespace: namespace, Payload: payload})
	})
}
