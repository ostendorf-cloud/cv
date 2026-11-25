// Package github is a small client for the two GitHub REST endpoints the CV
// render enriches a project with: a repository's languages and its latest
// release. It is bounded in time per call and in the number of calls per
// client, caches every result for the life of the client, never retries and
// treats a rate limit as the end of all further calls. See docs/concepts/cv.md,
// "A GitHub API outage degrades the project list, never the release".
package github

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the public GitHub REST API.
	DefaultBaseURL = "https://api.github.com"
	// DefaultCallTimeout bounds one HTTP call.
	DefaultCallTimeout = 10 * time.Second
	// DefaultMaxCalls bounds the HTTP calls one client makes in its lifetime.
	// Two calls fetch one repository, so this covers thirty repositories.
	DefaultMaxCalls = 60

	maxBodyBytes = 1 << 20
	userAgent    = "cv (https://github.com/ostendorf-cloud/cv)"
)

// The errors a lookup reports. Match them with errors.Is.
var (
	// ErrNotFound means the repository does not exist, or is not visible to
	// the token in use.
	ErrNotFound = errors.New("repository not found")
	// ErrRateLimited means GitHub refused the call for rate limit reasons.
	// Once seen, the client makes no further call.
	ErrRateLimited = errors.New("rate limited")
	// ErrCallBudget means the client already made its maximum number of calls.
	ErrCallBudget = errors.New("call budget exhausted")
	// ErrMalformed means GitHub answered with a body that is not what the
	// endpoint documents.
	ErrMalformed = errors.New("malformed response")
	// ErrInvalidRepo means the repository is not spelled owner/name. No call
	// is made for it.
	ErrInvalidRepo = errors.New("invalid repository name")
	// ErrUnexpectedStatus means GitHub answered with a status this client
	// does not handle, a plain 403 or a 5xx for example.
	ErrUnexpectedStatus = errors.New("unexpected status")
)

var repoName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)

// Language is one language of a repository and the bytes of code in it.
type Language struct {
	Name  string
	Bytes int64
}

// Release is a repository's latest published release.
type Release struct {
	Tag         string
	Name        string
	URL         string
	PublishedAt string
}

// Info is what GitHub knows about one repository. LatestRelease is nil for a
// repository without a published release.
type Info struct {
	// Languages is ordered by bytes, largest first, then by name.
	Languages     []Language
	LatestRelease *Release
}

// Config configures a Client. The zero value of every field but Token is
// replaced by a default.
type Config struct {
	BaseURL     string
	Token       string
	HTTPClient  *http.Client
	CallTimeout time.Duration
	MaxCalls    int
}

// Client fetches repository facts. It is safe for concurrent use and
// serialises its calls, so the cache and the call budget stay exact.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	timeout time.Duration
	max     int

	mu      sync.Mutex
	calls   int
	limited bool
	cache   map[string]result
}

type result struct {
	info Info
	err  error
}

// New returns a client for cfg. An empty Token makes anonymous calls, which
// work against public repositories at GitHub's lower rate limit.
func New(cfg Config) *Client {
	c := &Client{
		baseURL: strings.TrimRight(cmp.Or(cfg.BaseURL, DefaultBaseURL), "/"),
		token:   cfg.Token,
		http:    cfg.HTTPClient,
		timeout: cmp.Or(cfg.CallTimeout, DefaultCallTimeout),
		max:     cmp.Or(cfg.MaxCalls, DefaultMaxCalls),
		cache:   map[string]result{},
	}
	if c.http == nil {
		c.http = &http.Client{}
	}
	return c
}

// Repo returns the languages and latest release of repo, spelled owner/name.
// The first outcome for a repo, success or failure, is kept: a second call
// for it makes no request and returns the same outcome, so a missing
// repository is reported by one call only.
func (c *Client) Repo(ctx context.Context, repo string) (Info, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if r, ok := c.cache[repo]; ok {
		return r.info, r.err
	}
	info, err := c.fetch(ctx, repo)
	if ctx.Err() == nil {
		c.cache[repo] = result{info: info, err: err}
	}
	return info, err
}

func (c *Client) fetch(ctx context.Context, repo string) (Info, error) {
	if !repoName.MatchString(repo) {
		return Info{}, fmt.Errorf("github: %q: %w", repo, ErrInvalidRepo)
	}

	var raw map[string]int64
	if err := c.get(ctx, "/repos/"+repo+"/languages", &raw); err != nil {
		return Info{}, err
	}
	info := Info{}
	for name, n := range raw {
		info.Languages = append(info.Languages, Language{Name: name, Bytes: n})
	}
	slices.SortFunc(info.Languages, func(a, b Language) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), strings.Compare(a.Name, b.Name))
	})

	var rel struct {
		Tag         string `json:"tag_name"`
		Name        string `json:"name"`
		URL         string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	err := c.get(ctx, "/repos/"+repo+"/releases/latest", &rel)
	switch {
	case errors.Is(err, ErrNotFound):
		// A repository without a release answers 404 here. It exists,
		// because the languages call above found it.
		return info, nil
	case err != nil:
		return Info{}, err
	case rel.Tag == "":
		return Info{}, fmt.Errorf("github: /repos/%s/releases/latest: no tag_name: %w", repo, ErrMalformed)
	}
	info.LatestRelease = &Release{Tag: rel.Tag, Name: rel.Name, URL: rel.URL, PublishedAt: rel.PublishedAt}
	return info, nil
}

// get makes one bounded call and decodes a 200 answer into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	if c.limited {
		return fmt.Errorf("github: get %s: %w", path, ErrRateLimited)
	}
	if c.calls >= c.max {
		return fmt.Errorf("github: get %s: %w", path, ErrCallBudget)
	}
	c.calls++

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("github: get %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// The error names the URL and the cause, never a request header.
		return fmt.Errorf("github: get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }() // nothing to do about a failed close of a read body

	switch {
	case resp.StatusCode == http.StatusOK:
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(out); err != nil {
			return fmt.Errorf("github: get %s: %w: %w", path, ErrMalformed, err)
		}
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("github: get %s: %w", path, ErrNotFound)
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && rateLimitHeaders(resp.Header):
		c.limited = true
		return fmt.Errorf("github: get %s: %w", path, ErrRateLimited)
	default:
		return fmt.Errorf("github: get %s: %w: %d", path, ErrUnexpectedStatus, resp.StatusCode)
	}
}

// rateLimitHeaders reports whether a 403 is a rate limit: the primary limit
// is spent, or GitHub asks to wait (the secondary limit).
func rateLimitHeaders(h http.Header) bool {
	return h.Get("X-RateLimit-Remaining") == "0" || h.Get("Retry-After") != ""
}
