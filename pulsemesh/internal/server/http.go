// Package server provides the PulseMesh HTTP server with Prometheus metrics,
// a topology API, WebSocket broadcasting, and static file serving.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/youruser/pulsemesh/internal/cluster"
	"github.com/youruser/pulsemesh/internal/prober"
)

// ServerConfig holds dependencies for the HTTP server.
type ServerConfig struct {
	Engine  *prober.Engine
	Cluster *cluster.Cluster
	Hub     *Hub
	Static  fs.FS // embedded web/dist filesystem
}

// Server is the PulseMesh HTTP server.
type Server struct {
	engine  *prober.Engine
	cluster *cluster.Cluster
	hub     *Hub
	static  fs.FS
	srv     *http.Server
}

// TopologyResponse is the JSON structure for GET /api/topology.
type TopologyResponse struct {
	Nodes  []TopologyNode            `json:"nodes"`
	Matrix map[string]MatrixEntry    `json:"matrix"`
}

// TopologyNode represents a single cluster member.
type TopologyNode struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"`
	Alive bool   `json:"alive"`
}

// MatrixEntry holds the metrics for a directional src→dst link.
type MatrixEntry struct {
	P50Ms    float64 `json:"p50_ms"`
	P95Ms    float64 `json:"p95_ms"`
	P99Ms    float64 `json:"p99_ms"`
	JitterMs float64 `json:"jitter_ms"`
	LossPct  float64 `json:"loss_pct"`
}

// NewServer creates a new HTTP server.
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		engine:  cfg.Engine,
		cluster: cfg.Cluster,
		hub:     cfg.Hub,
		static:  cfg.Static,
	}
}

// Start listens on the given address and serves HTTP traffic.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint.
	mux.Handle("GET /metrics", promhttp.Handler())

	// Topology API.
	mux.HandleFunc("GET /api/topology", s.handleTopology)

	// WebSocket upgrade.
	if s.hub != nil {
		mux.HandleFunc("GET /ws", s.hub.HandleWS)
	}

	// Static files (SPA fallback).
	if s.static != nil {
		fileServer := http.FileServer(http.FS(s.static))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				fileServer.ServeHTTP(w, r)
				return
			}
			f, err := s.static.Open(path)
			if err != nil {
				// Fallback to index.html for SPA routes
				r.URL.Path = "/"
			} else {
				_ = f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	slog.Info("HTTP server starting", "addr", addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

var httpClient = &http.Client{
	Timeout: 500 * time.Millisecond,
}

// CollectTopology aggregates the cluster topology and N×N matrix.
// If localOnly is true, it only returns the local node's direct probe measurements.
// If localOnly is false, it fetches and merges metrics from all alive cluster members.
func CollectTopology(cl *cluster.Cluster, eng *prober.Engine, localOnly bool) TopologyResponse {
	members := cl.Members()
	localName := cl.LocalNode()

	nodes := make([]TopologyNode, 0, len(members))
	for _, m := range members {
		nodes = append(nodes, TopologyNode{
			Name:  m.Name,
			Addr:  m.Addr,
			Alive: true,
		})
	}

	allMetrics := eng.GetAllMetrics()
	matrix := make(map[string]MatrixEntry, len(allMetrics)*len(members))
	for peerName, pm := range allMetrics {
		key := localName + "->" + peerName
		matrix[key] = MatrixEntry{
			P50Ms:    pm.P50Ms,
			P95Ms:    pm.P95Ms,
			P99Ms:    pm.P99Ms,
			JitterMs: pm.JitterMs,
			LossPct:  pm.LossPct,
		}
	}

	if localOnly || len(members) <= 1 {
		return TopologyResponse{
			Nodes:  nodes,
			Matrix: matrix,
		}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, m := range members {
		if m.Name == localName || m.HTTPPort <= 0 {
			continue
		}
		wg.Add(1)
		go func(node cluster.NodeInfo) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s:%d/api/topology?local=true", node.Addr, node.HTTPPort)
			resp, err := httpClient.Get(url)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			var sub TopologyResponse
			if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
				return
			}
			mu.Lock()
			for k, v := range sub.Matrix {
				matrix[k] = v
			}
			mu.Unlock()
		}(m)
	}
	wg.Wait()

	return TopologyResponse{
		Nodes:  nodes,
		Matrix: matrix,
	}
}

// handleTopology returns the cluster topology and NxN metrics matrix as JSON.
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	localOnly := r.URL.Query().Get("local") == "true" || r.Header.Get("X-PulseMesh-Local") != ""
	resp := CollectTopology(s.cluster, s.engine, localOnly)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("topology JSON encode error", "err", err)
	}
}
