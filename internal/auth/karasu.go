package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Verifier validates bearer tokens against Karasu's gateway-style verification
// endpoint (GET /oauth/verify). The bearer being validated is itself the
// credential, so — unlike RFC 7662 introspection — no separate caller
// authentication is required. Audience/scope are enforced server-side via the
// X-Required-Audience / X-Required-Scope request headers; the resolved
// principal is returned in X-Token-* response headers on success (HTTP 204).
type Verifier struct {
	URL              string
	RequiredAudience string
	RequiredScope    string
	HTTPClient       *http.Client
}

type Principal struct {
	ClientID string
	Scope    string
	Audience string
	Subject  string
}

func (v Verifier) Validate(ctx context.Context, bearerToken string) (Principal, error) {
	if strings.TrimSpace(bearerToken) == "" {
		return Principal{}, errors.New("bearer token is required")
	}

	client := v.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.URL, nil)
	if err != nil {
		return Principal{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	if v.RequiredAudience != "" {
		req.Header.Set("X-Required-Audience", v.RequiredAudience)
	}
	if v.RequiredScope != "" {
		req.Header.Set("X-Required-Scope", v.RequiredScope)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Principal{}, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return Principal{
			ClientID: resp.Header.Get("X-Client-ID"),
			Scope:    resp.Header.Get("X-Token-Scope"),
			Audience: resp.Header.Get("X-Token-Audience"),
			Subject:  resp.Header.Get("X-Token-Subject"),
		}, nil
	case http.StatusUnauthorized:
		return Principal{}, errors.New("token is inactive or invalid")
	case http.StatusForbidden:
		return Principal{}, errors.New("token does not satisfy required audience or scope")
	default:
		return Principal{}, errors.New("karasu verification request failed")
	}
}

func BearerToken(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("Authorization header must be Bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}
