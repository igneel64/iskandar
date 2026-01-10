package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/igneel64/iskandar/server/internal/config"
	"github.com/igneel64/iskandar/server/internal/logger"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load environment config: %v", err)
	}

	logger.Initialize(cfg.Logging)

	publicURLBase, err := url.Parse(cfg.BaseScheme + "://" + cfg.BaseDomain)
	if err != nil {
		log.Fatalf("Failed to parse public URL base: %v", err)
	}
	connectionStore := NewInMemoryConnectionStore(cfg.MaxTunnels)
	requestManager := NewInMemoryRequestManager()

	server := NewIskndrServer(publicURLBase, connectionStore, requestManager)

	logger.ServerStarted(cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), server))
}
