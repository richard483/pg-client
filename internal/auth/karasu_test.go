package auth

import (
	"context"
	"encoding/json"
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

func TestIntrospectorValidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.Form.Get("token") != "token-value" {
			t.Fatalf("expected token-value, got %s", r.Form.Get("token"))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"active":    true,
			"client_id": "app_test",
			"scope":     "pg-client:execute other:scope",
			"aud":       "pg-client-api",
			"sub":       "app_test",
		})
	}))
	defer server.Close()

	introspector := Introspector{
		URL:              server.URL,
		RequiredAudience: "pg-client-api",
		RequiredScope:    "pg-client:execute",
	}

	principal, err := introspector.Validate(context.Background(), "token-value")
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

func TestIntrospectorRejectsWrongScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": true,
			"scope":  "other:scope",
			"aud":    "pg-client-api",
		})
	}))
	defer server.Close()

	introspector := Introspector{
		URL:              server.URL,
		RequiredAudience: "pg-client-api",
		RequiredScope:    "pg-client:execute",
	}

	_, err := introspector.Validate(context.Background(), "token-value")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected scope error, got %v", err)
	}
}
