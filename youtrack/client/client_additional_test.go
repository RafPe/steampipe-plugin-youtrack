package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failingBody struct {
	readErr  error
	closeErr error
	closed   atomic.Bool
}

func (b *failingBody) Read([]byte) (int, error) { return 0, b.readErr }
func (b *failingBody) Close() error {
	b.closed.Store(true)
	return b.closeErr
}

func TestNewRejectsInvalidInputsAndOptions(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		token   string
		options []Option
	}{
		{name: "malformed URL", base: "%", token: "token"},
		{name: "unsupported scheme", base: "ftp://example.test", token: "token"},
		{name: "missing host", base: "https:///path", token: "token"},
		{name: "credentials", base: "https://user@example.test", token: "token"},
		{name: "query", base: "https://example.test?q=1", token: "token"},
		{name: "fragment", base: "https://example.test/#x", token: "token"},
		{name: "empty token", base: "https://example.test", token: " \t"},
		{name: "nil option", base: "https://example.test", token: "token", options: []Option{nil}},
		{name: "nil HTTP client", base: "https://example.test", token: "token", options: []Option{WithHTTPClient(nil)}},
		{name: "zero page", base: "https://example.test", token: "token", options: []Option{WithPageSize(0)}},
		{name: "oversized page", base: "https://example.test", token: "token", options: []Option{WithPageSize(43)}},
		{name: "zero body", base: "https://example.test", token: "token", options: []Option{WithMaxBodyBytes(0)}},
		{name: "zero attempts", base: "https://example.test", token: "token", options: []Option{WithRetry(0, 0)}},
		{name: "negative delay", base: "https://example.test", token: "token", options: []Option{WithRetry(1, -1)}},
		{name: "oversized delay", base: "https://example.test", token: "token", options: []Option{WithRetry(1, defaultMaxRetryDelay+time.Nanosecond)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.base, tt.token, tt.options...); err == nil {
				t.Fatal("New() error = nil, want error")
			}
		})
	}
}

func TestInvalidResponseDoesNotExposeToken(t *testing.T) {
	t.Parallel()

	token := strings.Join([]string{"invalid", "response", "value"}, "-")
	client := &Client{
		token: token,
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/" + token}},
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		})},
		maxAttempts: 1, maxBodyBytes: 1024,
	}

	_, err := client.get(context.Background(), "https://example.test")
	var invalid *InvalidResponseError
	if !errors.As(err, &invalid) {
		t.Fatalf("Client.get(reflected token content type) error = %v, want InvalidResponseError", err)
	}
	if strings.Contains(err.Error(), token) || strings.Contains(invalid.ContentType, token) {
		t.Errorf("Client.get(reflected token content type) error = %#v, must not expose token", invalid)
	}
}

func TestNewDoesNotExposeInvalidBaseURL(t *testing.T) {
	t.Parallel()

	secret := strings.Join([]string{"client", "url", "value"}, "-")
	_, err := New("https://user:"+secret+"%zz@example.test", "token")
	if err == nil {
		t.Fatal("New(invalid credential URL, token) error = nil, want error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("New(invalid credential URL, token) error = %q, must not contain URL credentials", err)
	}
}

func TestOptionsApplyAndHTTPErrorString(t *testing.T) {
	httpClient := &http.Client{}
	c, err := New("https://example.test", "token", WithHTTPClient(httpClient), WithPageSize(7), WithMaxBodyBytes(8), WithRetry(2, 3*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if c.httpClient == httpClient || c.httpClient.Transport != httpClient.Transport || c.pageSize != 7 || c.maxBodyBytes != 8 || c.maxAttempts != 2 || c.retryDelay != 3*time.Millisecond {
		t.Fatalf("options not applied: %#v", c)
	}
	errText := (&HTTPError{StatusCode: 403, Kind: ErrorAuthorization}).Error()
	if strings.Contains(errText, "token") || !strings.Contains(errText, "403") {
		t.Fatalf("HTTPError.Error() = %q", errText)
	}
}

func TestGetHeadersEmptyDestinationAndMalformedJSON(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("JSON headers = Accept %q, Content-Type %q", r.Header.Get("Accept"), r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch calls.Load() {
		case 1:
			w.WriteHeader(http.StatusNoContent)
		case 2:
			_, _ = io.WriteString(w, `{}`)
		default:
			_, _ = io.WriteString(w, `{`)
		}
	}))
	defer server.Close()
	c, _ := New(server.URL, "token")
	if err := c.Get(context.Background(), nil, nil, nil, nil); err != nil {
		t.Fatalf("empty Get() = %v", err)
	}
	if err := c.Get(context.Background(), nil, nil, nil, nil); err != nil {
		t.Fatalf("nil destination Get() = %v", err)
	}
	if err := c.Get(context.Background(), nil, nil, nil, &map[string]any{}); err == nil || !strings.Contains(err.Error(), "decode YouTrack response") {
		t.Fatalf("malformed Get() = %v", err)
	}
}

func TestSuccessfulContentTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantError   bool
	}{
		{name: "JSON", contentType: "application/json; charset=utf-8", body: `{}`},
		{name: "vendor JSON", contentType: "application/problem+json", body: `{}`},
		{name: "empty body ignores type", contentType: "text/plain"},
		{name: "non JSON", contentType: "text/plain", body: `{}`, wantError: true},
		{name: "missing", body: `{}`, wantError: true},
		{name: "malformed", contentType: `application/json; charset="`, body: `{}`, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {tt.contentType}}, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
				})},
				maxAttempts: 1, maxBodyBytes: 1024,
			}
			_, err := client.get(context.Background(), "https://example.test")
			if !tt.wantError {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var invalid *InvalidResponseError
			if !errors.As(err, &invalid) || !errors.Is(err, ErrInvalidResponse) || invalid.ContentType != tt.contentType {
				t.Fatalf("error = %#v, want inspectable invalid response", err)
			}
			_ = invalid.Error()
			if invalid.Cause != nil && !errors.Is(err, invalid.Cause) {
				t.Fatalf("error does not unwrap cause %v", invalid.Cause)
			}
		})
	}
	if got := (&InvalidResponseError{}).Error(); got != "invalid YouTrack response" {
		t.Fatalf("empty InvalidResponseError.Error() = %q", got)
	}
}

func TestGetTransportRequestReadAndCloseFailures(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		c := &Client{httpClient: http.DefaultClient, maxAttempts: 1}
		if _, err := c.get(context.Background(), "://bad"); err == nil || !strings.Contains(err.Error(), "create YouTrack request") {
			t.Fatalf("get invalid URL = %v", err)
		}
	})
	t.Run("transport", func(t *testing.T) {
		transportErr := errors.New("transport failed")
		c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, transportErr })}, maxAttempts: 1}
		if _, err := c.get(context.Background(), "https://example.test"); !errors.Is(err, transportErr) {
			t.Fatalf("get transport error = %v", err)
		}
	})
	for _, tt := range []struct {
		name     string
		readErr  error
		closeErr error
		contains string
	}{
		{name: "read", readErr: errors.New("read failed"), contains: "read YouTrack response"},
		{name: "close", readErr: io.EOF, closeErr: errors.New("close failed"), contains: "close YouTrack response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &failingBody{readErr: tt.readErr, closeErr: tt.closeErr}
			c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 204, Header: make(http.Header), Body: body}, nil
			})}, maxAttempts: 1, maxBodyBytes: 10}
			_, err := c.get(context.Background(), "https://example.test")
			if err == nil || !strings.Contains(err.Error(), tt.contains) || !body.closed.Load() {
				t.Fatalf("get error = %v, closed = %v", err, body.closed.Load())
			}
		})
	}
}

func TestStatusClassificationAndRetryBehavior(t *testing.T) {
	tests := []struct {
		status int
		kind   ErrorKind
	}{
		{401, ErrorAuthentication}, {403, ErrorAuthorization}, {404, ErrorNotFound},
		{429, ErrorRateLimited}, {400, ErrorClient}, {500, ErrorTransient},
		{502, ErrorTransient}, {503, ErrorTransient}, {504, ErrorTransient}, {501, ErrorServer},
	}
	for _, tt := range tests {
		if got := classifyStatus(tt.status); got != tt.kind {
			t.Errorf("classifyStatus(%d) = %q, want %q", tt.status, got, tt.kind)
		}
		e := newHTTPError(tt.status, "body", "invalid", time.Now())
		if e.Kind != tt.kind || e.Body != "body" || e.RetryAfter != -1 {
			t.Errorf("newHTTPError(%d) = %#v", tt.status, e)
		}
	}

	for _, tt := range []struct {
		name     string
		status   int
		attempts int
	}{
		{name: "retry exhaustion", status: 500, attempts: 2},
		{name: "nonretryable server", status: 501, attempts: 1},
		{name: "client", status: 400, attempts: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("failure"))}, nil
			})}, maxAttempts: 2, maxBodyBytes: 10, retryDelay: 0}
			_, err := c.get(context.Background(), "https://example.test")
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || int(calls.Load()) != tt.attempts {
				t.Fatalf("get() error = %v calls = %d", err, calls.Load())
			}
		})
	}
}

func TestRetryAfterAndWait(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  time.Duration
	}{
		{"", -1}, {"2", 2 * time.Second}, {"-2", -1}, {"invalid", -1},
		{now.Add(3 * time.Second).Format(http.TimeFormat), 3 * time.Second},
		{now.Add(-time.Second).Format(http.TimeFormat), 0},
		{"3600", defaultMaxRetryDelay},
		{now.Add(time.Hour).Format(http.TimeFormat), defaultMaxRetryDelay},
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.value, now); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
	if err := wait(context.Background(), 0); err != nil {
		t.Fatalf("wait(0) = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait(canceled) = %v", err)
	}
}

func TestListContinuesWhenServerCapsPageBelowRequestedTop(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("$skip") {
		case "0":
			requests.Add(1)
			_, _ = io.WriteString(w, `[1]`)
		case "1":
			requests.Add(1)
			_, _ = io.WriteString(w, `[2]`)
		case "2":
			requests.Add(1)
			_, _ = io.WriteString(w, `[]`)
		default:
			t.Errorf("List() $skip = %q, want 0, 1, or 2", r.URL.Query().Get("$skip"))
		}
	}))
	t.Cleanup(server.Close)

	c, err := New(server.URL, "token", WithPageSize(2))
	if err != nil {
		t.Fatalf("New(server, token) error = %v, want nil", err)
	}
	var got []int
	if err := c.List(context.Background(), nil, nil, nil, 0, &got); err != nil {
		t.Fatalf("List(server-capped pages) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2}) || requests.Load() != 3 {
		t.Errorf("List(server-capped pages) = %v with %d requests, want [1 2] with 3 requests", got, requests.Load())
	}
}

func TestListValidationPaginationAndErrors(t *testing.T) {
	c, _ := New("https://example.test", "token")
	for _, tt := range []struct {
		name  string
		limit int
		dst   any
	}{
		{name: "negative limit", limit: -1, dst: &[]int{}},
		{name: "nonpointer", dst: []int{}},
		{name: "nil pointer", dst: (*[]int)(nil)},
		{name: "pointer nonslice", dst: new(int)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.List(context.Background(), nil, nil, nil, tt.limit, tt.dst); err == nil {
				t.Fatal("List() error = nil")
			}
		})
	}

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch calls.Add(1) {
		case 1:
			_, _ = io.WriteString(w, `[1,2]`)
		case 2:
			_, _ = io.WriteString(w, `[]`)
		default:
			_, _ = io.WriteString(w, `{`)
		}
	}))
	defer server.Close()
	c, _ = New(server.URL, "token", WithPageSize(2))
	got := []int{0}
	if err := c.List(context.Background(), nil, url.Values{"x": {"y"}}, nil, 0, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{0, 1, 2}) || calls.Load() != 2 {
		t.Fatalf("List() = %v calls=%d", got, calls.Load())
	}
	if err := c.List(context.Background(), nil, nil, nil, 1, &got); err == nil {
		t.Fatal("List malformed page error = nil")
	}
}

func TestReadResponseLimitsAndErrors(t *testing.T) {
	if got, err := readResponse(strings.NewReader("abcdef"), 3, 400); err != nil || string(got) != "abc" {
		t.Fatalf("bounded error body = %q, %v", got, err)
	}
	if _, err := readResponse(strings.NewReader("abcd"), 3, 200); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("large successful body = %v", err)
	}
	readErr := errors.New("boom")
	if _, err := readResponse(&failingBody{readErr: readErr}, 3, 200); !errors.Is(err, readErr) {
		t.Fatalf("read failure = %v", err)
	}
}

func TestCloneValuesNilAndRequestURLWithoutQuery(t *testing.T) {
	c, _ := New("https://example.test", "token")
	if got := cloneValues(nil); got == nil || len(got) != 0 {
		t.Fatalf("cloneValues(nil) = %#v", got)
	}
	if got := c.requestURL(nil, nil, nil); got != "https://example.test/api" {
		t.Fatalf("requestURL() = %q", got)
	}
}

func TestZeroAttemptsReturnsError(t *testing.T) {
	c := &Client{maxAttempts: 0}
	if _, err := c.get(context.Background(), "https://example.test"); err == nil {
		t.Fatal("get() error = nil")
	}
}

func TestJSONDecodeTypesAreCompatible(t *testing.T) {
	var dst []map[string]any
	if err := json.Unmarshal([]byte(`[{"id":"1"}]`), &dst); err != nil {
		t.Fatal(err)
	}
}
