package cast

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

var (
	authorityOnce sync.Once
	authority     *Authority
	authorityErr  error
)

func issued(t *testing.T) *Credentials {
	t.Helper()
	authorityOnce.Do(func() { authority, authorityErr = NewAuthority("Kitchen") })
	if authorityErr != nil {
		t.Fatal(authorityErr)
	}
	c, err := authority.Issue("Kitchen", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func challenge(algorithm int, h Hash, nonce []byte) []byte {
	var c []byte
	c = protowire.AppendTag(c, challengeAlgorithm, protowire.VarintType)
	c = protowire.AppendVarint(c, uint64(algorithm))
	if nonce != nil {
		c = protowire.AppendTag(c, challengeNonce, protowire.BytesType)
		c = protowire.AppendBytes(c, nonce)
	}
	c = protowire.AppendTag(c, challengeHash, protowire.VarintType)
	c = protowire.AppendVarint(c, uint64(h))

	var out []byte
	out = protowire.AppendTag(out, authChallenge, protowire.BytesType)
	return protowire.AppendBytes(out, c)
}

type response struct {
	signature, device, crl []byte
	intermediates          [][]byte
	hash                   Hash
	errorType              int
	refused                bool
}

func readAnswer(t *testing.T, b []byte) response {
	t.Helper()
	var r response
	if e, ok, _ := field(b, authError); ok {
		r.refused = true
		if v, ok := varint(e, errorType); ok {
			r.errorType = int(v)
		}
		return r
	}
	body, ok, err := field(b, authResponse)
	if err != nil || !ok {
		t.Fatalf("no response in the answer: %v", err)
	}
	for len(body) > 0 {
		number, kind, n := protowire.ConsumeTag(body)
		body = body[n:]
		switch kind {
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(body)
			body = body[n:]
			switch number {
			case responseSignature:
				r.signature = v
			case responseDevice:
				r.device = v
			case responseIntermediate:
				r.intermediates = append(r.intermediates, v)
			case responseCRL:
				r.crl = v
			}
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(body)
			body = body[n:]
			if number == responseHash {
				r.hash = Hash(v)
			}
		}
	}
	return r
}

func varint(b []byte, want protowire.Number) (uint64, bool) {
	for len(b) > 0 {
		number, kind, n := protowire.ConsumeTag(b)
		b = b[n:]
		if number == want && kind == protowire.VarintType {
			v, _ := protowire.ConsumeVarint(b)
			return v, true
		}
		b = b[protowire.ConsumeFieldValue(number, kind, b):]
	}
	return 0, false
}

func TestAChallengeReadsBack(t *testing.T) {
	c, err := ParseChallenge(challenge(PKCS1v15, SHA256, []byte("nonce")))
	if err != nil {
		t.Fatal(err)
	}
	if c.Algorithm != PKCS1v15 || c.Hash != SHA256 || string(c.Nonce) != "nonce" {
		t.Errorf("read back %+v", c)
	}
}

func TestAnEmptyChallengeIsPKCS1WithSHA1(t *testing.T) {
	var out []byte
	out = protowire.AppendTag(out, authChallenge, protowire.BytesType)
	out = protowire.AppendBytes(out, nil)

	c, err := ParseChallenge(out)
	if err != nil {
		t.Fatal(err)
	}
	if c.Algorithm != PKCS1v15 || c.Hash != SHA1 {
		t.Errorf("read back %+v", c)
	}
}

func TestTheAnswerVerifiesAgainstTheDeviceCertificate(t *testing.T) {
	creds := issued(t)
	peer := creds.Peer.Certificate[0]

	for _, h := range []Hash{SHA1, SHA256} {
		r := NewReceiver("Kitchen")
		r.Credentials = func() *Credentials { return creds }

		out, err := r.Receive(Message{
			Source: SenderID, Destination: ReceiverID, Namespace: NSDeviceAuth,
			Binary: challenge(PKCS1v15, h, nil),
		})
		if err != nil || len(out) != 1 {
			t.Fatalf("%s: %d answers, %v", h, len(out), err)
		}
		got := readAnswer(t, out[0].Binary)
		if got.refused {
			t.Fatalf("%s: refused with %d", h, got.errorType)
		}
		if got.hash != h {
			t.Errorf("%s: the answer says %s", h, got.hash)
		}

		device, err := x509.ParseCertificate(got.device)
		if err != nil {
			t.Fatalf("%s: the device certificate: %v", h, err)
		}
		key := device.PublicKey.(*rsa.PublicKey)

		var digest []byte
		kind := crypto.SHA1
		if h == SHA256 {
			d := sha256.Sum256(peer)
			digest, kind = d[:], crypto.SHA256
		} else {
			d := sha1.Sum(peer)
			digest = d[:]
		}
		if err := rsa.VerifyPKCS1v15(key, kind, digest, got.signature); err != nil {
			t.Errorf("%s: the signature does not verify over the peer certificate: %v", h, err)
		}

		ica, err := x509.ParseCertificate(got.intermediates[0])
		if err != nil {
			t.Fatalf("%s: the intermediate: %v", h, err)
		}
		if err := device.CheckSignatureFrom(ica); err != nil {
			t.Errorf("%s: the device certificate does not chain to the intermediate: %v", h, err)
		}
	}
}

func TestTheAnswerCarriesTheRevocationListWhenThereIsOne(t *testing.T) {
	creds := issued(t)
	r := NewReceiver("Kitchen")
	r.Credentials = func() *Credentials { return creds }

	ask := func() response {
		out, _ := r.Receive(Message{
			Source: SenderID, Destination: ReceiverID, Namespace: NSDeviceAuth,
			Binary: challenge(PKCS1v15, SHA256, nil),
		})
		return readAnswer(t, out[0].Binary)
	}

	if got := ask(); got.crl != nil {
		t.Errorf("a revocation list came back with none set: %d bytes", len(got.crl))
	}
	r.CRL = func() []byte { return []byte("revoked") }
	if got := ask(); string(got.crl) != "revoked" {
		t.Errorf("the revocation list came back as %q", got.crl)
	}
}

func TestNoCredentialsIsRefused(t *testing.T) {
	r := NewReceiver("Kitchen")
	out, err := r.Receive(Message{
		Source: SenderID, Destination: ReceiverID, Namespace: NSDeviceAuth,
		Binary: challenge(PKCS1v15, SHA256, nil),
	})
	if err != nil || len(out) != 1 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if got := readAnswer(t, out[0].Binary); !got.refused || got.errorType != AuthInternal {
		t.Errorf("answered %+v", got)
	}
}

func TestPSSIsRefused(t *testing.T) {
	creds := issued(t)
	r := NewReceiver("Kitchen")
	r.Credentials = func() *Credentials { return creds }

	out, _ := r.Receive(Message{
		Source: SenderID, Destination: ReceiverID, Namespace: NSDeviceAuth,
		Binary: challenge(PSS, SHA256, nil),
	})
	if got := readAnswer(t, out[0].Binary); !got.refused || got.errorType != AuthNoAlgorithm {
		t.Errorf("answered %+v", got)
	}
}

func TestTheIssuedPeerCertificateIsSignedByTheDeviceKey(t *testing.T) {
	creds := issued(t)
	device, _ := x509.ParseCertificate(creds.Device)
	peer := creds.Peer.Leaf
	if err := device.CheckSignature(peer.SignatureAlgorithm, peer.RawTBSCertificate, peer.Signature); err != nil {
		t.Errorf("the peer certificate: %v", err)
	}
	if got := creds.NotAfter.Sub(creds.NotBefore); got != PeerLife {
		t.Errorf("the peer certificate lives %v", got)
	}
}

func TestAnOracleAnswerReadsBack(t *testing.T) {
	creds := issued(t)
	key, err := x509.MarshalPKCS8PrivateKey(creds.Peer.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	b64 := base64.StdEncoding.EncodeToString
	body, _ := json.Marshal(Oracle{
		Public:       b64(creds.Peer.Certificate[0]),
		Private:      b64(key),
		Device:       b64(creds.Device),
		Intermediate: b64(creds.Intermediates[0]),
		SHA1:         b64(creds.SHA1),
		SHA256:       b64(creds.SHA256),
		NotBefore:    1790380800,
		NotAfter:     1790553600,
		Now:          1790400000,
	})

	got, now, err := ParseOracle(body)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Remote {
		t.Error("an oracle's answer is not marked remote")
	}
	if got.NotAfter.Unix() != 1790553600 || now.Unix() != 1790400000 {
		t.Errorf("times read back as %v and %v", got.NotAfter, now)
	}
	if string(got.SHA256) != string(creds.SHA256) || got.Peer.Leaf == nil {
		t.Error("the answer did not read back whole")
	}
}

func TestAnOracleKeyMayBePKCS1(t *testing.T) {
	issued(t)
	k := authority.device
	got, err := privateKey(x509.MarshalPKCS1PrivateKey(k))
	if err != nil {
		t.Fatal(err)
	}
	if !got.(*rsa.PrivateKey).Equal(k) {
		t.Error("a PKCS#1 key read back as a different key")
	}
}
