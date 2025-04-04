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

func TestWithTransport(t *testing.T) {
	trt := &Transport{}

	require.Error(t, WithTransport(nil).adjust(trt))
	require.Nil(t, trt.origin)

	var httpTransport *http.Transport

	require.Error(t, WithTransport(httpTransport).adjust(trt))
	require.Nil(t, trt.origin)

	require.NoError(t, WithTransport(&http.Transport{}).adjust(trt))
	require.NotNil(t, trt.origin)
}

func TestWithSchemeHTTP(t *testing.T) {
	trt := &Transport{}

	require.Error(t, WithSchemeHTTP("").adjust(trt))
	require.Empty(t, trt.schemeHTTP)

	require.Error(t, WithSchemeHTTP("http").adjust(trt))
	require.Empty(t, trt.schemeHTTP)

	require.NoError(t, WithSchemeHTTP("uhttp").adjust(trt))
	require.Equal(t, "uhttp", trt.schemeHTTP)
}

func TestWithSchemeHTTPS(t *testing.T) {
	trt := &Transport{}

	require.Error(t, WithSchemeHTTPS("").adjust(trt))
	require.Empty(t, trt.schemeHTTPS)

	require.Error(t, WithSchemeHTTPS("https").adjust(trt))
	require.Empty(t, trt.schemeHTTPS)

	require.NoError(t, WithSchemeHTTPS("uhttps").adjust(trt))
	require.Equal(t, "uhttps", trt.schemeHTTPS)
}

func TestNewBadResolver(t *testing.T) {
	trt, err := New(nil)
	require.Error(t, err)
	require.Nil(t, trt)
}

func TestNewBadTransport(t *testing.T) {
	trt, err := New(&Keeper{})
	require.Error(t, err)
	require.Nil(t, trt)

	trt, err = New(&Keeper{}, WithTransport(nil))
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

	listener, err := net.Listen(NetworkName, socketPath)
	require.NoError(t, err)

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

	serverFaults := make(chan error)
	defer close(serverFaults)

	defer func() {
		require.NoError(t, server.Shutdown(t.Context()))
		require.Equal(t, http.ErrServerClosed, <-serverFaults)

		usedProtos.Range(
			func(key any, _ any) bool {
				if useHTTP2 {
					require.Equal(t, http2Proto, key)
				} else {
					require.Equal(t, http1Proto, key)
				}

				return true
			},
		)
	}()

	go func() {
		serverFaults <- server.Serve(listener)
	}()

	httpTransport := cloneDefaultHTTPTransport(t)

	if useHTTP2 {
		httpTransport.Protocols = &protos
	}

	var keeper Keeper

	require.NoError(t, keeper.AddPath(testHostname, socketPath))

	trt, err := New(&keeper, WithTransport(httpTransport))
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

	listener, err := tls.Listen(NetworkName, socketPath, listenTLSConfig)
	require.NoError(t, err)

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

	serverFaults := make(chan error)
	defer close(serverFaults)

	defer func() {
		require.NoError(t, server.Shutdown(t.Context()))
		require.Equal(t, http.ErrServerClosed, <-serverFaults)

		usedProtos.Range(
			func(key any, _ any) bool {
				if useHTTP2 {
					require.Equal(t, http2Proto, key)
				} else {
					require.Equal(t, http1Proto, key)
				}

				return true
			},
		)
	}()

	go func() {
		serverFaults <- server.Serve(listener)
	}()

	httpTransport := cloneDefaultHTTPTransport(t)

	httpTransport.TLSClientConfig = &tls.Config{
		Certificates: clientCerts,
		MinVersion:   tls.VersionTLS13,
		RootCAs:      caPool,
	}

	var keeper Keeper

	require.NoError(t, keeper.AddPath(testHostname, socketPath))

	trt, err := New(&keeper, WithTransport(httpTransport))
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
