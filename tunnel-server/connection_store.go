package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/shared"
)

type ConnectionStore interface {
	RegisterConnection(conn *websocket.Conn) (string, error)
	GetConnection(subdomainKey string) (*shared.SafeWebSocketConn, error)
	RemoveConnection(subdomainKey string)
}

var ErrMaxTunnelsReached = errors.New("maximum number of tunnels reached")

type InMemoryConnectionStore struct {
	connMap    map[string]*shared.SafeWebSocketConn
	mu         sync.RWMutex
	maxTunnels int
}

func NewInMemoryConnectionStore(maxTunnels int) *InMemoryConnectionStore {
	return &InMemoryConnectionStore{
		connMap:    make(map[string]*shared.SafeWebSocketConn),
		maxTunnels: maxTunnels,
	}
}

func (i *InMemoryConnectionStore) RegisterConnection(conn *websocket.Conn) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if len(i.connMap) >= i.maxTunnels {
		return "", ErrMaxTunnelsReached
	}

	subdomainKey, err := generateSubdomainKey()
	if err != nil {
		return "", err
	}
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

func (i *InMemoryConnectionStore) RemoveConnection(subdomainKey string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.connMap, subdomainKey)
}

func generateSubdomainKey() (string, error) {
	const (
		subdomainLength = 8
		charset         = "abcdefghijklmnopqrstuvwxyz0123456789"
		charsetLength   = len(charset)
	)

	result := make([]byte, subdomainLength)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(charsetLength)))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}
