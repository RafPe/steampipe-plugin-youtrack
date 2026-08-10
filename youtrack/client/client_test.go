package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewNormalizesAPIURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "root", base: "https://example.test", want: "https://example.test/api"},
		{name: "root slash", base: "https://example.test/", want: "https://example.test/api"},
		{name: "subpath", base: "https://example.test/youtrack", want: "https://example.test/youtrack/api"},
		{name: "already api", base: "https://example.test/youtrack/api/", want: "https://example.test/youtrack/api"},
		{name: "repeated api", base: "https://example.test/api/api", want: "https://example.test/api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New(tt.base, "secret")
			if err != nil {
				t.Fatalf("New(%q, token) error = %v, want nil", tt.base, err)
			}
			if got.baseURL.String() != tt.want {
				t.Errorf("New(%q, token).baseURL = %q, want %q", tt.base, got.baseURL, tt.want)
			}
		})
	}
}

func TestGetEscapesPathAndPreservesRepeatedQuery(t *testing.T) {
	t.Parallel()

	var gotRequest *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}))
	t.Cleanup(server.Close)

	c, err := New(server.URL+"/youtrack", "top secret")
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", server.URL, err)
	}
	query := url.Values{"tag": {"one", "two"}}
	var dst map[string]string
	if err := c.Get(context.Background(), []string{"issues", "a/b ?#"}, query, []string{"id", "summary"}, &dst); err != nil {
		t.Fatalf("Get(path, query, fields) error = %v, want nil", err)
	}
	if gotRequest.URL.EscapedPath() != "/youtrack/api/issues/a%2Fb%20%3F%23" {
		t.Errorf("Get() escaped path = %q, want %q", gotRequest.URL.EscapedPath(), "/youtrack/api/issues/a%2Fb%20%3F%23")
	}
	if got := gotRequest.URL.Query()["tag"]; !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Errorf("Get() tag query = %#v, want %#v", got, []string{"one", "two"})
	}
	if got := gotRequest.URL.Query()["fields"]; !reflect.DeepEqual(got, []string{"id", "summary"}) {
		t.Errorf("Get() fields query = %#v, want %#v", got, []string{"id", "summary"})
	}
	if got := gotRequest.Header.Get("Authorization"); got != "Bearer top secret" {
		t.Errorf("Get() Authorization = %q, want bearer token", got)
	}
}

func TestTokenDoesNotLeakIntoErrors(t *testing.T) {
	t.Parallel()

	const token = "do-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	server.Close()
	c, err := New(server.URL, token)
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", server.URL, err)
	}
	err = c.Get(context.Background(), []string{"issues"}, nil, nil, &map[string]any{})
	if err == nil {
		t.Fatal("Get() error = nil, want transport error")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("Get() error = %q, must not contain token", err)
	}
}

func TestTokenIsRedactedFromHTTPErrorBody(t *testing.T) {
	t.Parallel()

	const token = "reflected-permanent-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Authorization: Bearer "+token, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, token)
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", server.URL, err)
	}
	err = c.Get(context.Background(), nil, nil, nil, &map[string]any{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Get(reflected token) error = %T, want *HTTPError", err)
	}
	if strings.Contains(httpErr.Body, token) || !strings.Contains(httpErr.Body, "[REDACTED]") {
		t.Errorf("HTTPError.Body token redacted = false, body = %q", httpErr.Body)
	}
}

func TestClientRejectsCrossOriginRedirectWithoutForwardingToken(t *testing.T) {
	t.Parallel()

	const token = "redirect-secret"
	var targetAuthorization string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(source.Close)

	c, err := New(source.URL, token)
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", source.URL, err)
	}
	err = c.Get(context.Background(), nil, nil, nil, &map[string]any{})
	if !errors.Is(err, errUnsafeRedirect) {
		t.Errorf("Get(cross-origin redirect) error = %v, want errUnsafeRedirect", err)
	}
	if targetAuthorization != "" {
		t.Errorf("cross-origin target Authorization = %q, want empty", targetAuthorization)
	}
}

func TestClientAllowsSameOriginRedirect(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/final" {
			http.Redirect(w, r, server.URL+"/final", http.StatusFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("same-origin redirect Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)

	c, err := New(server.URL, "token")
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", server.URL, err)
	}
	if err := c.Get(context.Background(), nil, nil, nil, &map[string]any{}); err != nil {
		t.Errorf("Get(same-origin redirect) error = %v, want nil", err)
	}
}

func TestClientPreservesCustomRedirectPolicy(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("custom redirect rejected")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	httpClient := server.Client()
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return wantErr }
	c, err := New(server.URL, "token", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("New(%q, token, custom redirect client) error = %v, want nil", server.URL, err)
	}
	if err := c.Get(context.Background(), nil, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Errorf("Get(custom redirect policy) error = %v, want %v", err, wantErr)
	}
}

func TestClientLimitsSameOriginRedirects(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token")
	if err != nil {
		t.Fatalf("New(%q, token) error = %v, want nil", server.URL, err)
	}
	if err := c.Get(context.Background(), nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Errorf("Get(redirect loop) error = %v, want redirect limit error", err)
	}
}

func TestListPaginatesAndHonorsLimit(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		skip := r.URL.Query().Get("$skip")
		top := r.URL.Query().Get("$top")
		switch skip {
		case "0":
			_, _ = fmt.Fprintf(w, `[{"id":"1"},{"id":"2"}]`)
			if top != "2" {
				t.Errorf("List() first $top = %q, want 2", top)
			}
		case "2":
			_, _ = fmt.Fprintf(w, `[{"id":"3"}]`)
			if top != "1" {
				t.Errorf("List() second $top = %q, want 1", top)
			}
		default:
			t.Errorf("List() $skip = %q, want 0 or 2", skip)
		}
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token", WithPageSize(2))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	var got []struct {
		ID string `json:"id"`
	}
	if err := c.List(context.Background(), []string{"issues"}, nil, []string{"id"}, 3, &got); err != nil {
		t.Fatalf("List(limit=3) error = %v, want nil", err)
	}
	want := []struct {
		ID string `json:"id"`
	}{{"1"}, {"2"}, {"3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List(limit=3) = %#v, want %#v", got, want)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("List(limit=3) requests = %d, want 2", got)
	}
}

func TestListUsesConservativeDefaultPageSizeForRichResources(t *testing.T) {
	t.Parallel()

	const safePageSize = 25
	var gotTop string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTop = r.URL.Query().Get("$top")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	t.Cleanup(server.Close)

	c, err := New(server.URL, "token")
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	var got []map[string]any
	if err := c.List(context.Background(), []string{"issues"}, nil, []string{"id", "description", "comments(text)"}, 0, &got); err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if gotTop != strconv.Itoa(safePageSize) {
		t.Errorf("List() default $top = %q, want %d", gotTop, safePageSize)
	}
}

func TestListEnforcesConfiguredResponseLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"description":"`+strings.Repeat("x", 64)+`"}]`)
	}))
	t.Cleanup(server.Close)

	c, err := New(server.URL, "token", WithMaxBodyBytes(32))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	var got []map[string]any
	err = c.List(context.Background(), []string{"issues"}, nil, []string{"description"}, 0, &got)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Errorf("List() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestGetRetriesTransientResponses(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		if n == 2 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ok"}`)
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token", WithRetry(3, time.Millisecond))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	var got map[string]string
	if err := c.Get(context.Background(), []string{"issues"}, nil, nil, &got); err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if requests.Load() != 3 {
		t.Errorf("Get() requests = %d, want 3", requests.Load())
	}
}

func TestGetRetryWaitHonorsContext(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token", WithRetry(2, defaultMaxRetryDelay))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = c.Get(ctx, []string{"issues"}, nil, nil, &map[string]any{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Get(deadline) error = %v, want context deadline exceeded", err)
	}
}

func TestHTTPErrorClassificationAndBoundedBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, strings.Repeat("x", 128))
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token", WithMaxBodyBytes(16))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	err = c.Get(context.Background(), []string{"missing"}, nil, nil, &map[string]any{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Get() error = %T %v, want *HTTPError", err, err)
	}
	if httpErr.Kind != ErrorNotFound {
		t.Errorf("HTTPError.Kind = %v, want %v", httpErr.Kind, ErrorNotFound)
	}
	if len(httpErr.Body) != 16 {
		t.Errorf("len(HTTPError.Body) = %d, want 16", len(httpErr.Body))
	}
}

func TestSuccessfulResponseBodyIsBounded(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat(" ", 32)+`{}`)
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL, "token", WithMaxBodyBytes(16))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	err = c.Get(context.Background(), []string{"issues"}, nil, nil, &map[string]any{})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Errorf("Get() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestIndependentClientsAreConcurrent(t *testing.T) {
	t.Parallel()

	var (
		mu  sync.Mutex
		got = make(map[string]string)
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)
	one, _ := New(server.URL+"/one", "first")
	two, _ := New(server.URL+"/two", "second")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = one.Get(context.Background(), []string{"issues"}, nil, nil, &map[string]any{})
		}()
		go func() {
			defer wg.Done()
			_ = two.Get(context.Background(), []string{"issues"}, nil, nil, &map[string]any{})
		}()
	}
	wg.Wait()
	if got["/one/api/issues"] != "Bearer first" {
		t.Errorf("first client Authorization = %q, want Bearer first", got["/one/api/issues"])
	}
	if got["/two/api/issues"] != "Bearer second" {
		t.Errorf("second client Authorization = %q, want Bearer second", got["/two/api/issues"])
	}
}
