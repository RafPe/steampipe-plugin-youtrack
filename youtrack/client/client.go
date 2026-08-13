package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	// defaultPageSize is the $top requested per list page. YouTrack applies a
	// server-side default of 42 only when $top is omitted; an explicit $top may
	// be larger. List reduces the page size to the query limit when smaller.
	defaultPageSize      = 100
	defaultMaxBodyBytes  = 4 << 20
	defaultMaxAttempts   = 3
	defaultRetryDelay    = 100 * time.Millisecond
	defaultMaxRetryDelay = 30 * time.Second
	// maxPageSize is a client-side sanity bound for WithPageSize, sized to keep
	// one page comfortably within the response body limit.
	maxPageSize = 1000
)

// ErrResponseTooLarge indicates that a response exceeded the configured body limit.
var ErrResponseTooLarge = errors.New("response body too large")

// ErrInvalidResponse indicates that a successful response did not satisfy the
// protocol expected by the client.
var ErrInvalidResponse = errors.New("invalid YouTrack response")

var errUnsafeRedirect = errors.New("unsafe YouTrack redirect")

// ErrorKind classifies an HTTP response failure.
type ErrorKind string

const (
	// ErrorAuthentication identifies 401 responses.
	ErrorAuthentication ErrorKind = "authentication"
	// ErrorAuthorization identifies 403 responses.
	ErrorAuthorization ErrorKind = "authorization"
	// ErrorNotFound identifies 404 responses.
	ErrorNotFound ErrorKind = "not_found"
	// ErrorRateLimited identifies 429 responses.
	ErrorRateLimited ErrorKind = "rate_limited"
	// ErrorClient identifies other 4xx responses.
	ErrorClient ErrorKind = "client"
	// ErrorTransient identifies retryable 5xx responses.
	ErrorTransient ErrorKind = "transient"
	// ErrorServer identifies non-retryable 5xx responses.
	ErrorServer ErrorKind = "server"
)

// HTTPError describes a non-successful YouTrack HTTP response.
type HTTPError struct {
	StatusCode int
	Kind       ErrorKind
	Body       string
	RetryAfter time.Duration
}

// InvalidResponseError describes a successful response that cannot be treated
// as a YouTrack JSON response.
type InvalidResponseError struct {
	ContentType string
	Cause       error
}

// Error implements error.
func (e *InvalidResponseError) Error() string {
	if e.ContentType == "" {
		return "invalid YouTrack response"
	}
	return fmt.Sprintf("invalid YouTrack response content type %q", e.ContentType)
}

// Unwrap supports errors.Is for both ErrInvalidResponse and the underlying
// content-type parsing error.
func (e *InvalidResponseError) Unwrap() []error {
	errs := []error{ErrInvalidResponse}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

// Error implements error without including credentials or request headers.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("YouTrack request failed with status %d (%s)", e.StatusCode, e.Kind)
}

// Option configures a Client during construction.
type Option func(*Client) error

// WithHTTPClient sets the HTTP client used for requests. The caller remains
// responsible for the transport's lifecycle.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("HTTP client must not be nil")
		}
		c.httpClient = secureHTTPClient(httpClient)
		return nil
	}
}

// WithPageSize sets the maximum number of items requested per list page.
func WithPageSize(pageSize int) Option {
	return func(c *Client) error {
		if pageSize <= 0 || pageSize > maxPageSize {
			return fmt.Errorf("page size must be between 1 and %d", maxPageSize)
		}
		c.pageSize = pageSize
		return nil
	}
}

// WithMaxBodyBytes sets the largest response body accepted by the client.
func WithMaxBodyBytes(maxBytes int64) Option {
	return func(c *Client) error {
		if maxBytes <= 0 {
			return errors.New("maximum body size must be positive")
		}
		c.maxBodyBytes = maxBytes
		return nil
	}
}

// WithRetry sets the maximum GET attempts and fallback delay between attempts.
func WithRetry(maxAttempts int, delay time.Duration) Option {
	return func(c *Client) error {
		if maxAttempts <= 0 {
			return errors.New("maximum attempts must be positive")
		}
		if delay < 0 || delay > defaultMaxRetryDelay {
			return fmt.Errorf("retry delay must be between 0 and %s", defaultMaxRetryDelay)
		}
		c.maxAttempts = maxAttempts
		c.retryDelay = delay
		return nil
	}
}

// Client is a concurrency-safe YouTrack API client. Its configuration is
// immutable after New returns.
type Client struct {
	baseURL      *url.URL
	token        string
	httpClient   *http.Client
	pageSize     int
	maxBodyBytes int64
	maxAttempts  int
	retryDelay   time.Duration
}

// New constructs a Client. baseURL may point at a YouTrack root, subpath, or
// existing API root; New normalizes it to a single trailing /api component.
func New(baseURL, token string, options ...Option) (*Client, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("token must not be empty")
	}
	c := &Client{
		baseURL:      normalized,
		token:        token,
		httpClient:   secureHTTPClient(http.DefaultClient),
		pageSize:     defaultPageSize,
		maxBodyBytes: defaultMaxBodyBytes,
		maxAttempts:  defaultMaxAttempts,
		retryDelay:   defaultRetryDelay,
	}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("client option must not be nil")
		}
		if err := option(c); err != nil {
			return nil, fmt.Errorf("configure YouTrack client: %w", err)
		}
	}
	return c, nil
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.New("invalid YouTrack base URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("YouTrack base URL must use http or https")
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("YouTrack base URL must contain a host and no credentials, query, or fragment")
	}
	trimmedPath := strings.Trim(u.Path, "/")
	var parts []string
	if trimmedPath != "" {
		parts = strings.Split(trimmedPath, "/")
	}
	for len(parts) > 0 && parts[len(parts)-1] == "api" {
		parts = parts[:len(parts)-1]
	}
	parts = append(parts, "api")
	u.Path = "/" + strings.Join(parts, "/")
	u.RawPath = ""
	return u, nil
}

func secureHTTPClient(source *http.Client) *http.Client {
	clientCopy := *source
	previous := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		origin := via[0].URL
		if !strings.EqualFold(req.URL.Scheme, origin.Scheme) || !strings.EqualFold(req.URL.Host, origin.Host) {
			return errUnsafeRedirect
		}
		if previous != nil {
			return previous(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &clientCopy
}

// Get fetches and decodes one resource. Each path element is escaped as one
// URL segment. Query and fields values are copied and may contain repeats.
func (c *Client) Get(ctx context.Context, path []string, query url.Values, fields []string, dst any) error {
	requestURL := c.requestURL(path, query, fields)
	body, err := c.get(ctx, requestURL)
	if err != nil {
		return err
	}
	if dst == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode YouTrack response: %w", err)
	}
	return nil
}

// ListPages fetches pages sequentially and passes each non-empty page to
// onPage as a pointer to a slice of the same type as dst; dst itself is never
// written. onPage returns whether to continue with the next page. A positive
// limit bounds the total result count; zero retrieves pages until YouTrack
// returns an empty page.
func (c *Client) ListPages(ctx context.Context, path []string, query url.Values, fields []string, limit int, dst any, onPage func(page any) (bool, error)) error {
	if limit < 0 {
		return errors.New("list limit must not be negative")
	}
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Pointer || dstValue.IsNil() || dstValue.Elem().Kind() != reflect.Slice {
		return errors.New("list destination must be a non-nil pointer to a slice")
	}
	sliceType := dstValue.Elem().Type()
	for skip := 0; limit == 0 || skip < limit; {
		top := c.pageSize
		if limit > 0 && limit-skip < top {
			top = limit - skip
		}
		pageQuery := cloneValues(query)
		pageQuery.Set("$skip", strconv.Itoa(skip))
		pageQuery.Set("$top", strconv.Itoa(top))
		page := reflect.New(sliceType)
		if err := c.Get(ctx, path, pageQuery, fields, page.Interface()); err != nil {
			return err
		}
		items := page.Elem()
		if items.Len() == 0 {
			return nil
		}
		keepGoing, err := onPage(page.Interface())
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
		skip += items.Len()
	}
	return nil
}

// List fetches pages into a pointer to a slice. A positive limit bounds the
// total result count; zero retrieves pages until YouTrack returns an empty page.
func (c *Client) List(ctx context.Context, path []string, query url.Values, fields []string, limit int, dst any) error {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Pointer || dstValue.IsNil() || dstValue.Elem().Kind() != reflect.Slice {
		return errors.New("list destination must be a non-nil pointer to a slice")
	}
	result := dstValue.Elem()
	return c.ListPages(ctx, path, query, fields, limit, dst, func(page any) (bool, error) {
		result.Set(reflect.AppendSlice(result, reflect.ValueOf(page).Elem()))
		return true, nil
	})
}

func (c *Client) requestURL(path []string, query url.Values, fields []string) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimRight(c.baseURL.String(), "/"))
	for _, segment := range path {
		builder.WriteByte('/')
		builder.WriteString(url.PathEscape(segment))
	}
	values := cloneValues(query)
	for _, field := range fields {
		values.Add("fields", field)
	}
	if encoded := values.Encode(); encoded != "" {
		builder.WriteByte('?')
		builder.WriteString(encoded)
	}
	return builder.String()
}

func cloneValues(source url.Values) url.Values {
	destination := make(url.Values, len(source)+1)
	for key, values := range source {
		destination[key] = append([]string(nil), values...)
	}
	return destination
}

func (c *Client) get(ctx context.Context, requestURL string) ([]byte, error) {
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create YouTrack request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send YouTrack request: %w", err)
		}
		body, readErr := readResponse(resp.Body, c.maxBodyBytes, resp.StatusCode)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close YouTrack response: %w", closeErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(body) != 0 {
				contentType := redactSecret(resp.Header.Get("Content-Type"), c.token)
				mediaType, _, parseErr := mime.ParseMediaType(contentType)
				if parseErr != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
					return nil, &InvalidResponseError{ContentType: contentType, Cause: parseErr}
				}
			}
			return body, nil
		}
		httpErr := newHTTPError(resp.StatusCode, redactSecret(string(body), c.token), resp.Header.Get("Retry-After"), time.Now())
		if attempt == c.maxAttempts || !isRetryable(resp.StatusCode) {
			return nil, httpErr
		}
		delay := c.retryDelay
		if httpErr.RetryAfter >= 0 {
			delay = httpErr.RetryAfter
		}
		if err := wait(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("YouTrack request exhausted without a response")
}

func readResponse(reader io.Reader, limit int64, status int) ([]byte, error) {
	readLimit := limit
	if status >= 200 && status < 300 {
		readLimit++
	}
	body, err := io.ReadAll(io.LimitReader(reader, readLimit))
	if err != nil {
		return nil, fmt.Errorf("read YouTrack response: %w", err)
	}
	if status >= 200 && status < 300 && int64(len(body)) > limit {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func newHTTPError(status int, body, retryAfter string, now time.Time) *HTTPError {
	return &HTTPError{
		StatusCode: status,
		Kind:       classifyStatus(status),
		Body:       body,
		RetryAfter: parseRetryAfter(retryAfter, now),
	}
}

func classifyStatus(status int) ErrorKind {
	switch {
	case status == http.StatusUnauthorized:
		return ErrorAuthentication
	case status == http.StatusForbidden:
		return ErrorAuthorization
	case status == http.StatusNotFound:
		return ErrorNotFound
	case status == http.StatusTooManyRequests:
		return ErrorRateLimited
	case status >= 400 && status < 500:
		return ErrorClient
	case isRetryable(status):
		return ErrorTransient
	default:
		return ErrorServer
	}
}

func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError ||
		status == http.StatusBadGateway || status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return -1
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		if delay > defaultMaxRetryDelay || delay < 0 {
			return defaultMaxRetryDelay
		}
		return delay
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return -1
	}
	delay := when.Sub(now)
	if delay < 0 {
		return 0
	}
	if delay > defaultMaxRetryDelay {
		return defaultMaxRetryDelay
	}
	return delay
}

func redactSecret(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
