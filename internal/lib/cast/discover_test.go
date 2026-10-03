package cast

import (
	"crypto/tls"
	"encoding/hex"
	"strings"
	"testing"
)

// txt reads the records into a map, which is how a resolver hands them over.
func txt(t *testing.T, d Device) map[string]string {
	t.Helper()

	out := map[string]string{}
	for _, r := range d.Records() {
		k, v, ok := strings.Cut(r, "=")
		if !ok {
			t.Fatalf("%q is not a key and a value", r)
		}
		if _, seen := out[k]; seen {
			t.Errorf("%q appears twice", k)
		}
		out[k] = v
	}
	return out
}

// A sender reads these before it connects at all, so a record that is missing is a device that is
// discovered and then not offered.
func TestTheRecordSaysEverythingASenderReads(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Lenovo Smart Display 10")

	got := txt(t, d)
	for _, want := range []string{"id", "cd", "ve", "md", "fn", "ca", "st", "bs", "nf", "rs"} {
		if _, ok := got[want]; !ok {
			t.Errorf("the record has no %q", want)
		}
	}

	if got["fn"] != "Kitchen" {
		t.Errorf("the name is %q", got["fn"])
	}
	if got["md"] != "Lenovo Smart Display 10" {
		t.Errorf("the model is %q", got["md"])
	}
	if got["ca"] != "231941" {
		t.Errorf("the capabilities are %q, want what a smart display advertises", got["ca"])
	}
}

// A sender remembers the id. One that changes is a new device in the cast menu every reboot, and
// the old ones never go away.
func TestTheIdIsStableForAnAddress(t *testing.T) {
	first := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")
	again := Identify("00:F4:8D:47:69:21", "Renamed", "Other")

	if first.ID != again.ID {
		t.Errorf("the id changed with the case of the address: %q and %q", first.ID, again.ID)
	}

	other := Identify("00:f4:8d:47:69:22", "Kitchen", "Model")
	if other.ID == first.ID {
		t.Error("two addresses gave the same id")
	}
}

func TestTheIdIsThirtyTwoHexCharacters(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")

	if len(d.ID) != 32 {
		t.Fatalf("the id is %d characters", len(d.ID))
	}
	if _, err := hex.DecodeString(d.ID); err != nil {
		t.Errorf("the id is not hexadecimal: %q", d.ID)
	}
}

// Hashing the address alone would give the same number for everything derived from it, and a
// device whose cloud id equals its device id is one a sender can be confused by.
func TestTheDerivedIdentifiersAreNotEachOther(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")
	got := txt(t, d)

	if got["id"] == got["cd"] {
		t.Error("the device id and the cloud id are the same number")
	}
	if strings.EqualFold(got["id"][:12], got["bs"]) {
		t.Error("the bootstrap id is the front of the device id")
	}
}

// st is what a sender shows as the device being busy.
func TestTheRecordSaysWhetherSomethingIsRunning(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")

	if got := txt(t, d)["st"]; got != "0" {
		t.Errorf("an idle device says st=%q", got)
	}

	d.Running = true
	d.Status = "Sun Ra"

	got := txt(t, d)
	if got["st"] != "1" {
		t.Errorf("a busy device says st=%q", got["st"])
	}
	if got["rs"] != "Sun Ra" {
		t.Errorf("the status line is %q", got["rs"])
	}
}

// The instance is the id and not the name: a friendly name may be anything somebody typed, and two
// services with one instance name is a pair of devices that take turns existing.
func TestTheInstanceNameIsTheId(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen Speaker!", "Model")

	if d.Instance() != d.ID {
		t.Errorf("the instance is %q", d.Instance())
	}
}

func TestWhatWouldNotBeOffered(t *testing.T) {
	good := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")
	if err := good.Valid(); err != nil {
		t.Fatalf("a good device was refused: %v", err)
	}

	for _, tc := range []struct {
		name   string
		change func(*Device)
	}{
		{"no id", func(d *Device) { d.ID = "" }},
		{"a short id", func(d *Device) { d.ID = "abcd" }},
		{"an id that is not hex", func(d *Device) { d.ID = strings.Repeat("z", 32) }},
		{"no name", func(d *Device) { d.Name = "" }},
		{"no capabilities", func(d *Device) { d.Capabilities = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := good
			tc.change(&d)

			if err := d.Valid(); err == nil {
				t.Error("was accepted")
			}
		})
	}
}

// The whole thing over real TLS on a real socket: a sender connects, talks, and is answered.
//
// Not a network test in any way that matters — it is a loopback listener on a port the kernel
// chose — but it is the one check that the certificate, the handshake, the framing and the state
// machine work together rather than each on its own.
func TestASenderConnectsOverTLSAndIsAnswered(t *testing.T) {
	creds := issued(t)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", TLS(func() *Credentials { return creds }))
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	s := NewService(NewReceiver("Kitchen"))
	s.Receiver.Player = &player{}

	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			go s.Serve(NewConn(c))
		}
	}()

	// A sender does not verify the certificate, because there is nothing for it to verify against.
	conn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	phone := &sender{t: t, conn: conn}

	phone.send(from(SenderID, NSConnection, Connect()))
	phone.send(from(SenderID, NSReceiver, `{"type":"GET_STATUS","requestId":1}`))

	got := phone.expect()
	h, err := Kind(got.Payload)
	if err != nil {
		t.Fatalf("the answer did not parse: %v", err)
	}
	if h.Type != TypeStatus || h.RequestID != 1 {
		t.Errorf("came back %+v", h)
	}

	// And something bigger than one read, so the framing is exercised over a real socket.
	phone.send(from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":2,"appId":"`+DefaultMediaReceiver+`"}`))

	launched := phone.expect()
	phone.expect()
	transport := status(t, launched.Payload).Applications[0].TransportID

	long := strings.Repeat("a very long track title ", 400)
	phone.send(toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":3,"media":{"contentId":"http://example/a.mp3",`+
			`"metadata":{"metadataType":3,"title":"`+long+`"}}}`))

	loaded := mediaStatus(t, phone.expect().Payload)
	if len(loaded) != 1 {
		t.Fatalf("%d statuses", len(loaded))
	}
	if loaded[0].Media == nil || loaded[0].Media.Metadata.Title != long {
		t.Error("a message larger than one read did not come back whole")
	}
}

// A sender that cannot negotiate is a device that appears in the list and then fails to connect,
// with nothing on either side saying why.
func TestOlderSendersCanStillNegotiate(t *testing.T) {
	creds := issued(t)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", TLS(func() *Credentials { return creds }))
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	// The handshake is lazy: an accepted connection does nothing until it is read or written, so
	// the server has to be told to do it or the client sees the socket close instead.
	go func() {
		c, err := listener.Accept()
		if err != nil {
			return
		}
		defer c.Close()

		if tc, ok := c.(*tls.Conn); ok {
			tc.Handshake()
		}
	}()

	conn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		MaxVersion:         tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("a sender limited to tls 1.2 could not connect: %v", err)
	}
	conn.Close()
}

// Asking a phone for a client certificate gets a handshake most of them abandon.
func TestNoClientCertificateIsAskedFor(t *testing.T) {
	if got := TLS(func() *Credentials { return nil }).ClientAuth; got != tls.NoClientCert {
		t.Errorf("the server asks for %v", got)
	}
}

// The port is what senders connect to, and a few have it built in.
func TestThePortIsTheOneSendersUse(t *testing.T) {
	if Port != 8009 {
		t.Errorf("the port is %d", Port)
	}
	if ServiceType != "_googlecast._tcp" {
		t.Errorf("the service is %q", ServiceType)
	}
}

// A record value with no equals sign in it is not a record, and a value containing one is still
// only split at the first.
func TestARecordValueMayContainAnEqualsSign(t *testing.T) {
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Model")
	d.Status = "playing a=b"

	if got := txt(t, d)["rs"]; got != "playing a=b" {
		t.Errorf("the status came back %q", got)
	}
}
