package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecuteRejectsInvalidRequest(t *testing.T) {
	executor := Executor{
		DSNBuilder:       func(database string) string { return "" },
		Timeout:          time.Second,
		MaxRows:          1000,
		MaxResponseBytes: 1024,
	}

	result := executor.Execute(context.Background(), Request{
		Database: "bad-db",
		Schema:   "public",
		Query:    "SELECT 1",
	})

	if result.Error == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(result.Error.Message, "db") {
		t.Fatalf("expected db error, got %s", result.Error.Message)
	}
}

func TestValidateRequestRequiresSchema(t *testing.T) {
	err := validateRequest(Request{
		Database: "postgres",
		Query:    "SELECT 1",
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema error, got %s", err.Error())
	}
}
