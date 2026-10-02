package cast

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// Credentials are what this device proves itself with: the certificate it presents on 8009, and the
// device certificate chain whose key has signed it.
type Credentials struct {
	Peer          tls.Certificate
	Device        []byte
	Intermediates [][]byte

	SHA1, SHA256 []byte

	NotBefore, NotAfter time.Time

	// Remote is whether they came from an oracle rather than being made here.
	Remote bool
}

// Signature is the device key's signature over the peer certificate under the hash asked for.
func (c *Credentials) Signature(h Hash) []byte {
	if h == SHA256 {
		return c.SHA256
	}
	return c.SHA1
}

// PeerLife is how long a peer certificate made here is good for, matching what an oracle issues.
const PeerLife = 48 * time.Hour

// Authority makes credentials of our own: a root, an intermediate and a device certificate, kept for
// as long as the process runs, and a fresh peer certificate each time it is asked.
type Authority struct {
	device       *rsa.PrivateKey
	deviceDER    []byte
	intermediate []byte
}

type savedAuthority struct {
	Device       []byte `json:"device_key"`
	DeviceCert   []byte `json:"device_cert"`
	Intermediate []byte `json:"intermediate_cert"`
}

// Marshal is the authority as it is kept between runs.
func (a *Authority) Marshal() ([]byte, error) {
	return json.Marshal(savedAuthority{
		Device:       x509.MarshalPKCS1PrivateKey(a.device),
		DeviceCert:   a.deviceDER,
		Intermediate: a.intermediate,
	})
}

// LoadAuthority reads back what Marshal kept, if it was made for this name.
func LoadAuthority(data []byte, name string) (*Authority, error) {
	var s savedAuthority
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("cast: reading the kept authority: %w", err)
	}
	key, err := x509.ParsePKCS1PrivateKey(s.Device)
	if err != nil {
		return nil, fmt.Errorf("cast: reading the kept device key: %w", err)
	}
	cert, err := x509.ParseCertificate(s.DeviceCert)
	if err != nil {
		return nil, fmt.Errorf("cast: reading the kept device certificate: %w", err)
	}
	if cert.Subject.CommonName != name {
		return nil, fmt.Errorf("cast: the kept authority is for %q, not %q", cert.Subject.CommonName, name)
	}
	if !key.PublicKey.Equal(cert.PublicKey) {
		return nil, fmt.Errorf("cast: the kept device key does not match its certificate")
	}
	if _, err := x509.ParseCertificate(s.Intermediate); err != nil {
		return nil, fmt.Errorf("cast: reading the kept intermediate: %w", err)
	}
	return &Authority{device: key, deviceDER: s.DeviceCert, intermediate: s.Intermediate}, nil
}

// NewAuthority makes the chain. Its dates are loose enough that a clock not yet set does not matter.
func NewAuthority(name string) (*Authority, error) {
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("cast: making the root key: %w", err)
	}
	root := &x509.Certificate{
		Subject:               pkix.Name{CommonName: name + " root"},
		NotBefore:             from,
		NotAfter:              until,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := sign(root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	root, _ = x509.ParseCertificate(rootDER)

	icaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("cast: making the intermediate key: %w", err)
	}
	ica := &x509.Certificate{
		Subject:               pkix.Name{CommonName: name + " intermediate"},
		NotBefore:             from,
		NotAfter:              until,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	icaDER, err := sign(ica, root, &icaKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	ica, _ = x509.ParseCertificate(icaDER)

	deviceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("cast: making the device key: %w", err)
	}
	device := &x509.Certificate{
		Subject:   pkix.Name{CommonName: name},
		NotBefore: from,
		NotAfter:  until,
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	deviceDER, err := sign(device, ica, &deviceKey.PublicKey, icaKey)
	if err != nil {
		return nil, err
	}

	return &Authority{device: deviceKey, deviceDER: deviceDER, intermediate: icaDER}, nil
}

// Issue makes a peer certificate valid from an hour before now for PeerLife, and signs it.
func (a *Authority) Issue(name string, now time.Time) (*Credentials, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cast: making the peer key: %w", err)
	}

	from := now.Add(-time.Hour)
	peer := &x509.Certificate{
		Subject:     pkix.Name{CommonName: name},
		NotBefore:   from,
		NotAfter:    from.Add(PeerLife),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	device, _ := x509.ParseCertificate(a.deviceDER)
	der, err := sign(peer, device, &key.PublicKey, a.device)
	if err != nil {
		return nil, err
	}

	s1, s256, err := signatures(a.device, der)
	if err != nil {
		return nil, err
	}

	leaf, _ := x509.ParseCertificate(der)
	return &Credentials{
		Peer:          tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf},
		Device:        a.deviceDER,
		Intermediates: [][]byte{a.intermediate},
		SHA1:          s1,
		SHA256:        s256,
		NotBefore:     peer.NotBefore,
		NotAfter:      peer.NotAfter,
	}, nil
}

func sign(template, parent *x509.Certificate, pub any, key crypto.Signer) ([]byte, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cast: making a serial: %w", err)
	}
	template.SerialNumber = serial

	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, key)
	if err != nil {
		return nil, fmt.Errorf("cast: making %s: %w", template.Subject.CommonName, err)
	}
	return der, nil
}

func signatures(key *rsa.PrivateKey, peer []byte) ([]byte, []byte, error) {
	d1 := sha1.Sum(peer)
	s1, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, d1[:])
	if err != nil {
		return nil, nil, fmt.Errorf("cast: signing with sha1: %w", err)
	}
	d256 := sha256.Sum256(peer)
	s256, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, d256[:])
	if err != nil {
		return nil, nil, fmt.Errorf("cast: signing with sha256: %w", err)
	}
	return s1, s256, nil
}

// Oracle is what an oracle answers with. Every certificate, key and signature is base64 DER, and
// every time is Unix seconds.
type Oracle struct {
	Public       string `json:"public"`
	Private      string `json:"private"`
	Device       string `json:"device"`
	Intermediate string `json:"intermediate"`
	SHA1         string `json:"sha1"`
	SHA256       string `json:"sha256"`
	NotBefore    int64  `json:"notBefore"`
	NotAfter     int64  `json:"notAfter"`
	Now          int64  `json:"now"`
}

// ParseOracle reads an oracle's answer into credentials, and the oracle's clock.
func ParseOracle(body []byte) (*Credentials, time.Time, error) {
	var o Oracle
	if err := json.Unmarshal(body, &o); err != nil {
		return nil, time.Time{}, fmt.Errorf("cast: the oracle's answer: %w", err)
	}

	raw := map[string][]byte{}
	for name, v := range map[string]string{
		"public": o.Public, "private": o.Private, "device": o.Device,
		"intermediate": o.Intermediate, "sha1": o.SHA1, "sha256": o.SHA256,
	} {
		if v == "" {
			return nil, time.Time{}, fmt.Errorf("cast: the oracle's answer has no %s", name)
		}
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("cast: the oracle's %s: %w", name, err)
		}
		raw[name] = b
	}

	key, err := privateKey(raw["private"])
	if err != nil {
		return nil, time.Time{}, err
	}
	leaf, err := x509.ParseCertificate(raw["public"])
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("cast: the oracle's public certificate: %w", err)
	}

	return &Credentials{
		Peer:          tls.Certificate{Certificate: [][]byte{raw["public"]}, PrivateKey: key, Leaf: leaf},
		Device:        raw["device"],
		Intermediates: [][]byte{raw["intermediate"]},
		SHA1:          raw["sha1"],
		SHA256:        raw["sha256"],
		NotBefore:     time.Unix(o.NotBefore, 0),
		NotAfter:      time.Unix(o.NotAfter, 0),
		Remote:        true,
	}, time.Unix(o.Now, 0), nil
}

func privateKey(der []byte) (crypto.Signer, error) {
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("cast: the oracle's private key is neither PKCS#1 nor PKCS#8: %w", err)
	}
	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("cast: the oracle's private key is a %T", k)
	}
	return signer, nil
}
