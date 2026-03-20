package slack

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

// tlsClientIDs maps auth source to a uTLS ClientHelloID that mimics
// the real TLS fingerprint of that client.
var tlsClientIDs = map[AuthSource]*utls.ClientHelloID{
	SourceDesktop: &utls.HelloChrome_120, // Electron uses Chromium TLS
	SourceChrome:  &utls.HelloChrome_120,
	SourceBrave:   &utls.HelloChrome_120,
	SourceFirefox: &utls.HelloFirefox_120,
}

// newHTTPClient creates an http.Client with a TLS fingerprint matching the auth source.
func newHTTPClient(source AuthSource) *http.Client {
	helloID, ok := tlsClientIDs[source]
	if !ok {
		helloID = &utls.HelloChrome_120
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: newUTLSTransport(helloID),
	}
}

type utlsTransport struct {
	helloID *utls.ClientHelloID
	inner   *http.Transport
}

func newUTLSTransport(helloID *utls.ClientHelloID) *utlsTransport {
	t := &utlsTransport{helloID: helloID}
	t.inner = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		DialTLSContext: func(_ context.Context, network, addr string) (net.Conn, error) {
			return t.dialUTLS(network, addr)
		},
	}
	return t
}

func (t *utlsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.inner.RoundTrip(req)
}

func (t *utlsTransport) dialUTLS(network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	conn, err := net.DialTimeout(network, addr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	// Build a custom spec from the preset, then force ALPN to http/1.1
	// Go's http.Transport cannot handle h2 over a raw net.Conn from uTLS.
	spec, err := utls.UTLSIdToSpec(*t.helloID)
	if err != nil {
		conn.Close()
		return nil, err
	}
	for i, ext := range spec.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
			spec.Extensions[i] = alpn
			break
		}
	}

	config := &utls.Config{ServerName: host}
	tlsConn := utls.UClient(conn, config, utls.HelloCustom)
	if err := tlsConn.ApplyPreset(&spec); err != nil {
		conn.Close()
		return nil, err
	}
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}
