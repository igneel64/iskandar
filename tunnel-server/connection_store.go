package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/shared"
)

type ConnectionStore interface {
	RegisterConnection(conn *websocket.Conn) (string, error)
	GetConnection(subdomainKey string) (*shared.SafeWebSocketConn, error)
	RemoveConnection(subdomainKey string) error
}

type InMemoryConnectionStore struct {
	connMap map[string]*shared.SafeWebSocketConn
	mu      sync.RWMutex
}

func NewInMemoryConnectionStore() *InMemoryConnectionStore {
	return &InMemoryConnectionStore{
		connMap: make(map[string]*shared.SafeWebSocketConn),
	}
}

func (i *InMemoryConnectionStore) RegisterConnection(conn *websocket.Conn) (string, error) {
	subdomainKey := generateSubdomainKey()
	i.mu.Lock()
	defer i.mu.Unlock()
	i.connMap[subdomainKey] = shared.NewSafeWebSocketConn(conn)
	return subdomainKey, nil
}

func (i *InMemoryConnectionStore) GetConnection(subdomainKey string) (*shared.SafeWebSocketConn, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	conn, exists := i.connMap[subdomainKey]
	if !exists {
		return nil, fmt.Errorf("subdomain not found")
	}
	return conn, nil
}

func (i *InMemoryConnectionStore) RemoveConnection(subdomainKey string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.connMap, subdomainKey)
	return nil
}

func generateSubdomainKey() string {
	const (
		subdomainLength = 8
		charset         = "abcdefghijklmnopqrstuvwxyz0123456789"
		charsetLength   = len(charset)
	)

	result := make([]byte, subdomainLength)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(charsetLength)))
		result[i] = charset[num.Int64()]
	}

	return string(result)
}
