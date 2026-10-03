package cast

import "testing"

// The device's own volume reaches every sender, and a level they already have sends nothing.
func TestTheDeviceVolumeIsReportedOnlyWhenItMoved(t *testing.T) {
	r := NewReceiver("Kitchen")
	for _, s := range []string{"sender-a", "sender-b"} {
		if _, err := r.Receive(from(s, NSConnection, `{"type":"CONNECT"}`)); err != nil {
			t.Fatal(err)
		}
	}

	out := r.Report(0.4, false)
	if len(out) != 1 || out[0].Destination != Broadcast {
		t.Fatalf("a new level sent %+v, want one broadcast", out)
	}
	got := status(t, out[0].Payload).Volume
	if *got.Level != 0.4 || *got.Muted {
		t.Errorf("the status says %v muted=%v, want 0.4 unmuted", *got.Level, *got.Muted)
	}

	if again := r.Report(0.4, false); len(again) != 0 {
		t.Errorf("the same level was sent again, %d messages", len(again))
	}
	if muted := r.Report(0.4, true); len(muted) != 1 {
		t.Errorf("a mute sent %d messages, want one broadcast", len(muted))
	}
}

// What a sender loads and what it says on a namespace nobody answers are both reported.
func TestLoadsAndUnspokenNamespacesAreReported(t *testing.T) {
	r := NewReceiver("Kitchen")

	var loaded []Media
	var unspoken [][2]string
	r.Loaded = func(m Media) { loaded = append(loaded, m) }
	r.Unspoken = func(namespace, kind string) { unspoken = append(unspoken, [2]string{namespace, kind}) }

	answered(t, r, from(SenderID, NSReceiver, `{"type":"LAUNCH","requestId":1,"appId":"C35B0678"}`))
	r.Receive(from(SenderID, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://ma.local/flow/1.flac","contentType":"audio/flac","streamType":"BUFFERED"}}`))
	r.Receive(from(SenderID, "urn:x-cast:io.music-assistant.cast", `{"type":"STATUS"}`))

	if len(loaded) != 1 || loaded[0].ContentType != "audio/flac" {
		t.Errorf("the load reported was %+v", loaded)
	}
	want := [][2]string{{"urn:x-cast:io.music-assistant.cast", "STATUS"}}
	if len(unspoken) != len(want) || unspoken[0] != want[0] {
		t.Errorf("unspoken namespaces were %v, want %v", unspoken, want)
	}
}
