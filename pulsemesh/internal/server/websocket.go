package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/youruser/pulsemesh/internal/cluster"
	"github.com/youruser/pulsemesh/internal/prober"
)

// Hub manages WebSocket clients and broadcasts topology updates.
type Hub struct {
	engine  *prober.Engine
	cluster *cluster.Cluster

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

// NewHub creates a new WebSocket broadcast hub.
func NewHub(engine *prober.Engine, cluster *cluster.Cluster) *Hub {
	return &Hub{
		engine:  engine,
		cluster: cluster,
		clients: make(map[*websocket.Conn]struct{}),
	}
}

// Run starts the broadcast ticker, sending topology updates every second to all
// connected WebSocket clients. It blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case <-ticker.C:
			h.broadcast()
		}
	}
}

// HandleWS upgrades an HTTP request to a WebSocket connection and registers it.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow any origin in development.
	})
	if err != nil {
		slog.Warn("websocket accept error", "err", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	slog.Info("websocket client connected", "remote", r.RemoteAddr)

	// Block reading until the client disconnects. We discard all incoming
	// messages since this is a broadcast-only channel.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()

	_ = conn.Close(websocket.StatusNormalClosure, "")
	slog.Info("websocket client disconnected", "remote", r.RemoteAddr)
}

// broadcast sends the current topology + metrics to all connected clients.
func (h *Hub) broadcast() {
	resp := CollectTopology(h.cluster, h.engine, false)

	data, err := json.Marshal(resp)
	if err != nil {
		slog.Warn("broadcast marshal error", "err", err)
		return
	}

	h.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for _, c := range clients {
		if err := c.Write(ctx, websocket.MessageText, data); err != nil {
			slog.Debug("broadcast write error", "err", err)
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			_ = c.Close(websocket.StatusInternalError, "write error")
		}
	}
}

// closeAll closes all connected WebSocket clients.
func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		_ = c.Close(websocket.StatusGoingAway, "server shutting down")
		delete(h.clients, c)
	}
}
