package utr

import "errors"

var (
	ErrDefaultHTTPTransportInvalid = errors.New("default http transport is not an http transport")
	ErrHostnameAlreadyExists       = errors.New("hostname is already exists")
	ErrHostnameInvalid             = errors.New("hostname is invalid")
	ErrHTTPTransportEmpty          = errors.New("http transport is not specified")
	ErrPathNotFound                = errors.New("path not found")
	ErrResolverEmpty               = errors.New("resolver is not specified")
	ErrSchemeEmpty                 = errors.New("scheme is not specified")
	ErrSchemeInvalid               = errors.New("scheme is not valid")
	ErrSchemeNotRegistered         = errors.New("scheme not registered")
)
