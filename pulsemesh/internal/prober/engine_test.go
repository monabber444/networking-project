package prober

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestEngineIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	const numEngines = 3
	basePorts := []int{19001, 19002, 19003}

	type engineEntry struct {
		engine *Engine
		nodeID [16]byte
		name   string
		port   int
	}

	engines := make([]engineEntry, numEngines)
	for i := 0; i < numEngines; i++ {
		var nodeID [16]byte
		nodeID[0] = byte(i + 1)
		engines[i] = engineEntry{
			engine: NewEngine(EngineConfig{
				ProbePort:     basePorts[i],
				ProbeInterval: 200 * time.Millisecond,
				ProbeTimeout:  2 * time.Second,
				NodeID:        nodeID,
			}),
			nodeID: nodeID,
			name:   fmt.Sprintf("node-%d", i+1),
			port:   basePorts[i],
		}
	}

	// Build peer lists (each engine talks to the other two).
	for i := range engines {
		var peers []PeerInfo
		for j := range engines {
			if i == j {
				continue
			}
			peers = append(peers, PeerInfo{
				Name:      engines[j].name,
				Addr:      "127.0.0.1",
				ProbePort: engines[j].port,
			})
		}
		engines[i].engine.SetPeers(peers)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start all engines.
	for i := range engines {
		if err := engines[i].engine.Start(ctx); err != nil {
			t.Fatalf("engine %d Start: %v", i, err)
		}
		defer engines[i].engine.Stop()
	}

	// Let them probe for a while.
	time.Sleep(5 * time.Second)

	// Verify metrics: each engine should have non-zero RTT to every other engine.
	for i := range engines {
		allMetrics := engines[i].engine.GetAllMetrics()
		for j := range engines {
			if i == j {
				continue
			}
			peerName := engines[j].name
			pm, ok := allMetrics[peerName]
			if !ok {
				t.Errorf("engine %s: no metrics for peer %s", engines[i].name, peerName)
				continue
			}
			if pm.P50Ms <= 0 {
				t.Errorf("engine %s -> %s: P50 = %.3fms, want > 0", engines[i].name, peerName, pm.P50Ms)
			}
			if pm.P95Ms <= 0 {
				t.Errorf("engine %s -> %s: P95 = %.3fms, want > 0", engines[i].name, peerName, pm.P95Ms)
			}
			// Loopback loss should be very low.
			if pm.LossPct > 5.0 {
				t.Errorf("engine %s -> %s: loss = %.1f%%, want < 5%%", engines[i].name, peerName, pm.LossPct)
			}
			t.Logf("engine %s -> %s: p50=%.3fms p95=%.3fms p99=%.3fms jitter=%.3fms loss=%.1f%%",
				engines[i].name, peerName, pm.P50Ms, pm.P95Ms, pm.P99Ms, pm.JitterMs, pm.LossPct)
		}
	}
}

func TestEngineCleanShutdown(t *testing.T) {
	e := NewEngine(EngineConfig{
		ProbePort:     19010,
		ProbeInterval: 100 * time.Millisecond,
		ProbeTimeout:  1 * time.Second,
		NodeID:        [16]byte{0xFF},
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	cancel()
	e.Stop() // should not hang

	// Verify calling Stop again is safe.
	e.Stop()
}
