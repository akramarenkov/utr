package utr

import (
	"fmt"
	"net/url"
	"sync"
)

type MapKeeper struct {
	table sync.Map
}

func (mkr *MapKeeper) AddPath(host, path string) error {
	if err := isValidHost(host); err != nil {
		return err
	}

	if prev, exists := mkr.table.LoadOrStore(host, path); exists {
		if prev != path {
			return ErrHostAlreadyExists
		}
	}

	return nil
}

func isValidHost(host string) error {
	origin := url.URL{
		Host: host,
	}

	if _, err := url.Parse(origin.String()); err != nil {
		return fmt.Errorf("%w: %w", ErrHostInvalid, err)
	}

	return nil
}

func (mkr *MapKeeper) LookupPath(host string) (string, error) {
	path, exists := mkr.table.Load(host)
	if !exists {
		return "", ErrPathNotFound
	}

	//nolint:revive,forcetypeassert // Value type is fully controlled
	return path.(string), nil
}
