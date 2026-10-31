package prober

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/youruser/pulsemesh/internal/metrics"
)

// PeerInfo describes a remote peer that the engine should probe.
type PeerInfo struct {
	Name      string
	Addr      string
	ProbePort int
}

// PeerMetrics is a snapshot of computed metrics for a single peer.
type PeerMetrics = metrics.PeerMetrics

// EngineConfig holds configuration for the probe Engine.
type EngineConfig struct {
	ProbePort     int
	ProbeInterval time.Duration
	ProbeTimeout  time.Duration
	NodeID        [16]byte
}

// inflightEntry tracks a single in-flight ping.
type inflightEntry struct {
	peerName string
	sentAt   time.Time
}

// Engine is the PulseMesh UDP probe engine. It sends periodic PING probes to
// peers and records RTT measurements.
type Engine struct {
	cfg EngineConfig

	mu      sync.RWMutex
	peers   []PeerInfo
	buffers map[string]*metrics.RingBuffer

	conn     *net.UDPConn
	seq      atomic.Uint64
	inflight sync.Map // seq -> inflightEntry

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewEngine creates a new probe Engine with the given configuration.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		cfg:     cfg,
		buffers: make(map[string]*metrics.RingBuffer),
	}
}

// Start begins listening for probes and sending periodic pings.
// It blocks until the listener is ready, then runs background goroutines
// until ctx is cancelled or Stop is called.
func (e *Engine) Start(ctx context.Context) error {
	addr := &net.UDPAddr{Port: e.cfg.ProbePort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	e.conn = conn

	ctx, e.cancel = context.WithCancel(ctx)

	// Reader goroutine.
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.readLoop(ctx)
	}()

	// Probe ticker goroutine.
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.probeLoop(ctx)
	}()

	// Reaper goroutine — marks timed-out probes as lost.
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.reaperLoop(ctx)
	}()

	slog.Info("probe engine started", "port", e.cfg.ProbePort)
	return nil
}

// SetPeers replaces the set of known peers.
func (e *Engine) SetPeers(peers []PeerInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.peers = make([]PeerInfo, len(peers))
	copy(e.peers, peers)

	// Ensure every peer has a ring buffer.
	for _, p := range peers {
		if _, ok := e.buffers[p.Name]; !ok {
			e.buffers[p.Name] = metrics.NewRingBuffer(1000)
		}
	}
}

// GetMetrics returns the ring buffer for a specific peer, or nil if unknown.
func (e *Engine) GetMetrics(peer string) *metrics.RingBuffer {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buffers[peer]
}

// GetAllMetrics returns a snapshot of computed metrics for every known peer.
func (e *Engine) GetAllMetrics() map[string]PeerMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make(map[string]PeerMetrics, len(e.buffers))
	for name, rb := range e.buffers {
		out[name] = PeerMetrics{
			P50Ms:    durationToMs(rb.P50()),
			P95Ms:    durationToMs(rb.P95()),
			P99Ms:    durationToMs(rb.P99()),
			JitterMs: durationToMs(rb.Jitter()),
			LossPct:  rb.LossRate() * 100.0,
		}
	}
	return out
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.conn != nil {
		_ = e.conn.Close()
	}
	e.wg.Wait()
	slog.Info("probe engine stopped")
}

// readLoop reads incoming UDP packets and handles them.
func (e *Engine) readLoop(ctx context.Context) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = e.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, remoteAddr, err := e.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				slog.Warn("read error", "err", err)
				continue
			}
		}

		pkt, err := Unmarshal(buf[:n])
		if err != nil {
			slog.Debug("invalid packet", "err", err, "from", remoteAddr)
			continue
		}

		switch pkt.MsgType {
		case MsgTypePing:
			e.handlePing(pkt, remoteAddr)
		case MsgTypePong:
			e.handlePong(pkt)
		}
	}
}

// handlePing responds to an incoming ping with a pong.
func (e *Engine) handlePing(ping Packet, remote *net.UDPAddr) {
	pong := NewPong(ping, e.cfg.NodeID)
	data, err := Marshal(pong)
	if err != nil {
		slog.Warn("marshal pong failed", "err", err)
		return
	}
	if _, err := e.conn.WriteToUDP(data, remote); err != nil {
		slog.Debug("send pong failed", "err", err, "to", remote)
	}
}

// handlePong processes an incoming pong, computing RTT.
func (e *Engine) handlePong(pong Packet) {
	val, ok := e.inflight.LoadAndDelete(pong.SequenceNum)
	if !ok {
		return // unknown or already-reaped sequence
	}
	entry := val.(inflightEntry)
	rtt := time.Since(entry.sentAt)

	e.mu.RLock()
	rb := e.buffers[entry.peerName]
	e.mu.RUnlock()

	if rb != nil {
		rb.Add(metrics.Sample{
			RTTNanos:  rtt.Nanoseconds(),
			Timestamp: time.Now(),
		})
	}
}

// probeLoop sends PINGs to up to K=3 random peers every ProbeInterval.
func (e *Engine) probeLoop(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.sendProbes()
		}
	}
}

// sendProbes selects up to 3 random peers and sends a PING to each.
func (e *Engine) sendProbes() {
	e.mu.RLock()
	peers := make([]PeerInfo, len(e.peers))
	copy(peers, e.peers)
	e.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	// Shuffle and take up to 3.
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	k := 3
	if len(peers) < k {
		k = len(peers)
	}

	for _, p := range peers[:k] {
		seq := e.seq.Add(1)
		ping := NewPing(seq, e.cfg.NodeID)
		data, err := Marshal(ping)
		if err != nil {
			continue
		}

		addr := &net.UDPAddr{
			IP:   net.ParseIP(p.Addr),
			Port: p.ProbePort,
		}
		e.inflight.Store(seq, inflightEntry{
			peerName: p.Name,
			sentAt:   time.Now(),
		})

		if _, err := e.conn.WriteToUDP(data, addr); err != nil {
			slog.Debug("send ping failed", "err", err, "peer", p.Name)
		}
	}
}

// reaperLoop runs every second and marks probes exceeding ProbeTimeout as lost.
func (e *Engine) reaperLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reapExpired()
		}
	}
}

// reapExpired marks timed-out in-flight probes as lost.
func (e *Engine) reapExpired() {
	now := time.Now()
	e.inflight.Range(func(key, value any) bool {
		entry := value.(inflightEntry)
		if now.Sub(entry.sentAt) > e.cfg.ProbeTimeout {
			e.inflight.Delete(key)

			e.mu.RLock()
			rb := e.buffers[entry.peerName]
			e.mu.RUnlock()

			if rb != nil {
				rb.Add(metrics.Sample{
					Timestamp: now,
					Lost:      true,
				})
			}
		}
		return true
	})
}

func durationToMs(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond)
}
