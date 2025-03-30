package utr_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/akramarenkov/utr"
)

func Example() {
	const (
		hostname    = "service"
		requestPath = "/request/path"
		socketPath  = "service.sock"
	)

	message := []byte("example message")

	if err := utr.Register(utr.WithHTTPDefaultTransport()); err != nil {
		panic(err)
	}

	if err := utr.AddPath(hostname, socketPath); err != nil {
		panic(err)
	}

	listener, err := net.Listen(utr.NetworkName, socketPath)
	if err != nil {
		panic(err)
	}

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

	faults := make(chan error)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.Shutdown(ctx)
		fmt.Println("Server shutdown error:", err)
		fmt.Println("Is server terminated normally:", errors.Is(<-faults, http.ErrServerClosed))
	}()

	go func() {
		faults <- server.Serve(listener)
	}()

	resp, err := http.Get("http+unix://service/request/path")
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println("Response status code:", resp.StatusCode)

	received, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println("Is received message equal to sent one:", bytes.Equal(received, message))
	// Output:
	// Response status code: 200
	// Is received message equal to sent one: true
	// Server shutdown error: <nil>
	// Is server terminated normally: true
}
