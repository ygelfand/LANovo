package cast

import (
	"crypto/tls"
	"errors"
)

// ErrNoCredentials is a handshake arriving before there is a certificate to present.
var ErrNoCredentials = errors.New("cast: no credentials yet")

// TLS is the server configuration a sender connects with, presenting whichever peer certificate is
// current at each handshake.
//
// TLS 1.2 is the floor: senders include phones and televisions that have not been updated in years.
func TLS(current func() *Credentials) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.NoClientCert,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			c := current()
			if c == nil {
				return nil, ErrNoCredentials
			}
			return &c.Peer, nil
		},
	}
}
