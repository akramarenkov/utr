package utr

import "errors"

var (
	ErrDefaultTransportInvalid = errors.New("default transport from net/http package is not a http transport")
	ErrHostnameAlreadyExists   = errors.New("hostname is already exists")
	ErrHostnameInvalid         = errors.New("hostname is invalid")
	ErrPathNotFound            = errors.New("path not found")
	ErrResolverEmpty           = errors.New("resolver is not specified")
	ErrSchemeEmpty             = errors.New("scheme is not specified")
	ErrSchemeInvalid           = errors.New("scheme is not valid")
	ErrSchemeNotRegistered     = errors.New("scheme is not registered")
	ErrTransportEmpty          = errors.New("upstream transport is not specified")
)
