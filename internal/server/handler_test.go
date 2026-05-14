package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pg-client/internal/auth"
	"pg-client/internal/db"
)

func TestExecuteRequiresBearerToken(t *testing.T) {
	handler := Handler{
		Auth:     fakeAuthenticator{},
		Executor: fakeExecutor{},
	}

	req := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestExecuteReturnsExecutionResult(t *testing.T) {
	handler := Handler{
		Auth:     fakeAuthenticator{},
		Executor: fakeExecutor{},
	}
	body := bytes.NewBufferString(`{"db":"postgres","schema":"public","query":"SELECT 1"}`)
	req := httptest.NewRequest(http.MethodPost, "/execute", body)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", payload["status"])
	}
	if _, ok := payload["principal"]; !ok {
		t.Fatal("expected principal in response")
	}
	if _, ok := payload["execution"]; !ok {
		t.Fatal("expected execution in response")
	}
}

func TestRoutesExposeOnlyExecute(t *testing.T) {
	handler := Handler{
		Auth:     fakeAuthenticator{},
		Executor: fakeExecutor{},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) Validate(ctx context.Context, bearerToken string) (auth.Principal, error) {
	return auth.Principal{
		ClientID: "app_test",
		Scope:    "pg-client:execute",
		Audience: "pg-client-api",
		Subject:  "app_test",
	}, nil
}

type fakeExecutor struct{}

func (fakeExecutor) Execute(ctx context.Context, req db.Request) db.Execution {
	return db.Execution{
		Database: req.Database,
		Schema:   req.Schema,
		Results: []db.Result{
			{
				Index:        1,
				CommandTag:   "SELECT 1",
				RowsAffected: 1,
				Rows:         []map[string]any{{"?column?": "1"}},
			},
		},
	}
}
