package sdp

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// The requests a phone sends and the answers it expects.
//
// Only one matters in practice. A phone looking for an audio sink sends
// ServiceSearchAttributeRequest — search and read in one round trip — and everything else in the
// protocol exists for clients that want to do it in two. Both are answered anyway, because a phone
// that asks a legal question and gets nothing back waits rather than moving on.

// What a PDU is.
const (
	PDUError                   = 0x01
	PDUServiceSearchRequest    = 0x02
	PDUServiceSearchResponse   = 0x03
	PDUAttributeRequest        = 0x04
	PDUAttributeResponse       = 0x05
	PDUSearchAttributeRequest  = 0x06
	PDUSearchAttributeResponse = 0x07
)

// Why a request could not be answered.
const (
	ErrorBadVersion      = 0x0001
	ErrorBadRecordHandle = 0x0002
	ErrorBadSyntax       = 0x0003
	ErrorBadPDUSize      = 0x0004
	ErrorBadContinuation = 0x0005
	ErrorNoResources     = 0x0006
)

// pduHeader is the id, the transaction and the parameter length.
const pduHeader = 5

// PDU is one message, either way.
type PDU struct {
	ID byte

	// Transaction pairs an answer with its question, and a phone may have several outstanding.
	Transaction uint16

	Params []byte
}

// ParsePDU reads one message.
func ParsePDU(buf []byte) (PDU, error) {
	if len(buf) < pduHeader {
		return PDU{}, ErrShort
	}

	n := int(binary.BigEndian.Uint16(buf[3:]))
	if len(buf) < pduHeader+n {
		return PDU{}, ErrShort
	}

	return PDU{
		ID:          buf[0],
		Transaction: binary.BigEndian.Uint16(buf[1:]),
		Params:      buf[pduHeader : pduHeader+n],
	}, nil
}

// Marshal writes it.
func (p PDU) Marshal() []byte {
	out := make([]byte, pduHeader+len(p.Params))
	out[0] = p.ID
	binary.BigEndian.PutUint16(out[1:], p.Transaction)
	binary.BigEndian.PutUint16(out[3:], uint16(len(p.Params)))
	copy(out[pduHeader:], p.Params)
	return out
}

// Error is the answer to a request that cannot be served.
func Error(transaction uint16, reason uint16) PDU {
	params := make([]byte, 2)
	binary.BigEndian.PutUint16(params, reason)

	return PDU{ID: PDUError, Transaction: transaction, Params: params}
}

// Search is what a phone is looking for and how much it will take back.
type Search struct {
	// Pattern is the service classes it wants, as UUIDs. A record matches when it carries all of
	// them, not any — an audio sink searched for by both AudioSink and A2DP has to have both.
	Pattern []uint32

	// MaxCount for a search, MaxBytes for an attribute read. Only one is meaningful per request.
	MaxCount uint16
	MaxBytes uint16

	// Attributes is what to return, as identifiers or inclusive ranges.
	Attributes []AttrRange

	// Continuation is what was left over last time, echoed back. Empty on a first request.
	Continuation []byte
}

// AttrRange is one attribute or a span of them.
type AttrRange struct{ First, Last uint16 }

// Contains reports whether an identifier falls in the range.
func (r AttrRange) Contains(id uint16) bool { return id >= r.First && id <= r.Last }

// Element writes the range the way an attribute list carries it: two bytes for a single identifier
// and four for a span, because the width is what says which it is.
func (r AttrRange) Element() Element {
	if r.First == r.Last {
		return Uint16(r.First)
	}
	return Uint32(uint32(r.First)<<16 | uint32(r.Last))
}

// ParseSearchAttribute reads a ServiceSearchAttributeRequest, which is the one phones send.
//
// Its shape: a sequence of UUIDs, a two byte cap, a sequence of attribute identifiers or ranges,
// then a continuation blob. The two sequences are data elements and the cap between them is not,
// which is the part that catches a parser written from the diagram rather than the text.
func ParseSearchAttribute(params []byte) (Search, error) {
	var s Search

	pattern, used, err := ParseElement(params)
	if err != nil {
		return s, fmt.Errorf("sdp: the search pattern: %w", err)
	}
	if pattern.Type != TypeSequence {
		return s, fmt.Errorf("sdp: the search pattern is type %d, want a sequence", pattern.Type)
	}
	for _, u := range pattern.Children {
		v, ok := u.Uint()
		if !ok {
			return s, fmt.Errorf("sdp: a search pattern entry is not a uuid")
		}
		s.Pattern = append(s.Pattern, v)
	}
	params = params[used:]

	if len(params) < 2 {
		return s, ErrShort
	}
	s.MaxBytes = binary.BigEndian.Uint16(params)
	params = params[2:]

	list, used, err := ParseElement(params)
	if err != nil {
		return s, fmt.Errorf("sdp: the attribute list: %w", err)
	}
	if list.Type != TypeSequence {
		return s, fmt.Errorf("sdp: the attribute list is type %d, want a sequence", list.Type)
	}
	for _, a := range list.Children {
		v, ok := a.Uint()
		if !ok {
			return s, fmt.Errorf("sdp: an attribute list entry is not a number")
		}

		// Two bytes is one identifier and four is a range, packed as first then last.
		if len(a.Value) == 4 {
			s.Attributes = append(s.Attributes, AttrRange{
				First: uint16(v >> 16),
				Last:  uint16(v),
			})
			continue
		}
		s.Attributes = append(s.Attributes, AttrRange{First: uint16(v), Last: uint16(v)})
	}
	params = params[used:]

	// What is left is the continuation, which carries its own length byte.
	if len(params) < 1 {
		return s, ErrShort
	}
	n := int(params[0])
	if len(params) < 1+n {
		return s, ErrShort
	}
	s.Continuation = params[1 : 1+n]

	return s, nil
}

// Wants reports whether a record's service classes satisfy the search.
//
// All of the pattern has to be present, not any of it. A phone that searches for both AudioSink and
// A2DP is asking for something that is both, and answering with something that is only one of them
// is how a device shows up and then cannot be used.
func (s Search) Wants(classes []uint32) bool {
	for _, want := range s.Pattern {
		if !slices.Contains(classes, want) {
			return false
		}
	}
	return true
}

// Selected is the attributes of a record this search asked for, in identifier order.
func (s Search) Selected(r Record) Record {
	var out Record
	for _, a := range r {
		for _, want := range s.Attributes {
			if want.Contains(a.ID) {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

// SearchAttributeResponse is the answer: the attribute lists of every matching record, wrapped in
// one more sequence, with a length in front and a continuation behind.
//
// The outer sequence is there even for a single record, and leaving it off is the mistake that
// produces a phone which sees the device and finds no services on it.
func SearchAttributeResponse(transaction uint16, records []Record) (PDU, error) {
	lists := make([]Element, 0, len(records))
	for _, r := range records {
		lists = append(lists, r.Element())
	}

	body, err := Sequence(lists...).Marshal()
	if err != nil {
		return PDU{}, err
	}

	params := make([]byte, 2, 2+len(body)+1)
	binary.BigEndian.PutUint16(params, uint16(len(body)))
	params = append(params, body...)

	// No continuation: everything fitted. A response too large for one PDU is a case this does not
	// handle yet, and a record this small will not reach it.
	params = append(params, 0)

	return PDU{ID: PDUSearchAttributeResponse, Transaction: transaction, Params: params}, nil
}
