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

	serverErr := make(chan error)
	defer close(serverErr)

	var blank net.ListenConfig

	listener, err := blank.Listen(b.Context(), unixNetworkName, testSocketPath)
	require.NoError(b, err)

	defer func() {
		require.NoError(b, server.Shutdown(b.Context()))
		require.Equal(b, http.ErrServerClosed, <-serverErr)
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper Keeper

	require.NoError(b, keeper.AddPath(testHostname, testSocketPath))

	httpTransport := cloneDefaultHTTPTransportBench(b)

	trt, err := New(&keeper, httpTransport)
	require.NoError(b, err)

	client := &http.Client{
		Transport: trt,
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

		if err := resp.Body.Close(); err != nil {
			require.NoError(b, err)
		}

		if resp.StatusCode != http.StatusOK {
			require.Equal(b, http.StatusOK, resp.StatusCode)
		}
	}
}

func BenchmarkRaceTransport(b *testing.B) {
	const requestPath = "/request/path"

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

	serverErr := make(chan error)
	defer close(serverErr)

	var blank net.ListenConfig

	listener, err := blank.Listen(b.Context(), unixNetworkName, testSocketPath)
	require.NoError(b, err)

	defer func() {
		require.NoError(b, server.Shutdown(b.Context()))
		require.Equal(b, http.ErrServerClosed, <-serverErr)
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper Keeper

	require.NoError(b, keeper.AddPath(testHostname, testSocketPath))

	httpTransport := cloneDefaultHTTPTransportBench(b)

	trt, err := New(&keeper, httpTransport)
	require.NoError(b, err)

	client := &http.Client{
		Transport: trt,
	}

	requestURL := url.URL{
		Scheme: DefaultSchemeHTTP,
		Host:   testHostname,
		Path:   requestPath,
	}

	requestURLString := requestURL.String()

	b.ResetTimer()

	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
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

				if err := resp.Body.Close(); err != nil {
					require.NoError(b, err)
				}

				if resp.StatusCode != http.StatusOK {
					require.Equal(b, http.StatusOK, resp.StatusCode)
				}
			}
		},
	)
}

func cloneDefaultHTTPTransportBench(b *testing.B) *http.Transport {
	tr, casted := http.DefaultTransport.(*http.Transport)
	require.True(b, casted)

	return tr.Clone()
}
