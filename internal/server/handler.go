package server

import (
	"context"
	"encoding/json"
	"net/http"

	"pg-client/internal/auth"
	"pg-client/internal/db"
)

type Authenticator interface {
	Validate(ctx context.Context, bearerToken string) (auth.Principal, error)
}

type SQLExecutor interface {
	Execute(ctx context.Context, req db.Request) db.Execution
}

type Handler struct {
	Auth     Authenticator
	Executor SQLExecutor
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /execute", h.Execute)
	return mux
}

func (h Handler) Execute(w http.ResponseWriter, r *http.Request) {
	token, err := auth.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	principal, err := h.Auth.Validate(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req db.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := h.Executor.Execute(r.Context(), req)
	payload := map[string]any{
		"status":    "ok",
		"principal": principal,
		"execution": result,
	}
	status := http.StatusOK
	if result.Error != nil {
		payload["status"] = "error"
		status = http.StatusBadRequest
	}
	writeJSON(w, status, payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"status": status,
		"error":  message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
