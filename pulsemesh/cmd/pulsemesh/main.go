// PulseMesh is a distributed network telemetry daemon that performs active
// probing between cluster members, collects latency/jitter/loss metrics, and
// exposes them via Prometheus, a REST API, and WebSocket streaming.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/youruser/pulsemesh/internal/cluster"
	"github.com/youruser/pulsemesh/internal/metrics"
	"github.com/youruser/pulsemesh/internal/prober"
	"github.com/youruser/pulsemesh/internal/server"
	"github.com/youruser/pulsemesh/web"
)

func main() {
	// --- CLI flags ---
	hostname, _ := os.Hostname()

	nodeName := flag.String("node-name", hostname, "Cluster node name")
	bindAddr := flag.String("bind-addr", "0.0.0.0", "Bind address for all listeners")
	gossipPort := flag.Int("gossip-port", 7946, "Memberlist gossip port")
	probePort := flag.Int("probe-port", 9000, "UDP probe engine port")
	httpPort := flag.Int("http-port", 8080, "HTTP server port")
	joinPeers := flag.String("join", "", "Comma-separated peer addresses to join (host:port)")
	probeInterval := flag.Duration("probe-interval", 500*time.Millisecond, "Probe interval")
	probeTimeout := flag.Duration("probe-timeout", 2*time.Second, "Probe timeout")

	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("starting PulseMesh",
		"node", *nodeName,
		"gossip_port", *gossipPort,
		"probe_port", *probePort,
		"http_port", *httpPort,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- 1. Create cluster ---
	cl, err := cluster.NewCluster(cluster.ClusterConfig{
		NodeName:   *nodeName,
		BindAddr:   *bindAddr,
		GossipPort: *gossipPort,
		ProbePort:  *probePort,
		HTTPPort:   *httpPort,
	})
	if err != nil {
		slog.Error("failed to create cluster", "err", err)
		os.Exit(1)
	}

	if *joinPeers != "" {
		peers := strings.Split(*joinPeers, ",")
		if err := cl.Join(peers); err != nil {
			slog.Error("failed to join cluster", "err", err)
			os.Exit(1)
		}
	}

	// --- 2. Create probe engine ---
	var nodeID [16]byte
	copy(nodeID[:], []byte(*nodeName))

	engine := prober.NewEngine(prober.EngineConfig{
		ProbePort:     *probePort,
		ProbeInterval: *probeInterval,
		ProbeTimeout:  *probeTimeout,
		NodeID:        nodeID,
	})

	// Set initial peers from cluster.
	updatePeers(engine, cl, *nodeName)

	// --- 3. Register Prometheus metrics ---
	metrics.RegisterPrometheusMetrics(engine, *nodeName)

	// --- 4. Create WebSocket hub ---
	hub := server.NewHub(engine, cl)
	go hub.Run(ctx)

	// --- 5. Start probe engine ---
	if err := engine.Start(ctx); err != nil {
		slog.Error("failed to start probe engine", "err", err)
		os.Exit(1)
	}

	// --- 6. Create and start HTTP server ---
	staticFS, err := web.DistFS()
	if err != nil {
		slog.Warn("failed to load embedded web assets", "err", err)
	}

	srv := server.NewServer(server.ServerConfig{
		Engine:  engine,
		Cluster: cl,
		Hub:     hub,
		Static:  staticFS,
	})

	go func() {
		addr := fmt.Sprintf("%s:%d", *bindAddr, *httpPort)
		if err := srv.Start(addr); err != nil {
			slog.Error("HTTP server error", "err", err)
			cancel()
		}
	}()

	// --- 7. Watch for membership changes ---
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updatePeers(engine, cl, *nodeName)
			}
		}
	}()

	// --- 8. Graceful shutdown ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	_ = srv.Shutdown(shutdownCtx)
	engine.Stop()
	_ = cl.Shutdown()

	slog.Info("PulseMesh stopped")
}

// updatePeers refreshes the engine's peer list from cluster membership.
func updatePeers(engine *prober.Engine, cl *cluster.Cluster, localName string) {
	members := cl.Members()
	var peers []prober.PeerInfo
	for _, m := range members {
		if m.Name == localName {
			continue
		}
		peers = append(peers, prober.PeerInfo{
			Name:      m.Name,
			Addr:      m.Addr,
			ProbePort: m.ProbePort,
		})
	}
	engine.SetPeers(peers)
}
