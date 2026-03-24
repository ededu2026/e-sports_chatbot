package main

import (
	"log"
	"net/http"

	"github.com/ededu2026/e-sports_chatbot/internal/config"
	"github.com/ededu2026/e-sports_chatbot/internal/server"
)

func main() {
	cfg := config.Load()
	srv := server.New(cfg)

	log.Printf("starting %s on :%s using model %s", cfg.SystemName, cfg.Port, cfg.Model)
	if err := http.ListenAndServe(":"+cfg.Port, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
