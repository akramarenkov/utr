package utr

import "errors"

var (
	ErrHostnameAlreadyExists = errors.New("hostname is already exists")
	ErrHostnameInvalid       = errors.New("hostname is invalid")
	ErrPathNotFound          = errors.New("path not found")
	ErrResolverEmpty         = errors.New("resolver is not specified")
	ErrSchemeEmpty           = errors.New("scheme is not specified")
	ErrSchemeInvalid         = errors.New("scheme is not valid")
	ErrTransportEmpty        = errors.New("upstream transport is not specified")
	ErrTransportInvalid      = errors.New("upstream transport is not a transport from net/http package")
)
