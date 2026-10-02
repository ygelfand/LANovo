// Package onvif makes a camera findable and describable the way NVRs expect: WS-Discovery on UDP
// 3702 and a minimal ONVIF device and media service over HTTP.
package onvif

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net"
	"strings"
	"sync"
)

const (
	discoveryAddr = "239.255.255.250:3702"
	maxDatagram   = 65535
)

// Probe is the part of a WS-Discovery Probe a responder needs.
type Probe struct {
	MessageID string
	Types     string
}

// ParseProbe reads a Probe. ok is false for anything else, including our own ProbeMatches.
func ParseProbe(b []byte) (Probe, bool) {
	var env struct {
		Header struct {
			MessageID string `xml:"MessageID"`
			Action    string `xml:"Action"`
		} `xml:"Header"`
		Body struct {
			Probe *struct {
				Types string `xml:"Types"`
			} `xml:"Probe"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(b, &env); err != nil || env.Body.Probe == nil {
		return Probe{}, false
	}
	return Probe{MessageID: strings.TrimSpace(env.Header.MessageID), Types: env.Body.Probe.Types}, true
}

// Wants reports whether a probe's Types would include a network video transmitter. An empty
// Types asks for everything.
func (p Probe) Wants() bool {
	t := strings.TrimSpace(p.Types)
	if t == "" {
		return true
	}
	for _, f := range strings.Fields(t) {
		_, local, _ := strings.Cut(f, ":")
		if local == "" {
			local = f
		}
		if local == "NetworkVideoTransmitter" || local == "Device" {
			return true
		}
	}
	return false
}

// Matches is the ProbeMatches answering p for a device at uuid whose device service is at xaddr.
func Matches(p Probe, uuid, xaddr, name string) []byte {
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://www.onvif.org/ver10/network/wsdl" xmlns:tds="http://www.onvif.org/ver10/device/wsdl">`+
		`<s:Header><a:MessageID>urn:uuid:%s</a:MessageID><a:RelatesTo>%s</a:RelatesTo>`+
		`<a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To>`+
		`<a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/ProbeMatches</a:Action></s:Header>`+
		`<s:Body><d:ProbeMatches><d:ProbeMatch>`+
		`<a:EndpointReference><a:Address>urn:uuid:%s</a:Address></a:EndpointReference>`+
		`<d:Types>dn:NetworkVideoTransmitter tds:Device</d:Types>`+
		`<d:Scopes>onvif://www.onvif.org/type/video_encoder onvif://www.onvif.org/Profile/Streaming onvif://www.onvif.org/name/%s onvif://www.onvif.org/hardware/LANovo</d:Scopes>`+
		`<d:XAddrs>%s</d:XAddrs><d:MetadataVersion>1</d:MetadataVersion>`+
		`</d:ProbeMatch></d:ProbeMatches></s:Body></s:Envelope>`,
		NewUUID(), escape(p.MessageID), uuid, scopeName(name), escape(xaddr))
}

func scopeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "%%%02X", r)
		}
	}
	return b.String()
}

func escape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// NewUUID is a random RFC 4122 version 4 UUID.
func NewUUID() string {
	var u [16]byte
	rand.Read(u[:])
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// Responder answers WS-Discovery probes until Close.
type Responder struct {
	UUID  string
	Name  string
	XAddr func(local net.IP) string

	mu   sync.Mutex
	conn *net.UDPConn
}

// Listen joins the discovery group on every multicast interface and answers in the background.
func (r *Responder) Listen() error {
	group, err := net.ResolveUDPAddr("udp4", discoveryAddr)
	if err != nil {
		return err
	}
	c, err := net.ListenMulticastUDP("udp4", nil, group)
	if err != nil {
		return err
	}
	c.SetReadBuffer(maxDatagram)
	r.mu.Lock()
	r.conn = c
	r.mu.Unlock()
	go r.serve(c)
	return nil
}

func (r *Responder) serve(c *net.UDPConn) {
	buf := make([]byte, maxDatagram)
	for {
		n, from, err := c.ReadFromUDP(buf)
		if err != nil {
			return
		}
		p, ok := ParseProbe(buf[:n])
		if !ok || !p.Wants() {
			continue
		}
		local := localFor(from.IP)
		if local == nil {
			continue
		}
		reply := Matches(p, r.UUID, r.XAddr(local), r.Name)
		out, err := net.DialUDP("udp4", nil, from)
		if err != nil {
			continue
		}
		out.Write(reply)
		out.Close()
	}
}

func (r *Responder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil {
		r.conn.Close()
		r.conn = nil
	}
}

func localFor(peer net.IP) net.IP {
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: peer, Port: 3702})
	if err != nil {
		return nil
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}
