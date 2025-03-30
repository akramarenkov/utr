package utr

import (
	"fmt"
	"net/url"
	"sync"
)

// Keeper using [sync.Map].
type MapKeeper struct {
	table sync.Map
}

// Adds mapping of hostname and path to Unix socket.
func (mkr *MapKeeper) AddPath(hostname, path string) error {
	if err := isValidHostname(hostname); err != nil {
		return err
	}

	if prev, exists := mkr.table.LoadOrStore(hostname, path); exists {
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
func (mkr *MapKeeper) LookupPath(hostname string) (string, error) {
	path, exists := mkr.table.Load(hostname)
	if !exists {
		return "", ErrPathNotFound
	}

	//nolint:revive,forcetypeassert // Value type is fully controlled
	return path.(string), nil
}
