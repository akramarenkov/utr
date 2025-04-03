package utr

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

// Unix socket transport.
type Transport struct {
	base        *http.Transport
	origin      *http.Transport
	resolver    Resolver
	schemeHTTP  string
	schemeHTTPS string

	dialer    *net.Dialer
	tlsDialer *tls.Dialer
}

// Sets the [http.DefaultTransport] as the upstream [http.Transport] for Unix socket
// transport.
func WithDefaultTransport() Adjuster {
	adj := func(trt *Transport) error {
		def, casted := http.DefaultTransport.(*http.Transport)
		if !casted {
			return ErrDefaultTransportInvalid
		}

		trt.origin = def

		return nil
	}

	return adjust(adj)
}

// Sets the specified transport as the upstream [http.Transport] for Unix socket
// transport.
func WithTransport(transport *http.Transport) Adjuster {
	adj := func(trt *Transport) error {
		if transport == nil {
			return ErrTransportEmpty
		}

		trt.origin = transport

		return nil
	}

	return adjust(adj)
}

// Sets URL scheme for operation HTTP via Unix socket.
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

	return adjust(adj)
}

// Sets URL scheme for operation HTTPS via Unix socket.
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

	return adjust(adj)
}

func New(resolver Resolver, opts ...Adjuster) (*Transport, error) {
	if resolver == nil {
		return nil, ErrResolverEmpty
	}

	trt := &Transport{
		resolver: resolver,

		dialer: &net.Dialer{},
	}

	for _, adj := range opts {
		if err := adj.adjust(trt); err != nil {
			return nil, err
		}
	}

	if trt.origin == nil {
		return nil, ErrTransportEmpty
	}

	if trt.schemeHTTP == "" {
		trt.schemeHTTP = DefaultSchemeHTTP
	}

	if trt.schemeHTTPS == "" {
		trt.schemeHTTPS = DefaultSchemeHTTPS
	}

	trt.base = trt.origin.Clone()

	trt.tlsDialer = &tls.Dialer{
		Config: trt.base.TLSClientConfig,
	}

	trt.base.DialContext = trt.dial
	trt.base.DialTLSContext = trt.dialTLS

	return trt, nil
}

// Creates new Unix socket transport and registers it for upstream [http.Transport].
//
// The [Resolver] of paths to Unix sockets by hostnames must be set. [Keeper] can
// be used as it.
//
// The upstream [http.Transport] must be set using [WithDefaultTransport] or
// [WithTransport] functions.
//
// If URL schemes for operation HTTP and HTTPS via Unix socket are not set using
// [WithSchemeHTTP] and [WithSchemeHTTPS] functions, then URL schemes
// [DefaultSchemeHTTP] and [DefaultSchemeHTTPS] will be used. Multiple Unix socket
// transports with the same URL schemes cannot be registered for one upstream
// [http.Transport].
func Register(resolver Resolver, opts ...Adjuster) error {
	trt, err := New(resolver, opts...)
	if err != nil {
		return err
	}

	if err := trt.register(trt.schemeHTTP); err != nil {
		return err
	}

	return trt.register(trt.schemeHTTPS)
}

func (trt *Transport) register(scheme string) error {
	errs := make(chan error, 1)

	func() {
		defer func() {
			if fault := recover(); fault != nil {
				errs <- fmt.Errorf("%w: %v", ErrSchemeNotRegistered, fault)
			}
		}()

		trt.origin.RegisterProtocol(scheme, trt)

		errs <- nil
	}()

	return <-errs
}

// Implements the [http.RoundTripper] interface.
func (trt *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != trt.schemeHTTP && req.URL.Scheme != trt.schemeHTTPS {
		return trt.origin.RoundTrip(req)
	}

	cloned := req.Clone(req.Context())

	trt.replaceScheme(cloned)

	return trt.base.RoundTrip(cloned)
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

	return trt.dialer.DialContext(ctx, NetworkName, path)
}

func (trt *Transport) dialTLS(ctx context.Context, _, addr string) (net.Conn, error) {
	// There is no need and possibility to test the formation of the address in
	// the transport from the net/http package
	hostname, _, _ := net.SplitHostPort(addr)

	path, err := trt.resolver.LookupPath(hostname)
	if err != nil {
		return nil, err
	}

	return trt.tlsDialer.DialContext(ctx, NetworkName, path)
}
