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

func ExampleTransport() {
	const socketPath = "service.sock"

	message := []byte("example message")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}

	var router http.ServeMux

	router.HandleFunc(
		"/request/path",
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

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			fmt.Println("Server shutdown error:", err)
		}

		if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("Server has terminated abnormally:", err)
		}
	}()

	go func() {
		serverErr <- server.Serve(listener)
	}()

	var keeper utr.Keeper

	transport, err := utr.New(&keeper, http.DefaultTransport)
	if err != nil {
		panic(err)
	}

	if err := keeper.AddPath("service", socketPath); err != nil {
		panic(err)
	}

	http.DefaultClient.Transport = transport

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

	fmt.Println(
		"Is message sent by server equal to message received by client:",
		bytes.Equal(received, message),
	)
	// Output:
	// Response status code: 200
	// Is message sent by server equal to message received by client: true
}
