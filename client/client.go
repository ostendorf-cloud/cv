// Package client reads the CV document from cvd's GET /api/v1/cv.
//
// It does one read per call. It holds no cache and does not retry: the
// caller owns both, and passes the ETag of its last document to turn a read
// into a cheap revalidation. It lives outside internal/ so that another
// module can import it, and it defines its own document types for the same
// reason.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// supportedSchemaVersion is the only schema_version this package
	// decodes. It must equal cv.SchemaVersion, which the drift test checks.
	supportedSchemaVersion = 1

	// defaultLanguage is the language cvd serves without a lang parameter.
	defaultLanguage = "en"

	// defaultTimeout bounds one read when the caller sets none.
	defaultTimeout = 10 * time.Second

	// maxBodyBytes caps what one read takes from the response. The real
	// document is a few tens of kilobytes. The cap is far above that and
	// keeps a wrong or hostile server from filling memory.
	maxBodyBytes = 4 << 20

	cvPath = "api/v1/cv"
)

// Client reads the CV document of one language from one cvd. It is safe for
// concurrent use.
type Client struct {
	endpoint string
	language string
	http     http.Client
}

// Result is the outcome of a successful Get.
type Result struct {
	// Document is the decoded document. It is nil when NotModified is true.
	Document *Document
	// ETag is the strong ETag of the document, as the server sent it, quotes
	// included. Pass it to the next Get. After a 304 it is the ETag the
	// caller sent.
	ETag string
	// NotModified is true when the server answered 304: the document the
	// caller holds for ETag is current.
	NotModified bool
}

// Option configures New.
type Option func(*config)

type config struct {
	baseURL    string
	language   string
	httpClient *http.Client
	timeout    time.Duration
	errs       []error
}

// WithBaseURL sets the address of cvd, for example https://cv.example.org.
// It is required. It must be an absolute http or https URL with a host and
// without credentials, query or fragment. A path prefix is kept.
func WithBaseURL(baseURL string) Option {
	return func(c *config) { c.baseURL = baseURL }
}

// WithLanguage sets the language to read, de or en. The default is en, the
// language cvd serves when no language is given.
func WithLanguage(language string) Option {
	return func(c *config) {
		if language == "" {
			c.errs = append(c.errs, fmt.Errorf("%w: the language is empty", ErrConfig))
			return
		}
		c.language = language
	}
}

// WithHTTPClient sets the HTTP client the requests go through, for a
// transport, a proxy or a test. New copies it, so a later change to hc does
// not reach the Client. Without WithTimeout, hc.Timeout applies, and zero
// means only the context of the call bounds a read.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) {
		if hc == nil {
			c.errs = append(c.errs, fmt.Errorf("%w: the http client is nil", ErrConfig))
			return
		}
		c.httpClient = hc
	}
}

// WithTimeout bounds one whole read, body included. It wins over the timeout
// of a client from WithHTTPClient. The default is ten seconds.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		if d <= 0 {
			c.errs = append(c.errs, fmt.Errorf("%w: the timeout is not positive", ErrConfig))
			return
		}
		c.timeout = d
	}
}

// New builds a Client. It validates every option once and returns all
// problems together, wrapping ErrConfig. No message repeats the base URL.
func New(opts ...Option) (*Client, error) {
	cfg := config{language: defaultLanguage}
	for _, opt := range opts {
		opt(&cfg)
	}

	endpoint, err := endpointURL(cfg.baseURL, cfg.language)
	if err != nil {
		cfg.errs = append(cfg.errs, err)
	}
	if len(cfg.errs) > 0 {
		return nil, errors.Join(cfg.errs...)
	}

	hc := http.Client{Timeout: defaultTimeout}
	if cfg.httpClient != nil {
		hc = *cfg.httpClient
	}
	if cfg.timeout > 0 {
		hc.Timeout = cfg.timeout
	}
	return &Client{endpoint: endpoint, language: cfg.language, http: hc}, nil
}

// endpointURL checks baseURL and returns the URL of the document route.
func endpointURL(baseURL, language string) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("%w: the base URL is required", ErrConfig)
	}
	// The parse error is dropped: it quotes the input.
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: the base URL does not parse", ErrConfig)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: the base URL scheme must be http or https", ErrConfig)
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("%w: the base URL has no host", ErrConfig)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: the base URL must not carry credentials", ErrConfig)
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("%w: the base URL must not carry a query or a fragment", ErrConfig)
	}

	u = u.JoinPath(cvPath)
	u.RawQuery = url.Values{"lang": {language}}.Encode()
	return u.String(), nil
}

// Get reads the document once. It uses ctx for the whole request and does
// not retry.
//
// A non empty etag, the ETag of the caller's last Result, is sent as
// If-None-Match. The server then answers 304 when nothing changed, and Get
// returns a Result with NotModified set and no Document.
//
// Errors the caller can match with errors.Is or errors.As: *StatusError,
// *SchemaVersionError, *LanguageError, ErrInvalidETag, ErrTooLarge,
// ErrDecode, ErrTransport, and the context errors of ctx.
func (c *Client) Get(ctx context.Context, etag string) (Result, error) {
	if !validETag(etag) {
		return Result{}, ErrInvalidETag
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return Result{}, ErrTransport
	}
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, transportError(ctx, err)
	}
	defer func() {
		// Nothing useful is left to do with a close error after the read.
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode == http.StatusNotModified && etag != "":
		return Result{ETag: etag, NotModified: true}, nil
	case resp.StatusCode != http.StatusOK:
		return Result{}, &StatusError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return Result{}, transportError(ctx, err)
	}
	if len(body) > maxBodyBytes {
		return Result{}, ErrTooLarge
	}

	doc, err := c.decode(body)
	if err != nil {
		return Result{}, err
	}
	return Result{Document: doc, ETag: resp.Header.Get("ETag")}, nil
}

// decode checks the schema version and the language before it decodes the
// whole document, so a document of a future shape reports the version, not a
// shape error. Unknown keys are ignored.
func (c *Client) decode(body []byte) (*Document, error) {
	var head struct {
		SchemaVersion int    `json:"schema_version"`
		Language      string `json:"language"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		return nil, ErrDecode
	}
	if head.SchemaVersion != supportedSchemaVersion {
		return nil, &SchemaVersionError{Got: head.SchemaVersion, Supported: supportedSchemaVersion}
	}
	if head.Language != c.language {
		return nil, &LanguageError{Want: c.language, Got: head.Language}
	}

	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, ErrDecode
	}
	return &doc, nil
}

// transportError maps an error of the HTTP client to one that carries no
// URL, address or body. The context of the caller and a timeout stay
// matchable, everything else becomes ErrTransport.
func transportError(ctx context.Context, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("client: request: %w", cerr)
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return fmt.Errorf("client: request: %w", context.DeadlineExceeded)
	}
	return ErrTransport
}

// validETag reports whether etag can travel as a header value: empty, or
// printable ASCII and tabs only.
func validETag(etag string) bool {
	return !strings.ContainsFunc(etag, func(r rune) bool {
		return r != '\t' && (r < 0x20 || r > 0x7e)
	})
}
