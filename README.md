# UTR

[![Go Reference](https://pkg.go.dev/badge/github.com/akramarenkov/utr.svg)](https://pkg.go.dev/github.com/akramarenkov/utr)
[![Go Report Card](https://goreportcard.com/badge/github.com/akramarenkov/utr)](https://goreportcard.com/report/github.com/akramarenkov/utr)
[![Coverage Status](https://coveralls.io/repos/github/akramarenkov/utr/badge.svg)](https://coveralls.io/github/akramarenkov/utr)

## Purpose

Library that provides HTTP client operation from the standard library via Unix socket

## Usage

Example:

```go
package main

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

func main() {
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

    faults := make(chan error)
    defer close(faults)

    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            fmt.Println("Server shutdown error:", err)
        }

        if err := <-faults; !errors.Is(err, http.ErrServerClosed) {
            fmt.Println("Server has terminated abnormally:", err)
        }
    }()

    go func() {
        faults <- server.Serve(listener)
    }()

    var keeper utr.Keeper

    if err := utr.Register(&keeper, utr.WithDefaultTransport()); err != nil {
        panic(err)
    }

    if err := keeper.AddPath("service", socketPath); err != nil {
        panic(err)
    }

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
```
