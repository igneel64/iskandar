package main

import (
	"log"
	"net/http"

	"github.com/igneel64/iskandar/tunnel/internal/logger"
)

func main() {
	logger.Initialize(true)

	connectionStore := NewInMemoryConnectionStore()
	requestManager := NewInMemoryRequestManager()
	s := NewIskndrServer(connectionStore, requestManager)

	logger.ServerStarted(8080)
	log.Fatal(http.ListenAndServe(":8080", s))
}
