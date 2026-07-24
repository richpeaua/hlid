// Command hlid is Hlid's binary entrypoint: load config, assemble the server, serve.
package main

import (
	"flag"
	"log"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/server"
)

func main() {
	configPath := flag.String("config", "hlid.yaml", "path to the Hlid config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
