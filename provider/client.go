package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mrz1836/postmark"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const defaultBaseURL = "https://api.postmarkapp.com"

// Postmark "resource not found" error codes, confirmed against the live API
// (https://postmarkapp.com/developer/api/overview#error-codes):
//
//   - Domain    -> 510  (HTTP 422)
//   - Signature -> 501  (HTTP 422)
//   - Template  -> 1101 (HTTP 422)
//
// A missing Server instead returns a bare HTTP 404 with an empty body (there is
// no dedicated server error code), which the library surfaces as
// "request failed with status 404: ..."; isNotFound catches that via its
// status-404 fallback.
const (
	errCodeBadAPIToken       int64 = 10
	errCodeSignatureNotFound int64 = 501
	errCodeDomainNotFound    int64 = 510
	errCodeTemplateNotFound  int64 = 1101
)

// newPostmarkClient builds a Postmark client. The upstream constructor's
// argument order is (serverToken, accountToken) — wrapped here so callers can't
// transpose the two tokens by accident.
func newPostmarkClient(cfg Config, serverToken string) *postmark.Client {
	c := postmark.NewClient(serverToken, cfg.AccountToken)
	if cfg.BaseURL != "" {
		c.BaseURL = cfg.BaseURL
	}
	c.HTTPClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: &retryTransport{next: http.DefaultTransport, maxRetries: 4},
	}
	return c
}

// accountClient returns a client for account-scoped operations (Servers,
// Domains, Sender Signatures). It errors if no account token is configured.
func accountClient(ctx context.Context) (*postmark.Client, error) {
	cfg := infer.GetConfig[Config](ctx)
	if cfg.AccountToken == "" {
		return nil, errors.New("postmark: an account API token is required; set the " +
			"`postmark:accountToken` provider config or the POSTMARK_ACCOUNT_TOKEN environment variable")
	}
	return newPostmarkClient(cfg, cfg.ServerToken), nil
}

// resolveServerToken determines which Server API token a Template operation
// should use, following precedence A → B → C:
//
//	A. args.ServerToken (idiomatic: wired from server.apiTokens[0])
//	B. args.ServerID    (provider looks the token up via the account token)
//	C. provider config serverToken (single-server convenience)
func resolveServerToken(ctx context.Context, args TemplateArgs) (string, error) {
	cfg := infer.GetConfig[Config](ctx)

	if args.ServerToken != nil && *args.ServerToken != "" {
		return *args.ServerToken, nil
	}

	if args.ServerID != nil {
		if cfg.AccountToken == "" {
			return "", errors.New("postmark: resolving a Template server token via `serverId` " +
				"requires an account token (`postmark:accountToken` / POSTMARK_ACCOUNT_TOKEN)")
		}
		client := newPostmarkClient(cfg, "")
		srv, err := client.GetServer(ctx, int64(*args.ServerID))
		if err != nil {
			return "", fmt.Errorf("looking up server %d to obtain its API token: %w", *args.ServerID, err)
		}
		if len(srv.APITokens) == 0 {
			return "", fmt.Errorf("server %d has no API tokens", *args.ServerID)
		}
		return srv.APITokens[0], nil
	}

	if cfg.ServerToken != "" {
		return cfg.ServerToken, nil
	}

	return "", errors.New("postmark: a Template requires a Server API token; set `serverToken` " +
		"(e.g. from server.apiTokens[0]), `serverId`, or the provider `postmark:serverToken` config")
}

// templateClient returns a client whose Server token is resolved for the given
// Template inputs.
func templateClient(ctx context.Context, args TemplateArgs) (*postmark.Client, error) {
	token, err := resolveServerToken(ctx, args)
	if err != nil {
		return nil, err
	}
	cfg := infer.GetConfig[Config](ctx)
	return newPostmarkClient(cfg, token), nil
}

// isNotFound reports whether err indicates the upstream resource no longer
// exists, so that Read can signal deletion to the Pulumi engine.
//
// The mrz1836/postmark client discards the HTTP status code when the response
// body is JSON, returning a postmark.APIError instead, so we key primarily on
// Postmark's numeric error codes with a message/status fallback.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr postmark.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode {
		case errCodeSignatureNotFound, errCodeDomainNotFound, errCodeTemplateNotFound:
			return true
		}
		msg := strings.ToLower(apiErr.Message)
		return strings.Contains(msg, "not found") ||
			strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "could not be found")
	}
	// Non-JSON error bodies are wrapped as "request failed with status %d: ...".
	return strings.Contains(strings.ToLower(err.Error()), "status 404")
}

// idToInt64 parses a Pulumi resource ID (which we store as the decimal form of
// a Postmark numeric ID) back into an int64.
func idToInt64(id string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid resource id %q: %w", id, err)
	}
	return n, nil
}

// --- small generic helpers ---------------------------------------------------

func ptr[T any](v T) *T { return &v }

func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// orDefault returns def when s is empty. Used for enum-valued Postmark fields
// (Color, DeliveryType, TrackLinks) that reject an empty string and must carry a
// valid default when the user leaves them unset.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// --- retrying HTTP transport -------------------------------------------------

// retryTransport retries idempotent-ish Postmark calls on HTTP 429 and
// transient 5xx responses with exponential backoff, honoring Retry-After. It
// buffers the request body so it can be replayed across attempts.
type retryTransport struct {
	next       http.RoundTripper
	maxRetries int
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		body = b
	}

	backoff := 500 * time.Millisecond
	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err = t.next.RoundTrip(req)
		if err == nil && !shouldRetry(resp.StatusCode) {
			return resp, nil
		}
		if attempt >= t.maxRetries {
			return resp, err
		}

		wait := backoff
		if resp != nil {
			if ra := retryAfter(resp.Header.Get("Retry-After")); ra > 0 {
				wait = ra
			}
			// Drain and close so the connection can be reused.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func retryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
