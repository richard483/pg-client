package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBearerToken(t *testing.T) {
	token, err := BearerToken("Bearer abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "abc123" {
		t.Fatalf("expected token abc123, got %s", token)
	}

	_, err = BearerToken("Basic abc123")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifierValidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-value" {
			t.Fatalf("expected bearer token in Authorization header, got %q", got)
		}
		if got := r.Header.Get("X-Required-Audience"); got != "pg-client-api" {
			t.Fatalf("expected X-Required-Audience pg-client-api, got %q", got)
		}
		if got := r.Header.Get("X-Required-Scope"); got != "pg-client:execute" {
			t.Fatalf("expected X-Required-Scope pg-client:execute, got %q", got)
		}

		w.Header().Set("X-Client-ID", "app_test")
		w.Header().Set("X-Token-Scope", "pg-client:execute other:scope")
		w.Header().Set("X-Token-Audience", "pg-client-api")
		w.Header().Set("X-Token-Subject", "app_test")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	verifier := Verifier{
		URL:              server.URL,
		RequiredAudience: "pg-client-api",
		RequiredScope:    "pg-client:execute",
	}

	principal, err := verifier.Validate(context.Background(), "token-value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if principal.ClientID != "app_test" {
		t.Fatalf("expected app_test, got %s", principal.ClientID)
	}
	if principal.Audience != "pg-client-api" {
		t.Fatalf("expected pg-client-api, got %s", principal.Audience)
	}
}

func TestVerifierRejectsForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Karasu returns 403 when the token doesn't satisfy required scope/audience.
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	verifier := Verifier{
		URL:              server.URL,
		RequiredAudience: "pg-client-api",
		RequiredScope:    "pg-client:execute",
	}

	_, err := verifier.Validate(context.Background(), "token-value")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "audience or scope") {
		t.Fatalf("expected audience/scope error, got %v", err)
	}
}

func TestVerifierRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	verifier := Verifier{URL: server.URL}

	_, err := verifier.Validate(context.Background(), "token-value")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "inactive or invalid") {
		t.Fatalf("expected inactive/invalid error, got %v", err)
	}
}
