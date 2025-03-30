package utr

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

type Transport struct {
	base        *http.Transport
	resolver    Resolver
	schemeHTTP  string
	schemeHTTPS string

	dialer    net.Dialer
	tlsDialer tls.Dialer
}

func WithHTTPDefaultTransport() Adjuster {
	adj := func(trt *Transport) error {
		def, casted := http.DefaultTransport.(*http.Transport)
		if !casted {
			return ErrDefaultHTTPTransportInvalid
		}

		trt.base = def

		return nil
	}

	return adjust(adj)
}

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

func Register(opts ...Adjuster) error {
	trt := &Transport{}

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

	trt.tlsDialer = tls.Dialer{
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
	host, _, _ := net.SplitHostPort(addr)

	path, err := trt.resolver.LookupPath(host)
	if err != nil {
		return nil, err
	}

	return trt.dialer.DialContext(ctx, NetworkName, path)
}

func (trt *Transport) dialTLS(ctx context.Context, _, addr string) (net.Conn, error) {
	// There is no need and possibility to test the formation of the address in
	// the transport from the net/http package
	host, _, _ := net.SplitHostPort(addr)

	path, err := trt.resolver.LookupPath(host)
	if err != nil {
		return nil, err
	}

	return trt.tlsDialer.DialContext(ctx, NetworkName, path)
}
