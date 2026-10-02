package rtsp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	sps = []byte{0x67, 0x42, 0xc0, 0x1f, 0xda, 0x01, 0x40, 0x16, 0xe8}
	pps = []byte{0x68, 0xce, 0x3c, 0x80}
)

func annexB(nalus ...[]byte) []byte {
	var b []byte
	for _, n := range nalus {
		b = append(b, 0, 0, 0, 1)
		b = append(b, n...)
	}
	return b
}

func slice(kind byte, n int) []byte {
	b := make([]byte, n)
	b[0] = 0x60 | kind
	for i := 1; i < n; i++ {
		b[i] = byte(i*7 + 1)
	}
	return b
}

func TestNALUsSplitsThreeAndFourByteStartCodes(t *testing.T) {
	in := append([]byte{0, 0, 1}, sps...)
	in = append(in, 0, 0, 0, 1)
	in = append(in, pps...)
	got := NALUs(in)
	if len(got) != 2 || !bytes.Equal(got[0], sps) || !bytes.Equal(got[1], pps) {
		t.Fatalf("got %x", got)
	}
}

func TestFragmentsReassembleToTheNALU(t *testing.T) {
	big := slice(nalIDR, 5000)
	p := packetizer{ssrc: 1}
	pkts := p.packets([][]byte{sps, big}, 42)
	nalus := depacketize(t, pkts)
	if len(nalus) != 2 || !bytes.Equal(nalus[0], sps) || !bytes.Equal(nalus[1], big) {
		t.Fatalf("reassembled %d NALUs", len(nalus))
	}
	for i, pk := range pkts {
		if len(pk) > headerLen+maxPayload {
			t.Errorf("packet %d is %d bytes", i, len(pk))
		}
		marker := pk[1]&0x80 != 0
		if marker != (i == len(pkts)-1) {
			t.Errorf("packet %d marker %v", i, marker)
		}
		if seq := binary.BigEndian.Uint16(pk[2:]); seq != uint16(i) {
			t.Errorf("packet %d seq %d", i, seq)
		}
	}
}

func depacketize(t *testing.T, pkts [][]byte) [][]byte {
	t.Helper()
	var out [][]byte
	var fu []byte
	for _, pk := range pkts {
		body := pk[headerLen:]
		if body[0]&0x1f != 28 {
			out = append(out, append([]byte(nil), body...))
			continue
		}
		if body[1]&0x80 != 0 {
			fu = []byte{body[0]&0xe0 | body[1]&0x1f}
		}
		fu = append(fu, body[2:]...)
		if body[1]&0x40 != 0 {
			out = append(out, fu)
			fu = nil
		}
	}
	return out
}

type client struct {
	t    *testing.T
	c    net.Conn
	r    *bufio.Reader
	cseq int
	url  string
}

func dial(t *testing.T, addr, path string) *client {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(5 * time.Second))
	return &client{t: t, c: c, r: bufio.NewReader(c), url: "rtsp://" + addr + "/" + path}
}

func (cl *client) do(method, url string, headers ...string) (int, map[string]string, string) {
	cl.t.Helper()
	cl.cseq++
	fmt.Fprintf(cl.c, "%s %s RTSP/1.0\r\nCSeq: %d\r\n%s\r\n", method, url, cl.cseq, joinHeaders(headers))
	line, err := cl.r.ReadString('\n')
	if err != nil {
		cl.t.Fatal(err)
	}
	f := strings.Fields(line)
	code, _ := strconv.Atoi(f[1])
	h := map[string]string{}
	for {
		l, _ := cl.r.ReadString('\n')
		l = strings.TrimRight(l, "\r\n")
		if l == "" {
			break
		}
		k, v, _ := strings.Cut(l, ":")
		h[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	if h["cseq"] != strconv.Itoa(cl.cseq) {
		cl.t.Errorf("%s answered CSeq %q, want %d", method, h["cseq"], cl.cseq)
	}
	body := ""
	if n, _ := strconv.Atoi(h["content-length"]); n > 0 {
		b := make([]byte, n)
		io.ReadFull(cl.r, b)
		body = string(b)
	}
	return code, h, body
}

func joinHeaders(h []string) string {
	var b strings.Builder
	for _, s := range h {
		b.WriteString(s + "\r\n")
	}
	return b.String()
}

func serve(t *testing.T) (*Server, *Stream, string) { return serveWaiting(t, 3*time.Second) }

func serveWaiting(t *testing.T, wait time.Duration) (*Server, *Stream, string) {
	srv := NewServer()
	srv.Wait = wait
	st := NewStream()
	srv.Handle("main", st)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return srv, st, ln.Addr().String()
}

func TestDescribeWaitsForParameterSets(t *testing.T) {
	_, _, quick := serveWaiting(t, 50*time.Millisecond)
	early := dial(t, quick, "main")
	if code, _, _ := early.do("DESCRIBE", early.url); code != 503 {
		t.Fatalf("DESCRIBE with no frame in time: %d", code)
	}

	_, st, addr := serve(t)
	cl := dial(t, addr, "main")
	go func() {
		time.Sleep(100 * time.Millisecond)
		st.Write(annexB(sps, pps, slice(nalIDR, 100)), 0)
	}()
	code, h, body := cl.do("DESCRIBE", cl.url)
	if code != 200 || h["content-type"] != "application/sdp" {
		t.Fatalf("DESCRIBE: %d %v", code, h)
	}
	for _, want := range []string{"H264/90000", "packetization-mode=1", "profile-level-id=42C01F", "sprop-parameter-sets=Z0LAH9oBQBbo,aM48gA==", "a=control:trackID=0"} {
		if !strings.Contains(body, want) {
			t.Errorf("SDP lacks %q:\n%s", want, body)
		}
	}
	if code, _, _ := dial(t, addr, "nope").do("DESCRIBE", "rtsp://"+addr+"/nope"); code != 404 {
		t.Errorf("unknown path: %d", code)
	}
}

func TestInterleavedPlayStartsAtAKeyframe(t *testing.T) {
	_, st, addr := serve(t)
	st.Write(annexB(sps, pps, slice(nalIDR, 100)), 0)
	cl := dial(t, addr, "main")
	cl.do("OPTIONS", cl.url)
	cl.do("DESCRIBE", cl.url)
	code, h, _ := cl.do("SETUP", cl.url+"/trackID=0", "Transport: RTP/AVP/TCP;unicast;interleaved=0-1")
	if code != 200 || !strings.Contains(h["transport"], "interleaved=0-1") {
		t.Fatalf("SETUP: %d %v", code, h)
	}
	sid, _, _ := strings.Cut(h["session"], ";")
	if code, _, _ := cl.do("PLAY", cl.url, "Session: wrong"); code != 454 {
		t.Fatalf("PLAY with a stranger's session: %d", code)
	}
	if code, _, _ := cl.do("PLAY", cl.url, "Session: "+sid); code != 200 {
		t.Fatalf("PLAY: %d", code)
	}
	waitViewers(t, st, 1)

	st.Write(annexB(slice(1, 50)), 40*time.Millisecond)
	idr := slice(nalIDR, 4000)
	st.Write(annexB(idr), 80*time.Millisecond)

	var pkts [][]byte
	for len(pkts) == 0 || pkts[len(pkts)-1][1]&0x80 == 0 {
		var hdr [4]byte
		if _, err := io.ReadFull(cl.r, hdr[:]); err != nil {
			t.Fatal(err)
		}
		if hdr[0] != '$' || hdr[1] != 0 {
			t.Fatalf("frame header %x", hdr)
		}
		p := make([]byte, binary.BigEndian.Uint16(hdr[2:]))
		io.ReadFull(cl.r, p)
		pkts = append(pkts, p)
	}
	nalus := depacketize(t, pkts)
	if len(nalus) != 3 || !bytes.Equal(nalus[0], sps) || !bytes.Equal(nalus[1], pps) || !bytes.Equal(nalus[2], idr) {
		t.Fatalf("first access unit played: %d NALUs", len(nalus))
	}

	cl.do("TEARDOWN", cl.url, "Session: "+sid)
	waitViewers(t, st, 0)
}

func TestASenderReportFollowsTheFirstAccessUnit(t *testing.T) {
	_, st, addr := serve(t)
	st.Write(annexB(sps, pps, slice(nalIDR, 100)), 0)
	cl := dial(t, addr, "main")
	_, h, _ := cl.do("SETUP", cl.url+"/trackID=0", "Transport: RTP/AVP/TCP;unicast;interleaved=2-3")
	sid, _, _ := strings.Cut(h["session"], ";")
	cl.do("PLAY", cl.url, "Session: "+sid)
	waitViewers(t, st, 1)

	st.Write(annexB(slice(nalIDR, 4000)), 80*time.Millisecond)
	var packets, octets uint32
	var ssrc, ts uint32
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(cl.r, hdr[:]); err != nil {
			t.Fatal(err)
		}
		p := make([]byte, binary.BigEndian.Uint16(hdr[2:]))
		io.ReadFull(cl.r, p)
		if hdr[1] == 2 {
			packets++
			octets += uint32(len(p) - headerLen)
			ts, ssrc = binary.BigEndian.Uint32(p[4:]), binary.BigEndian.Uint32(p[8:])
			continue
		}
		if hdr[1] != 3 {
			t.Fatalf("a frame on channel %d", hdr[1])
		}
		if p[1] != rtcpSR || binary.BigEndian.Uint16(p[2:]) != 6 {
			t.Fatalf("control packet type %d length %d", p[1], binary.BigEndian.Uint16(p[2:]))
		}
		if got := binary.BigEndian.Uint32(p[4:]); got != ssrc {
			t.Errorf("report SSRC %08x, media %08x", got, ssrc)
		}
		sent := time.Unix(int64(binary.BigEndian.Uint32(p[8:]))-ntpToUnix, 0)
		if d := time.Since(sent); d < -time.Second || d > 2*time.Second {
			t.Errorf("report wall clock %v away from now", d)
		}
		if got := binary.BigEndian.Uint32(p[16:]); got != ts {
			t.Errorf("report RTP time %d, last packet's %d", got, ts)
		}
		if got := binary.BigEndian.Uint32(p[20:]); got != packets {
			t.Errorf("report counts %d packets, %d arrived", got, packets)
		}
		if got := binary.BigEndian.Uint32(p[24:]); got != octets {
			t.Errorf("report counts %d octets, %d arrived", got, octets)
		}
		sdes := p[28:]
		if len(sdes)%4 != 0 || sdes[1] != rtcpSDES || int(binary.BigEndian.Uint16(sdes[2:])+1)*4 != len(sdes) {
			t.Fatalf("SDES %x", sdes)
		}
		if sdes[8] != 1 || !strings.HasPrefix(string(sdes[10:10+int(sdes[9])]), "lanovo-") {
			t.Errorf("SDES item %d %q", sdes[8], sdes[10:10+int(sdes[9])])
		}
		return
	}
}

func TestUDPPlaySendsToTheClientPort(t *testing.T) {
	_, st, addr := serve(t)
	st.Write(annexB(sps, pps, slice(nalIDR, 100)), 0)
	rx, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer rx.Close()
	port := rx.LocalAddr().(*net.UDPAddr).Port

	cl := dial(t, addr, "main")
	code, h, _ := cl.do("SETUP", cl.url+"/trackID=0", fmt.Sprintf("Transport: RTP/AVP;unicast;client_port=%d-%d", port, port+1))
	if code != 200 || !strings.Contains(h["transport"], "server_port=") {
		t.Fatalf("SETUP: %d %v", code, h)
	}
	sid, _, _ := strings.Cut(h["session"], ";")
	cl.do("PLAY", cl.url, "Session: "+sid)
	waitViewers(t, st, 1)

	st.Write(annexB(slice(nalIDR, 200)), 40*time.Millisecond)
	rx.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 2000)
	n, _, err := rx.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < headerLen || buf[1]&0x7f != payloadType {
		t.Fatalf("got %d bytes, payload type %d", n, buf[1]&0x7f)
	}

	cl.c.Close()
	waitViewers(t, st, 0)
}

func TestAskingForAStreamCallsForItsSource(t *testing.T) {
	srv := NewServer()
	srv.Wait = 2 * time.Second
	st := NewStream()
	asked := make(chan struct{}, 8)
	st.OnDemand = func() {
		asked <- struct{}{}
		go st.Write(annexB(sps, pps, slice(nalIDR, 100)), 0)
	}
	srv.Handle("main", st)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer srv.Close()

	cl := dial(t, ln.Addr().String(), "main")
	if code, _, _ := cl.do("DESCRIBE", cl.url); code != 200 {
		t.Fatalf("DESCRIBE with a source that starts on demand: %d", code)
	}
	select {
	case <-asked:
	default:
		t.Fatal("DESCRIBE did not ask for the source")
	}
}

func waitViewers(t *testing.T, st *Stream, n int) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for st.Viewers() != n {
		if time.Now().After(until) {
			t.Fatalf("%d viewers, want %d", st.Viewers(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
