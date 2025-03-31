package utr

import (
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func BenchmarkTransport(b *testing.B) {
	const requestPath = "/request/path"

	listener, err := net.Listen(NetworkName, testSocketPath)
	require.NoError(b, err)

	var router http.ServeMux

	router.HandleFunc(
		requestPath,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	)

	server := &http.Server{
		Handler:     &router,
		ReadTimeout: time.Second,
	}

	faults := make(chan error)
	defer close(faults)

	defer func() {
		require.NoError(b, server.Shutdown(b.Context()))
		require.Equal(b, http.ErrServerClosed, <-faults)
	}()

	go func() {
		faults <- server.Serve(listener)
	}()

	httpTransport := cloneDefaultHTTPTransportBench(b)

	require.NoError(b, AddPath(testHostname, testSocketPath))
	require.NoError(b, Register(WithHTTPTransport(httpTransport)))

	client := &http.Client{
		Transport: httpTransport,
	}

	requestURL := url.URL{
		Scheme: DefaultSchemeHTTP,
		Host:   testHostname,
		Path:   requestPath,
	}

	requestURLString := requestURL.String()

	for b.Loop() {
		request, err := http.NewRequestWithContext(
			b.Context(),
			http.MethodGet,
			requestURLString,
			http.NoBody,
		)
		if err != nil {
			require.NoError(b, err)
		}

		resp, err := client.Do(request)
		if err != nil {
			require.NoError(b, err)
		}

		if resp.StatusCode != http.StatusOK {
			require.Equal(b, http.StatusOK, resp.StatusCode)
		}

		if err := resp.Body.Close(); err != nil {
			require.NoError(b, err)
		}
	}
}

func cloneDefaultHTTPTransportBench(b *testing.B) *http.Transport {
	tr, casted := http.DefaultTransport.(*http.Transport)
	require.True(b, casted)

	return tr.Clone()
}
