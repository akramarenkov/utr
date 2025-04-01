package utr

import (
	"fmt"
	"net/url"
	"sync"
)

// Keeps and resolves mappings of hostnames and paths to Unix sockets.
type Keeper struct {
	table sync.Map
}

// Adds mapping of hostname and path to Unix socket.
func (kpr *Keeper) AddPath(hostname, path string) error {
	if err := isValidHostname(hostname); err != nil {
		return err
	}

	if prev, exists := kpr.table.LoadOrStore(hostname, path); exists {
		if prev != path {
			return ErrHostnameAlreadyExists
		}
	}

	return nil
}

func isValidHostname(hostname string) error {
	origin := url.URL{
		Host: hostname,
	}

	if _, err := url.Parse(origin.String()); err != nil {
		return fmt.Errorf("%w: %w", ErrHostnameInvalid, err)
	}

	return nil
}

// Resolves path to Unix socket by hostname.
func (kpr *Keeper) LookupPath(hostname string) (string, error) {
	path, exists := kpr.table.Load(hostname)
	if !exists {
		return "", ErrPathNotFound
	}

	//nolint:revive,forcetypeassert // Value type is fully controlled
	return path.(string), nil
}
