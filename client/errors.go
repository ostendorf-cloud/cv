package client

import (
	"errors"
	"fmt"
)

// The errors below never carry the base URL, an ETag or any part of a
// response body in their message. A typed error holds the few values a caller
// needs, such as a status code, as fields.

var (
	// ErrConfig is wrapped by the error New returns for an invalid option.
	ErrConfig = errors.New("client: invalid configuration")

	// ErrInvalidETag is returned by Get for an ETag that cannot be sent as a
	// header value.
	ErrInvalidETag = errors.New("client: invalid etag")

	// ErrTransport is returned when the request could not be sent or the
	// response could not be read, for a reason other than the caller's
	// context or a timeout. The cause is dropped on purpose: a dial error
	// names the host.
	ErrTransport = errors.New("client: request failed")

	// ErrTooLarge is returned when the response body is larger than the
	// client reads.
	ErrTooLarge = errors.New("client: response body too large")

	// ErrDecode is returned when the body is not a JSON document of the
	// expected shape. The decoder's message is dropped on purpose: it can
	// quote the body.
	ErrDecode = errors.New("client: cannot decode the document")
)

// StatusError is returned for any status other than 200, and for a 304 the
// caller did not ask for with an ETag. A 304 for a request that sent an ETag
// is a Result, not an error.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("client: unexpected status %d", e.StatusCode)
}

// SchemaVersionError is returned for a document whose schema_version this
// package does not know. Upgrade the module that holds this package.
type SchemaVersionError struct {
	// Got is the version the document carries. Zero means the key is absent.
	Got int
	// Supported is the only version this package decodes.
	Supported int
}

func (e *SchemaVersionError) Error() string {
	return fmt.Sprintf("client: unsupported schema_version %d, this client reads %d", e.Got, e.Supported)
}

// LanguageError is returned for a document whose language differs from the
// one the client asked for.
type LanguageError struct {
	// Want is the language the client asked for.
	Want string
	// Got is the language the document carries. It comes from the response,
	// so treat it as untrusted text.
	Got string
}

func (e *LanguageError) Error() string {
	return fmt.Sprintf("client: the document is not in the requested language %q", e.Want)
}
