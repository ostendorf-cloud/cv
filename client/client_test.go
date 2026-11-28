package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ostendorf-cloud/cv/client"
)

const testETag = `"0123456789abcdef"`

// documentJSON is a small valid document for lang.
func documentJSON(t *testing.T, lang string) []byte {
	t.Helper()
	name := "Release 1"
	published := "2026-11-01T10:00:00Z"
	doc := client.Document{
		SchemaVersion: 1,
		Language:      lang,
		Projects: []client.Project{{
			ID:          "proj-1",
			GithubRepo:  "example/repo",
			Description: "A project.",
			Languages:   []client.ProjectLanguage{{Name: "Go", Bytes: 900}, {Name: "Shell", Bytes: 100}},
			LatestRelease: &client.ProjectRelease{
				Tag: "v1.2.3", Name: &name, PublishedAt: &published,
			},
		}},
		Contact: client.Contact{Email: "me@example.com"},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// serve starts a server for h and returns its URL.
func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

// respond answers every request with status and body, and an ETag.
func respond(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("ETag", testETag)
		w.WriteHeader(status)
		_, _ = w.Write(body) // a failed write shows as a failed read in the test
	}
}

func newClient(t *testing.T, opts ...client.Option) *client.Client {
	t.Helper()
	c, err := client.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var got *http.Request
		url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Clone(context.Background())
			respond(http.StatusOK, documentJSON(t, "en"))(w, r)
		}))

		res, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if res.NotModified || res.Document == nil {
			t.Fatalf("result = %+v, want a document", res)
		}
		if res.ETag != testETag {
			t.Errorf("etag = %q, want %q", res.ETag, testETag)
		}
		if res.Document.Language != "en" || res.Document.SchemaVersion != 1 {
			t.Errorf("document = %d %q", res.Document.SchemaVersion, res.Document.Language)
		}
		p := res.Document.Projects[0]
		if len(p.Languages) != 2 || p.Languages[0].Name != "Go" || p.Languages[0].Bytes != 900 {
			t.Errorf("languages = %+v", p.Languages)
		}
		if p.LatestRelease == nil || p.LatestRelease.Tag != "v1.2.3" || p.LatestRelease.URL != nil {
			t.Errorf("latest_release = %+v", p.LatestRelease)
		}

		if got.Method != http.MethodGet || got.URL.Path != "/api/v1/cv" || got.URL.Query().Get("lang") != "en" {
			t.Errorf("request = %s %s", got.Method, got.URL)
		}
		if v := got.Header.Get("If-None-Match"); v != "" {
			t.Errorf("If-None-Match = %q, want none without an etag", v)
		}
	})

	t.Run("language option", func(t *testing.T) {
		t.Parallel()
		var lang atomic.Value
		url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lang.Store(r.URL.Query().Get("lang"))
			respond(http.StatusOK, documentJSON(t, "de"))(w, r)
		}))

		res, err := newClient(t, client.WithBaseURL(url), client.WithLanguage("de")).Get(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if lang.Load() != "de" || res.Document.Language != "de" {
			t.Errorf("asked %v, got %q", lang.Load(), res.Document.Language)
		}
	})

	t.Run("base URL path prefix", func(t *testing.T) {
		t.Parallel()
		var path atomic.Value
		url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path.Store(r.URL.Path)
			respond(http.StatusOK, documentJSON(t, "en"))(w, r)
		}))

		if _, err := newClient(t, client.WithBaseURL(url+"/prefix/")).Get(context.Background(), ""); err != nil {
			t.Fatal(err)
		}
		if path.Load() != "/prefix/api/v1/cv" {
			t.Errorf("path = %v", path.Load())
		}
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		t.Parallel()
		var doc map[string]any
		if err := json.Unmarshal(documentJSON(t, "en"), &doc); err != nil {
			t.Fatal(err)
		}
		doc["added_later"] = map[string]any{"a": []int{1, 2}}
		doc["projects"] = []any{map[string]any{"id": "proj-1", "stars": 7}}
		body, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		url := serve(t, respond(http.StatusOK, body))

		res, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if res.Document.Projects[0].ID != "proj-1" {
			t.Errorf("project = %+v", res.Document.Projects[0])
		}
	})

	t.Run("not modified", func(t *testing.T) {
		t.Parallel()
		var sent atomic.Value
		url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sent.Store(r.Header.Get("If-None-Match"))
			w.WriteHeader(http.StatusNotModified)
		}))

		res, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), testETag)
		if err != nil {
			t.Fatalf("a 304 is not an error: %v", err)
		}
		if !res.NotModified || res.Document != nil || res.ETag != testETag {
			t.Errorf("result = %+v, want NotModified, no document and the sent etag", res)
		}
		if sent.Load() != testETag {
			t.Errorf("If-None-Match = %v, want %q", sent.Load(), testETag)
		}
	})

	t.Run("not modified without an etag sent", func(t *testing.T) {
		t.Parallel()
		url := serve(t, respond(http.StatusNotModified, nil))

		_, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
		var se *client.StatusError
		if !errors.As(err, &se) || se.StatusCode != http.StatusNotModified {
			t.Fatalf("err = %v, want a StatusError 304", err)
		}
	})

	t.Run("document after a stale etag", func(t *testing.T) {
		t.Parallel()
		url := serve(t, respond(http.StatusOK, documentJSON(t, "en")))

		res, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), `"old"`)
		if err != nil {
			t.Fatal(err)
		}
		if res.NotModified || res.Document == nil || res.ETag != testETag {
			t.Errorf("result = %+v", res)
		}
	})
}

func TestClient_Get_status(t *testing.T) {
	t.Parallel()

	const leak = "secret-body-text"

	for _, status := range []int{
		http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusInternalServerError, http.StatusServiceUnavailable,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				respond(status, []byte(leak))(w, r)
			}))

			_, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
			var se *client.StatusError
			if !errors.As(err, &se) || se.StatusCode != status {
				t.Fatalf("err = %v, want a StatusError %d", err, status)
			}
			if strings.Contains(err.Error(), leak) || strings.Contains(err.Error(), url) {
				t.Errorf("the message carries the body or the URL: %q", err)
			}
			if n := calls.Load(); n != 1 {
				t.Errorf("%d requests, want 1: the client never retries", n)
			}
		})
	}
}

func TestClient_Get_refusedDocument(t *testing.T) {
	t.Parallel()

	const leak = "secret-body-text"

	tests := []struct {
		name    string
		lang    string
		body    string
		wantErr func(error) bool
	}{
		{
			name: "unknown schema version",
			body: `{"schema_version":2,"language":"en"}`,
			wantErr: func(err error) bool {
				var e *client.SchemaVersionError
				return errors.As(err, &e) && e.Got == 2 && e.Supported == 1
			},
		},
		{
			name: "schema version below the known one",
			body: `{"schema_version":0,"language":"en"}`,
			wantErr: func(err error) bool {
				var e *client.SchemaVersionError
				return errors.As(err, &e) && e.Got == 0
			},
		},
		{
			name: "schema version missing",
			body: `{"language":"en"}`,
			wantErr: func(err error) bool {
				var e *client.SchemaVersionError
				return errors.As(err, &e) && e.Got == 0
			},
		},
		{
			name: "unknown schema version with a shape this client cannot decode",
			body: `{"schema_version":2,"language":"en","experience":{"is":"now an object"}}`,
			wantErr: func(err error) bool {
				var e *client.SchemaVersionError
				return errors.As(err, &e) && e.Got == 2
			},
		},
		{
			name: "wrong language",
			body: `{"schema_version":1,"language":"de"}`,
			wantErr: func(err error) bool {
				var e *client.LanguageError
				return errors.As(err, &e) && e.Want == "en" && e.Got == "de"
			},
		},
		{
			name: "language missing",
			body: `{"schema_version":1}`,
			wantErr: func(err error) bool {
				var e *client.LanguageError
				return errors.As(err, &e) && e.Got == ""
			},
		},
		{
			name: "language option differs from the document",
			lang: "de",
			body: `{"schema_version":1,"language":"en"}`,
			wantErr: func(err error) bool {
				var e *client.LanguageError
				return errors.As(err, &e) && e.Want == "de" && e.Got == "en"
			},
		},
		{
			name:    "not json",
			body:    leak,
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name:    "empty body",
			body:    "",
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name:    "truncated json",
			body:    `{"schema_version":1,"language":"en","experience":[{"id":"` + leak,
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name:    "json array instead of an object",
			body:    `["` + leak + `"]`,
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name:    "right header, wrong field type",
			body:    `{"schema_version":1,"language":"en","experience":"` + leak + `"}`,
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name:    "trailing data after the document",
			body:    `{"schema_version":1,"language":"en"} ` + leak,
			wantErr: func(err error) bool { return errors.Is(err, client.ErrDecode) },
		},
		{
			name: "body over the cap",
			body: `{"schema_version":1,"language":"en","pad":"` + strings.Repeat("a", 5<<20) + `"}`,
			wantErr: func(err error) bool {
				return errors.Is(err, client.ErrTooLarge)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			url := serve(t, respond(http.StatusOK, []byte(tt.body)))
			opts := []client.Option{client.WithBaseURL(url)}
			if tt.lang != "" {
				opts = append(opts, client.WithLanguage(tt.lang))
			}

			res, err := newClient(t, opts...).Get(context.Background(), "")
			if err == nil || !tt.wantErr(err) {
				t.Fatalf("err = %v", err)
			}
			if res.Document != nil || res.NotModified {
				t.Errorf("result = %+v, want the zero Result with an error", res)
			}
			if strings.Contains(err.Error(), leak) || strings.Contains(err.Error(), url) {
				t.Errorf("the message carries the body or the URL: %q", err)
			}
		})
	}
}

func TestClient_Get_request(t *testing.T) {
	t.Parallel()

	t.Run("invalid etag", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		url := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))

		for _, etag := range []string{"\"a\"\r\nX-Injected: 1", "\"a\"\n", "\"\x00\"", "\"ä\""} {
			_, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), etag)
			if !errors.Is(err, client.ErrInvalidETag) {
				t.Errorf("etag %q: err = %v, want ErrInvalidETag", etag, err)
			}
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("%d requests, want none", n)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		t.Parallel()
		url := serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)

		_, err := newClient(t, client.WithBaseURL(url)).Get(ctx, "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if strings.Contains(err.Error(), url) {
			t.Errorf("the message carries the URL: %q", err)
		}
	})

	t.Run("deadline of the caller", func(t *testing.T) {
		t.Parallel()
		url := serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := newClient(t, client.WithBaseURL(url)).Get(ctx, "")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("client timeout", func(t *testing.T) {
		t.Parallel()
		url := serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))

		_, err := newClient(t, client.WithBaseURL(url), client.WithTimeout(50*time.Millisecond)).Get(context.Background(), "")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
		if strings.Contains(err.Error(), url) {
			t.Errorf("the message carries the URL: %q", err)
		}
	})

	t.Run("server gone", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()

		_, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
		if !errors.Is(err, client.ErrTransport) {
			t.Fatalf("err = %v, want ErrTransport", err)
		}
		host := strings.TrimPrefix(url, "http://")
		if strings.Contains(err.Error(), host) {
			t.Errorf("the message carries the address: %q", err)
		}
	})

	t.Run("redirect to a failing host", func(t *testing.T) {
		t.Parallel()
		url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1:1/elsewhere", http.StatusFound)
		}))

		_, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
		if !errors.Is(err, client.ErrTransport) {
			t.Fatalf("err = %v, want ErrTransport", err)
		}
		if strings.Contains(err.Error(), "127.0.0.1:1") {
			t.Errorf("the message carries the redirect target: %q", err)
		}
	})
}

// roundTripFunc is a fake transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("requests go through the given client", func(t *testing.T) {
		t.Parallel()
		var host atomic.Value
		hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			host.Store(r.URL.Host)
			rec := httptest.NewRecorder()
			respond(http.StatusOK, documentJSON(t, "en"))(rec, r)
			return rec.Result(), nil
		})}

		res, err := newClient(t, client.WithBaseURL("https://cv.example.org"), client.WithHTTPClient(hc)).Get(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if host.Load() != "cv.example.org" || res.Document == nil {
			t.Errorf("host = %v, document = %v", host.Load(), res.Document)
		}
	})

	t.Run("the given client is copied, not changed", func(t *testing.T) {
		t.Parallel()
		hc := &http.Client{}

		newClient(t, client.WithBaseURL("https://cv.example.org"), client.WithHTTPClient(hc), client.WithTimeout(3*time.Second))
		if hc.Timeout != 0 {
			t.Errorf("the caller's client timeout = %s, want 0", hc.Timeout)
		}
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	// A value in every rejected URL that must never reach a message.
	const secret = "secret-token"

	tests := []struct {
		name string
		opts []client.Option
	}{
		{"no options", nil},
		{"empty base URL", []client.Option{client.WithBaseURL("")}},
		{"no scheme", []client.Option{client.WithBaseURL("cv.example.org/" + secret)}},
		{"unparsable", []client.Option{client.WithBaseURL("://" + secret)}},
		{"scheme ftp", []client.Option{client.WithBaseURL("ftp://example.org/" + secret)}},
		{"no host", []client.Option{client.WithBaseURL("https:///" + secret)}},
		{"credentials", []client.Option{client.WithBaseURL("https://user:" + secret + "@example.org")}},
		{"query", []client.Option{client.WithBaseURL("https://example.org/?token=" + secret)}},
		{"empty query", []client.Option{client.WithBaseURL("https://example.org/?")}},
		{"fragment", []client.Option{client.WithBaseURL("https://example.org/#" + secret)}},
		{"empty language", []client.Option{client.WithBaseURL("https://example.org"), client.WithLanguage("")}},
		{"zero timeout", []client.Option{client.WithBaseURL("https://example.org"), client.WithTimeout(0)}},
		{"negative timeout", []client.Option{client.WithBaseURL("https://example.org"), client.WithTimeout(-time.Second)}},
		{"nil http client", []client.Option{client.WithBaseURL("https://example.org"), client.WithHTTPClient(nil)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := client.New(tt.opts...)
			if !errors.Is(err, client.ErrConfig) {
				t.Fatalf("err = %v, want ErrConfig", err)
			}
			if c != nil {
				t.Error("a client came back with an error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the message carries part of the base URL: %q", err)
			}
		})
	}

	t.Run("problems are reported together", func(t *testing.T) {
		t.Parallel()
		_, err := client.New(client.WithBaseURL(""), client.WithLanguage(""), client.WithTimeout(0))
		if err == nil {
			t.Fatal("no error")
		}
		if n := len(strings.Split(err.Error(), "\n")); n != 3 {
			t.Errorf("%d problems in %q, want 3", n, err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		for _, base := range []string{"http://localhost:8080", "https://cv.example.org/", "https://cv.example.org/prefix"} {
			if _, err := client.New(client.WithBaseURL(base), client.WithLanguage("de")); err != nil {
				t.Errorf("%q: %v", base, err)
			}
		}
	})
}
