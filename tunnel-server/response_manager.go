package main

import (
	"errors"
	"sync"

	"github.com/igneel64/iskandar/shared/protocol"
)

type MessageChannel = chan protocol.Message

type RequestManager interface {
	GetRequestChannel(requestId string) (MessageChannel, bool)
	RegisterRequest(requestId, subdomain string) (MessageChannel, error)
	RemoveRequest(requestId, subdomain string)
}

var ErrMaxRequestsPerTunnel = errors.New("maximum number of concurrent requests per tunnel reached")

type InMemoryRequestManager struct {
	requestChannelMap map[string]MessageChannel
	requestCounts     map[string]int
	maxPerTunnel      int
	mu                sync.RWMutex
}

func NewInMemoryRequestManager(maxPerTunnel int) *InMemoryRequestManager {
	return &InMemoryRequestManager{
		requestChannelMap: make(map[string]MessageChannel),
		requestCounts:     make(map[string]int),
		maxPerTunnel:      maxPerTunnel,
	}
}

func (i *InMemoryRequestManager) GetRequestChannel(requestId string) (MessageChannel, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	ch, ok := i.requestChannelMap[requestId]
	return ch, ok
}

func (i *InMemoryRequestManager) RegisterRequest(requestId, subdomain string) (MessageChannel, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.requestCounts[subdomain] >= i.maxPerTunnel {
		return nil, ErrMaxRequestsPerTunnel
	}

	requestChannel := make(MessageChannel, 5)
	i.requestChannelMap[requestId] = requestChannel
	i.requestCounts[subdomain]++
	return requestChannel, nil
}

func (i *InMemoryRequestManager) RemoveRequest(requestId, subdomain string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if ch, ok := i.requestChannelMap[requestId]; ok {
		close(ch)
		delete(i.requestChannelMap, requestId)
		if i.requestCounts[subdomain] > 0 {
			i.requestCounts[subdomain]--
		}
		if i.requestCounts[subdomain] == 0 {
			delete(i.requestCounts, subdomain)
		}
	}
}
