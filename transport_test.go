package utr

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithSchemeHTTP(t *testing.T) {
	trt := &Transport{}

	require.Error(t, WithSchemeHTTP("")(trt))
	require.Empty(t, trt.schemeHTTP)

	require.Error(t, WithSchemeHTTP("http")(trt))
	require.Empty(t, trt.schemeHTTP)

	require.NoError(t, WithSchemeHTTP("uhttp")(trt))
	require.Equal(t, "uhttp", trt.schemeHTTP)
}

func TestWithSchemeHTTPS(t *testing.T) {
	trt := &Transport{}

	require.Error(t, WithSchemeHTTPS("")(trt))
	require.Empty(t, trt.schemeHTTPS)

	require.Error(t, WithSchemeHTTPS("https")(trt))
	require.Empty(t, trt.schemeHTTPS)

	require.NoError(t, WithSchemeHTTPS("uhttps")(trt))
	require.Equal(t, "uhttps", trt.schemeHTTPS)
}

func TestNewBadResolver(t *testing.T) {
	trt, err := New(nil, &http.Transport{})
	require.Error(t, err)
	require.Nil(t, trt)
}

func TestNewBadTransport(t *testing.T) {
	trt, err := New(&Keeper{}, nil)
	require.Error(t, err)
	require.Nil(t, trt)

	var httpTransport *http.Transport

	trt, err = New(&Keeper{}, httpTransport)
	require.Error(t, err)
	require.Nil(t, trt)
}

func TestNewBadOpts(t *testing.T) {
	trt, err := New(&Keeper{}, &http.Transport{}, WithSchemeHTTP("http"))
	require.Error(t, err)
	require.Nil(t, trt)
}

func TestTransport(t *testing.T) {
	testTransportBase(t, testSocketPath, false)
	testTransportBase(t, filepath.Join(t.TempDir(), testSocketPath), false)
}

func TestTransportHTTP2(t *testing.T) {
	testTransportBase(t, filepath.Join(t.TempDir(), testSocketPath), true)
}

func testTransportBase(t *testing.T, socketPath string, useHTTP2 bool) {
	const requestPath = "/request/path"

	message := prepareMessage(t)

	var (
		router     http.ServeMux
		usedProtos sync.Map
	)

	router.HandleFunc(
		requestPath,
		func(w http.ResponseWriter, r *http.Request) {
			usedProtos.Store(r.Proto, struct{}{})

			_, _ = w.Write(message)
		},
	)

	server := &http.Server{
		Handler:     &router,
		ReadTimeout: time.Second,
	}

	var protos http.Protocols

	if useHTTP2 {
		protos.SetUnencryptedHTTP2(true)
		server.Protocols = &protos
	}

	serverErr := make(chan error)
	defer close(serverErr)

	listener, err := net.Listen(unixNetworkName, socketPath)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, server.Shutdown(t.Context()))
		require.Equal(t, http.ErrServerClosed, <-serverErr)

		if useHTTP2 {
			_, exists := usedProtos.Load(http2Proto)
			require.True(t, exists)

			_, exists = usedProtos.Load(http1Proto)
			require.False(t, exists)
		} else {
			_, exists := usedProtos.Load(http2Proto)
			require.False(t, exists)

			_, exists = usedProtos.Load(http1Proto)
			require.True(t, exists)
		}
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper Keeper

	require.NoError(t, keeper.AddPath(testHostname, socketPath))

	httpTransport := cloneDefaultHTTPTransport(t)

	if useHTTP2 {
		httpTransport.Protocols = &protos
	}

	trt, err := New(&keeper, httpTransport)
	require.NoError(t, err)

	client := &http.Client{
		Transport: trt,
	}

	requestURL := url.URL{
		Scheme: DefaultSchemeHTTP,
		Host:   testHostname,
		Path:   requestPath,
	}

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		requestURL.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	output, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, message, output)
	require.NoError(t, resp.Body.Close())

	requestURLNonexistentHostname := url.URL{
		Scheme: DefaultSchemeHTTP,
		Host:   testHostname + testHostname,
		Path:   requestPath,
	}

	request, err = http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		requestURLNonexistentHostname.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	//nolint:bodyclose // False positive
	resp, err = client.Do(request)
	require.Error(t, err)
	require.Nil(t, resp)

	client.CloseIdleConnections()
}

func TestTransportTLS(t *testing.T) {
	testTransportTLSBase(t, testSocketPath, false)
	testTransportTLSBase(t, filepath.Join(t.TempDir(), testSocketPath), false)
}

func TestTransportTLSHTTP2(t *testing.T) {
	testTransportTLSBase(t, filepath.Join(t.TempDir(), testSocketPath), true)
}

func testTransportTLSBase(t *testing.T, socketPath string, useHTTP2 bool) {
	const requestPath = "/request/path"

	message := prepareMessage(t)
	caPool, serverCerts, clientCerts := genTempPKI(t, socketPath)

	listenTLSConfig := &tls.Config{
		Certificates: serverCerts,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}

	if useHTTP2 {
		listenTLSConfig.NextProtos = []string{"h2"}
	}

	var (
		router     http.ServeMux
		usedProtos sync.Map
	)

	router.HandleFunc(
		requestPath,
		func(w http.ResponseWriter, r *http.Request) {
			usedProtos.Store(r.Proto, struct{}{})

			_, _ = w.Write(message)
		},
	)

	server := &http.Server{
		Handler:     &router,
		ReadTimeout: time.Second,
		TLSConfig:   listenTLSConfig,
	}

	serverErr := make(chan error)
	defer close(serverErr)

	listener, err := tls.Listen(unixNetworkName, socketPath, listenTLSConfig)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, server.Shutdown(t.Context()))
		require.Equal(t, http.ErrServerClosed, <-serverErr)

		if useHTTP2 {
			_, exists := usedProtos.Load(http2Proto)
			require.True(t, exists)

			_, exists = usedProtos.Load(http1Proto)
			require.False(t, exists)
		} else {
			_, exists := usedProtos.Load(http2Proto)
			require.False(t, exists)

			_, exists = usedProtos.Load(http1Proto)
			require.True(t, exists)
		}
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper Keeper

	require.NoError(t, keeper.AddPath(testHostname, socketPath))

	httpTransport := cloneDefaultHTTPTransport(t)

	httpTransport.TLSClientConfig = &tls.Config{
		Certificates: clientCerts,
		MinVersion:   tls.VersionTLS13,
		RootCAs:      caPool,
	}

	trt, err := New(&keeper, httpTransport)
	require.NoError(t, err)

	client := &http.Client{
		Transport: trt,
	}

	requestURL := url.URL{
		Scheme: DefaultSchemeHTTPS,
		Host:   testHostname,
		Path:   requestPath,
	}

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		requestURL.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	output, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, message, output)
	require.NoError(t, resp.Body.Close())

	requestURLNonexistentHostname := url.URL{
		Scheme: DefaultSchemeHTTPS,
		Host:   testHostname + testHostname,
		Path:   requestPath,
	}

	request, err = http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		requestURLNonexistentHostname.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	//nolint:bodyclose // False positive
	resp, err = client.Do(request)
	require.Error(t, err)
	require.Nil(t, resp)

	client.CloseIdleConnections()
}

func TestTransportPassthrough(t *testing.T) {
	const requestPath = "/request/path"

	message := prepareMessage(t)

	var router http.ServeMux

	router.HandleFunc(
		requestPath,
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(message)
		},
	)

	server := &http.Server{
		Handler:     &router,
		ReadTimeout: time.Second,
	}

	serverErr := make(chan error)
	defer close(serverErr)

	listener, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, server.Shutdown(t.Context()))
		require.Equal(t, http.ErrServerClosed, <-serverErr)
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper Keeper

	httpTransport := cloneDefaultHTTPTransport(t)

	trt, err := New(&keeper, httpTransport)
	require.NoError(t, err)

	client := &http.Client{
		Transport: trt,
	}

	requestURL := url.URL{
		Scheme: "http",
		Host:   listener.Addr().String(),
		Path:   requestPath,
	}

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		requestURL.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	output, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, message, output)
	require.NoError(t, resp.Body.Close())

	client.CloseIdleConnections()
}

func prepareMessage(t *testing.T) []byte {
	const messageSize = 1024

	message := make([]byte, messageSize)

	readded, err := rand.Read(message)
	require.NoError(t, err)
	require.Equal(t, messageSize, readded)

	return message
}

func cloneDefaultHTTPTransport(t *testing.T) *http.Transport {
	tr, casted := http.DefaultTransport.(*http.Transport)
	require.True(t, casted)

	return tr.Clone()
}

func genTempPKI(
	t *testing.T,
	path string,
) (*x509.CertPool, []tls.Certificate, []tls.Certificate) {
	const (
		certLifeTimeInDays = 1
		keySize            = 1024

		caSN     = 689023454
		clientSN = 689023455
		serverSN = 689023456
	)

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, certLifeTimeInDays)

	caTempl := &x509.Certificate{
		SerialNumber: big.NewInt(caSN),
		Subject: pkix.Name{
			CommonName: "Temporary CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	serverTempl := &x509.Certificate{
		SerialNumber: big.NewInt(serverSN),
		Subject: pkix.Name{
			CommonName: "Temporary server",
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		DNSNames:    []string{path},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	clientTempl := &x509.Certificate{
		SerialNumber: big.NewInt(clientSN),
		Subject: pkix.Name{
			CommonName: "Temporary client",
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	caPool, caKey := genCA(t, caTempl, keySize)
	serverCert := genNodeTLSCert(t, serverTempl, keySize, caTempl, caKey)
	clientCert := genNodeTLSCert(t, clientTempl, keySize, caTempl, caKey)

	return caPool, []tls.Certificate{serverCert}, []tls.Certificate{clientCert}
}

func genCA(
	t *testing.T,
	templ *x509.Certificate,
	keySize int,
) (*x509.CertPool, *rsa.PrivateKey) {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	require.NoError(t, err)

	cert, err := x509.CreateCertificate(rand.Reader, templ, templ, &key.PublicKey, key)
	require.NoError(t, err)

	x509Cert, err := x509.ParseCertificate(cert)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	return pool, key
}

func genNodeTLSCert(
	t *testing.T,
	nodeTempl *x509.Certificate,
	keySize int,
	caTempl *x509.Certificate,
	caKey *rsa.PrivateKey,
) tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	require.NoError(t, err)

	cert, err := x509.CreateCertificate(rand.Reader, nodeTempl, caTempl, &key.PublicKey, caKey)
	require.NoError(t, err)

	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  key,
	}

	return tlsCert
}
