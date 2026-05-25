package main

import (
	"log"
	"net/http"

	"pg-client/internal/auth"
	"pg-client/internal/config"
	"pg-client/internal/db"
	"pg-client/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	handler := server.Handler{
		Auth: auth.Verifier{
			URL:              cfg.KarasuVerifyURL,
			RequiredAudience: cfg.RequiredAudience,
			RequiredScope:    cfg.RequiredScope,
		},
		Executor: db.Executor{
			DSNBuilder:       cfg.PostgresDSN,
			Timeout:          cfg.QueryTimeout,
			MaxRows:          cfg.MaxRows,
			MaxResponseBytes: cfg.MaxResponseBytes,
		},
	}

	addr := ":" + cfg.Port
	log.Printf("pg-client listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler.Routes()))
}
