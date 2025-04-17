package utr

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

// Provides adjusting of a Unix domain socket transport.
type Adjuster func(t *Transport) error

// Unix domain socket transport.
type Transport struct {
	base        *http.Transport
	resolver    Resolver
	schemeHTTP  string
	schemeHTTPS string
	upstream    *http.Transport

	dialer    *net.Dialer
	tlsDialer *tls.Dialer
}

// Sets URL scheme for operation HTTP via Unix domain socket.
func WithSchemeHTTP(scheme string) Adjuster {
	adj := func(trt *Transport) error {
		if scheme == "" {
			return ErrSchemeEmpty
		}

		if scheme == httpScheme {
			return fmt.Errorf("%w: %s", ErrSchemeInvalid, scheme)
		}

		trt.schemeHTTP = scheme

		return nil
	}

	return adj
}

// Sets URL scheme for operation HTTPS via Unix domain socket.
func WithSchemeHTTPS(scheme string) Adjuster {
	adj := func(trt *Transport) error {
		if scheme == "" {
			return ErrSchemeEmpty
		}

		if scheme == httpsScheme {
			return fmt.Errorf("%w: %s", ErrSchemeInvalid, scheme)
		}

		trt.schemeHTTPS = scheme

		return nil
	}

	return adj
}

// Creates new Unix domain socket transport with upstream [http.Transport]. For Unix domain socket
// schemes will be used a clone of an upstream [http.Transport], for other schemes
// an upstream [http.Transport] will be used directly.
//
// The [Resolver] of paths to Unix domain sockets by hostnames must be set. [Keeper] can
// be used as it.
//
// The upstream [http.Transport] must be set. [http.RoundTripper] used for convenience
// to work with [http.DefaultTransport].
//
// If URL schemes for operation HTTP and HTTPS via Unix domain socket are not set using
// [WithSchemeHTTP] and [WithSchemeHTTPS] functions, then URL schemes
// [DefaultSchemeHTTP] and [DefaultSchemeHTTPS] will be used.
func New(resolver Resolver, upstream http.RoundTripper, opts ...Adjuster) (*Transport, error) {
	if resolver == nil {
		return nil, ErrResolverEmpty
	}

	trt := &Transport{
		resolver: resolver,

		dialer: &net.Dialer{},
	}

	if err := trt.setUpstream(upstream); err != nil {
		return nil, err
	}

	for _, adj := range opts {
		if err := adj(trt); err != nil {
			return nil, err
		}
	}

	if trt.schemeHTTP == "" {
		trt.schemeHTTP = DefaultSchemeHTTP
	}

	if trt.schemeHTTPS == "" {
		trt.schemeHTTPS = DefaultSchemeHTTPS
	}

	trt.base = trt.upstream.Clone()

	trt.tlsDialer = &tls.Dialer{
		Config: trt.base.TLSClientConfig,
	}

	trt.base.DialContext = trt.dial
	trt.base.DialTLSContext = trt.dialTLS

	return trt, nil
}

func (trt *Transport) setUpstream(upstream http.RoundTripper) error {
	httpTransport, casted := upstream.(*http.Transport)
	if !casted {
		return ErrTransportInvalid
	}

	if httpTransport == nil {
		return ErrTransportEmpty
	}

	trt.upstream = httpTransport

	return nil
}

// Implements the [http.RoundTripper] interface.
func (trt *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != trt.schemeHTTP && req.URL.Scheme != trt.schemeHTTPS {
		return trt.upstream.RoundTrip(req)
	}

	cloned := req.Clone(req.Context())

	trt.replaceScheme(cloned)

	return trt.base.RoundTrip(cloned)
}

// Like the [http.Transport.CloseIdleConnections].
func (trt *Transport) CloseIdleConnections() {
	trt.base.CloseIdleConnections()
	trt.upstream.CloseIdleConnections()
}

func (trt *Transport) replaceScheme(req *http.Request) {
	switch req.URL.Scheme {
	case trt.schemeHTTP:
		req.URL.Scheme = httpScheme
	case trt.schemeHTTPS:
		req.URL.Scheme = httpsScheme
	}
}

func (trt *Transport) dial(ctx context.Context, _, addr string) (net.Conn, error) {
	// There is no need and possibility to test the formation of the address in
	// the transport from the net/http package
	hostname, _, _ := net.SplitHostPort(addr)

	path, err := trt.resolver.LookupPath(hostname)
	if err != nil {
		return nil, err
	}

	return trt.dialer.DialContext(ctx, unixNetworkName, path)
}

func (trt *Transport) dialTLS(ctx context.Context, _, addr string) (net.Conn, error) {
	// There is no need and possibility to test the formation of the address in
	// the transport from the net/http package
	hostname, _, _ := net.SplitHostPort(addr)

	path, err := trt.resolver.LookupPath(hostname)
	if err != nil {
		return nil, err
	}

	return trt.tlsDialer.DialContext(ctx, unixNetworkName, path)
}
