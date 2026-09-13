package sse

import (
	"encoding/json"
	"sync"
)

type Client chan []byte

type Hub struct {
	mu      sync.RWMutex
	clients map[Client]string
}

func New() *Hub {
	return &Hub{clients: map[Client]string{}}
}

func (h *Hub) Subscribe(orgID string) Client {
	ch := make(Client, 16)
	h.mu.Lock()
	h.clients[ch] = orgID
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch Client) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) Publish(orgID, kind string, payload any) {
	body, _ := json.Marshal(map[string]any{"kind": kind, "data": payload})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch, org := range h.clients {
		if org != orgID {
			continue
		}
		select {
		case ch <- body:
		default:
		}
	}
}
