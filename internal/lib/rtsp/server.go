package rtsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const idle = 60 * time.Second

// Server answers RTSP for the streams it has been handed, by path.
type Server struct {
	// Wait is how long DESCRIBE waits for a stream's first parameter sets.
	Wait time.Duration

	mu      sync.Mutex
	streams map[string]*Stream
	lns     []net.Listener
	conns   map[*conn]bool
	closed  bool
}

func NewServer() *Server {
	return &Server{Wait: 5 * time.Second, streams: map[string]*Stream{}, conns: map[*conn]bool{}}
}

// Handle serves st at rtsp://host/path.
func (s *Server) Handle(path string, st *Stream) {
	s.mu.Lock()
	s.streams[strings.Trim(path, "/")] = st
	s.mu.Unlock()
}

func (s *Server) stream(path string) *Stream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streams[path]
}

// Serve accepts on ln until Close.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return net.ErrClosed
	}
	s.lns = append(s.lns, ln)
	s.mu.Unlock()
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			return err
		}
		cn := &conn{srv: s, c: c}
		s.mu.Lock()
		s.conns[cn] = true
		s.mu.Unlock()
		go cn.serve()
	}
}

// Close stops listening and ends every session.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	lns := s.lns
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, ln := range lns {
		ln.Close()
	}
	for _, c := range conns {
		c.c.Close()
	}
	return nil
}

type request struct {
	method, url string
	header      map[string]string
}

type conn struct {
	srv *Server
	c   net.Conn
	wmu sync.Mutex
	ses *session
}

func (c *conn) interleaved(channel byte, p []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.c.Write(frame(channel, p))
	return err
}

func (c *conn) serve() {
	defer func() {
		if c.ses != nil {
			c.ses.close()
		}
		c.c.Close()
		c.srv.mu.Lock()
		delete(c.srv.conns, c)
		c.srv.mu.Unlock()
	}()
	r := bufio.NewReader(c.c)
	for {
		if c.ses != nil && c.ses.tcp != nil {
			c.c.SetReadDeadline(time.Time{})
		} else {
			c.c.SetReadDeadline(time.Now().Add(idle))
		}
		b, err := r.Peek(1)
		if err != nil {
			return
		}
		if b[0] == '$' {
			var h [4]byte
			if _, err := io.ReadFull(r, h[:]); err != nil {
				return
			}
			if _, err := r.Discard(int(h[2])<<8 | int(h[3])); err != nil {
				return
			}
			continue
		}
		req, err := readRequest(r)
		if err != nil {
			return
		}
		status, headers, body := c.handle(req)
		if err := c.reply(req, status, headers, body); err != nil {
			return
		}
	}
}

func readRequest(r *bufio.Reader) (*request, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(line)
	if len(parts) != 3 || !strings.HasPrefix(parts[2], "RTSP/") {
		return nil, errors.New("rtsp: bad request line")
	}
	req := &request{method: parts[0], url: parts[1], header: map[string]string{}}
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		l = strings.TrimRight(l, "\r\n")
		if l == "" {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if ok {
			req.header[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	if n, err := strconv.Atoi(req.header["content-length"]); err == nil && n > 0 {
		if _, err := r.Discard(n); err != nil {
			return nil, err
		}
	}
	return req, nil
}

var reasons = map[int]string{
	200: "OK", 400: "Bad Request", 404: "Not Found", 454: "Session Not Found",
	461: "Unsupported Transport", 501: "Not Implemented", 503: "Service Unavailable",
}

func (c *conn) reply(req *request, status int, headers []string, body string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "RTSP/1.0 %d %s\r\n", status, reasons[status])
	if cseq := req.header["cseq"]; cseq != "" {
		fmt.Fprintf(&b, "CSeq: %s\r\n", cseq)
	}
	b.WriteString("Server: LANovo\r\n")
	for _, h := range headers {
		b.WriteString(h + "\r\n")
	}
	if body != "" {
		fmt.Fprintf(&b, "Content-Length: %d\r\n", len(body))
	}
	b.WriteString("\r\n" + body)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := io.WriteString(c.c, b.String())
	return err
}

func streamPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := strings.Trim(u.Path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && strings.HasPrefix(p[i+1:], "trackID=") {
		p = p[:i]
	} else if strings.HasPrefix(p, "trackID=") {
		p = ""
	}
	return p
}

func (c *conn) handle(req *request) (int, []string, string) {
	switch req.method {
	case "OPTIONS":
		return 200, []string{"Public: OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN, GET_PARAMETER, SET_PARAMETER"}, ""
	case "DESCRIBE":
		st := c.srv.stream(streamPath(req.url))
		if st == nil {
			return 404, nil, ""
		}
		st.demand()
		host, _, _ := net.SplitHostPort(c.c.LocalAddr().String())
		sdp, ok := st.sdp(rand.Uint32(), host, c.srv.Wait)
		if !ok {
			return 503, nil, ""
		}
		return 200, []string{"Content-Type: application/sdp", "Content-Base: " + strings.TrimSuffix(req.url, "/") + "/"}, sdp
	case "SETUP":
		return c.setup(req)
	case "PLAY":
		if c.ses == nil || !sameSession(req, c.ses) {
			return 454, nil, ""
		}
		c.ses.stream.demand()
		c.ses.play()
		seq, ts := c.ses.rtpInfo()
		return 200, []string{
			"Session: " + c.ses.id,
			"Range: npt=0.000-",
			fmt.Sprintf("RTP-Info: url=%s/trackID=0;seq=%d;rtptime=%d", strings.TrimSuffix(req.url, "/"), seq, ts),
		}, ""
	case "TEARDOWN":
		if c.ses != nil {
			c.ses.close()
			c.ses = nil
		}
		return 200, nil, ""
	case "GET_PARAMETER", "SET_PARAMETER":
		var h []string
		if c.ses != nil {
			h = append(h, "Session: "+c.ses.id)
		}
		return 200, h, ""
	}
	return 501, nil, ""
}

func sameSession(req *request, s *session) bool {
	id, _, _ := strings.Cut(req.header["session"], ";")
	return strings.TrimSpace(id) == s.id
}

func (c *conn) setup(req *request) (int, []string, string) {
	st := c.srv.stream(streamPath(req.url))
	if st == nil {
		return 404, nil, ""
	}
	st.demand()
	if c.ses != nil {
		c.ses.close()
	}
	s := newSession(strconv.FormatUint(uint64(rand.Uint32()), 16))
	s.stream = st
	tr := req.header["transport"]
	sid := "Session: " + s.id + fmt.Sprintf(";timeout=%d", int(idle.Seconds()))

	if strings.Contains(tr, "RTP/AVP/TCP") {
		lo := 0
		if v := param(tr, "interleaved"); v != "" {
			a, _, _ := strings.Cut(v, "-")
			if n, err := strconv.Atoi(a); err == nil {
				lo = n
			}
		}
		s.tcp, s.channel = c, byte(lo)
		c.ses = s
		return 200, []string{sid, fmt.Sprintf("Transport: RTP/AVP/TCP;unicast;interleaved=%d-%d;ssrc=%08X", lo, lo+1, s.pz.ssrc)}, ""
	}

	ports := param(tr, "client_port")
	a, _, _ := strings.Cut(ports, "-")
	port, err := strconv.Atoi(a)
	if err != nil {
		return 461, nil, ""
	}
	remote, _, _ := net.SplitHostPort(c.c.RemoteAddr().String())
	peer, err := net.ResolveUDPAddr("udp", net.JoinHostPort(remote, strconv.Itoa(port)))
	if err != nil {
		return 461, nil, ""
	}
	rtp, rtcp, err := udpPair()
	if err != nil {
		return 503, nil, ""
	}
	s.udp, s.rtcp, s.peer = rtp, rtcp, peer
	c.ses = s
	lp := rtp.LocalAddr().(*net.UDPAddr).Port
	return 200, []string{sid, fmt.Sprintf("Transport: RTP/AVP;unicast;client_port=%d-%d;server_port=%d-%d;ssrc=%08X",
		port, port+1, lp, lp+1, s.pz.ssrc)}, ""
}

func param(transport, name string) string {
	for _, f := range strings.Split(transport, ";") {
		if k, v, ok := strings.Cut(strings.TrimSpace(f), "="); ok && k == name {
			return v
		}
	}
	return ""
}

func udpPair() (*net.UDPConn, *net.UDPConn, error) {
	for range 16 {
		rtp, err := net.ListenUDP("udp", &net.UDPAddr{})
		if err != nil {
			return nil, nil, err
		}
		port := rtp.LocalAddr().(*net.UDPAddr).Port
		if port%2 == 0 {
			if rtcp, err := net.ListenUDP("udp", &net.UDPAddr{Port: port + 1}); err == nil {
				return rtp, rtcp, nil
			}
		}
		rtp.Close()
	}
	return nil, nil, errors.New("rtsp: no even/odd UDP port pair free")
}
