package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Introspector struct {
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

type introspectionResponse struct {
	Active   bool   `json:"active"`
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
}

func (i Introspector) Validate(ctx context.Context, bearerToken string) (Principal, error) {
	if strings.TrimSpace(bearerToken) == "" {
		return Principal{}, errors.New("bearer token is required")
	}

	client := i.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	form := url.Values{}
	form.Set("token", bearerToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return Principal{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return Principal{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Principal{}, errors.New("karasu introspection request failed")
	}

	var data introspectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Principal{}, err
	}

	if !data.Active {
		return Principal{}, errors.New("token is inactive")
	}
	if i.RequiredAudience != "" && data.Audience != i.RequiredAudience {
		return Principal{}, errors.New("token audience is not allowed")
	}
	if i.RequiredScope != "" && !hasScope(data.Scope, i.RequiredScope) {
		return Principal{}, errors.New("token scope is not allowed")
	}

	return Principal{
		ClientID: data.ClientID,
		Scope:    data.Scope,
		Audience: data.Audience,
		Subject:  data.Subject,
	}, nil
}

func BearerToken(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("Authorization header must be Bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

func hasScope(scopeList, required string) bool {
	for _, scope := range strings.Fields(scopeList) {
		if scope == required {
			return true
		}
	}
	return false
}
