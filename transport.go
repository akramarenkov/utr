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
	resolver    Resolver
	schemeHTTP  string
	schemeHTTPS string

	dialer    *net.Dialer
	tlsDialer *tls.Dialer
}

// Sets the [http.DefaultTransport] as the upstream [http.Transport] for Unix socket
// transport.
func WithHTTPDefaultTransport() Adjuster {
	adj := func(trt *Transport) error {
		def, casted := http.DefaultTransport.(*http.Transport)
		if !casted {
			return ErrHTTPDefaultTransportInvalid
		}

		trt.base = def

		return nil
	}

	return adjust(adj)
}

// Sets the specified transport as the upstream [http.Transport] for Unix socket
// transport.
func WithHTTPTransport(transport *http.Transport) Adjuster {
	adj := func(trt *Transport) error {
		if transport == nil {
			return ErrHTTPTransportEmpty
		}

		trt.base = transport

		return nil
	}

	return adjust(adj)
}

// Sets path by hostname resolver for Unix socket transport.
func WithResolver(resolver Resolver) Adjuster {
	adj := func(trt *Transport) error {
		if resolver == nil {
			return ErrResolverEmpty
		}

		trt.resolver = resolver

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

// Creates and registers new Unix socket transport.
//
// The upstream [http.Transport] must be set using [WithHTTPDefaultTransport] or
// [WithHTTPTransport] functions.
//
// If the path by hostname resolver is not set using [WithResolver] function the global
// [Keeper] will be used. In this case, the mapping of the hostname and path to the Unix
// socket can be added using the [AddPath] function.
//
// If URL schemes for operation HTTP and HTTPS via Unix socket are not set using
// [WithSchemeHTTP] and [WithSchemeHTTPS] functions, then URL schemes
// [DefaultSchemeHTTP] and [DefaultSchemeHTTPS] will be used. Multiple Unix socket
// transports with the same URL schemes cannot be registered for one upstream
// [http.Transport].
func Register(opts ...Adjuster) error {
	trt := &Transport{
		dialer: &net.Dialer{},
	}

	for _, adj := range opts {
		if err := adj.adjust(trt); err != nil {
			return err
		}
	}

	if trt.base == nil {
		return ErrHTTPTransportEmpty
	}

	if trt.resolver == nil {
		trt.resolver = defaultKeeper
	}

	if trt.schemeHTTP == "" {
		trt.schemeHTTP = DefaultSchemeHTTP
	}

	if trt.schemeHTTPS == "" {
		trt.schemeHTTPS = DefaultSchemeHTTPS
	}

	trt.tlsDialer = &tls.Dialer{
		Config: trt.base.TLSClientConfig,
	}

	if err := trt.register(trt.schemeHTTP); err != nil {
		return err
	}

	if err := trt.register(trt.schemeHTTPS); err != nil {
		return err
	}

	trt.base = trt.base.Clone()
	trt.base.DialContext = trt.dial
	trt.base.DialTLSContext = trt.dialTLS

	return nil
}

func (trt *Transport) register(scheme string) error {
	errs := make(chan error, 1)

	func() {
		defer func() {
			if fault := recover(); fault != nil {
				errs <- fmt.Errorf("%w: %v", ErrSchemeNotRegistered, fault)
			}
		}()

		trt.base.RegisterProtocol(scheme, trt)

		errs <- nil
	}()

	return <-errs
}

// Implements the [http.RoundTripper] interface.
func (trt *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())

	trt.replaceScheme(cloned)

	return trt.base.RoundTrip(cloned)
}

func (trt *Transport) replaceScheme(request *http.Request) {
	switch request.URL.Scheme {
	case trt.schemeHTTP:
		request.URL.Scheme = httpScheme
	case trt.schemeHTTPS:
		request.URL.Scheme = httpsScheme
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
