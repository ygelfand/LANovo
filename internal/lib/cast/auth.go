package cast

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// Field numbers from cast_channel.proto.
const (
	authChallenge = 1
	authResponse  = 2
	authError     = 3

	challengeAlgorithm = 1
	challengeNonce     = 2
	challengeHash      = 3

	responseSignature    = 1
	responseDevice       = 2
	responseIntermediate = 3
	responseAlgorithm    = 4
	responseHash         = 6
	responseCRL          = 7

	errorType = 1
)

// Hash is the digest a challenge asks the signature to be made over.
type Hash int

const (
	SHA1   Hash = 0
	SHA256 Hash = 1
)

func (h Hash) String() string {
	if h == SHA256 {
		return "sha256"
	}
	return "sha1"
}

// The signature schemes a challenge can name.
const (
	PKCS1v15 = 1
	PSS      = 2
)

// The errors an answer can carry instead of a response.
const (
	AuthInternal    = 0
	AuthNoTLS       = 1
	AuthNoAlgorithm = 2
)

// Challenge is a sender asking this device to prove it is a Chromecast.
type Challenge struct {
	Algorithm int
	Nonce     []byte
	Hash      Hash
}

// ParseChallenge reads a DeviceAuthMessage and takes the challenge out of it.
func ParseChallenge(b []byte) (Challenge, error) {
	body, ok, err := field(b, authChallenge)
	if err != nil {
		return Challenge{}, err
	}
	if !ok {
		return Challenge{}, fmt.Errorf("cast: a device auth message with no challenge")
	}

	c := Challenge{Algorithm: PKCS1v15}
	for len(body) > 0 {
		number, kind, n := protowire.ConsumeTag(body)
		if n < 0 {
			return Challenge{}, protowire.ParseError(n)
		}
		body = body[n:]

		switch {
		case number == challengeAlgorithm && kind == protowire.VarintType:
			v, n := protowire.ConsumeVarint(body)
			if n < 0 {
				return Challenge{}, protowire.ParseError(n)
			}
			c.Algorithm, body = int(v), body[n:]
		case number == challengeHash && kind == protowire.VarintType:
			v, n := protowire.ConsumeVarint(body)
			if n < 0 {
				return Challenge{}, protowire.ParseError(n)
			}
			c.Hash, body = Hash(v), body[n:]
		case number == challengeNonce && kind == protowire.BytesType:
			v, n := protowire.ConsumeBytes(body)
			if n < 0 {
				return Challenge{}, protowire.ParseError(n)
			}
			c.Nonce, body = append([]byte(nil), v...), body[n:]
		default:
			n := protowire.ConsumeFieldValue(number, kind, body)
			if n < 0 {
				return Challenge{}, protowire.ParseError(n)
			}
			body = body[n:]
		}
	}
	return c, nil
}

// Answer is the response to a challenge, signed with the device key over the peer certificate, with
// the revocation list when there is one.
func Answer(creds *Credentials, h Hash, crl []byte) []byte {
	var r []byte
	r = protowire.AppendTag(r, responseSignature, protowire.BytesType)
	r = protowire.AppendBytes(r, creds.Signature(h))
	r = protowire.AppendTag(r, responseDevice, protowire.BytesType)
	r = protowire.AppendBytes(r, creds.Device)
	for _, ica := range creds.Intermediates {
		r = protowire.AppendTag(r, responseIntermediate, protowire.BytesType)
		r = protowire.AppendBytes(r, ica)
	}
	r = protowire.AppendTag(r, responseAlgorithm, protowire.VarintType)
	r = protowire.AppendVarint(r, PKCS1v15)
	r = protowire.AppendTag(r, responseHash, protowire.VarintType)
	r = protowire.AppendVarint(r, uint64(h))
	if len(crl) > 0 {
		r = protowire.AppendTag(r, responseCRL, protowire.BytesType)
		r = protowire.AppendBytes(r, crl)
	}

	var out []byte
	out = protowire.AppendTag(out, authResponse, protowire.BytesType)
	return protowire.AppendBytes(out, r)
}

// Refuse is an answer that carries an error instead of a response.
func Refuse(kind int) []byte {
	var e []byte
	e = protowire.AppendTag(e, errorType, protowire.VarintType)
	e = protowire.AppendVarint(e, uint64(kind))

	var out []byte
	out = protowire.AppendTag(out, authError, protowire.BytesType)
	return protowire.AppendBytes(out, e)
}

// field finds one length-delimited field at the top level of a message.
func field(b []byte, want protowire.Number) ([]byte, bool, error) {
	for len(b) > 0 {
		number, kind, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, false, protowire.ParseError(n)
		}
		b = b[n:]

		if number == want && kind == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, false, protowire.ParseError(n)
			}
			return v, true, nil
		}
		n = protowire.ConsumeFieldValue(number, kind, b)
		if n < 0 {
			return nil, false, protowire.ParseError(n)
		}
		b = b[n:]
	}
	return nil, false, nil
}
