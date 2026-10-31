// Package cluster provides gossip-based cluster membership for PulseMesh
// using HashiCorp memberlist.
package cluster

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hashicorp/memberlist"
)

// NodeInfo describes a cluster member and its service ports.
type NodeInfo struct {
	Name      string
	Addr      string
	ProbePort int
	HTTPPort  int
}

// ClusterConfig holds configuration for creating a Cluster.
type ClusterConfig struct {
	NodeName   string
	BindAddr   string
	GossipPort int
	ProbePort  int
	HTTPPort   int
}

// Cluster wraps a memberlist.Memberlist and provides a higher-level API for
// PulseMesh cluster membership.
type Cluster struct {
	ml *memberlist.Memberlist

	mu    sync.RWMutex
	nodes map[string]NodeInfo

	localProbePort int
	localHTTPPort  int
}

// metadataSize is the binary-encoded metadata size: 2 bytes ProbePort + 2 bytes HTTPPort.
const metadataSize = 4

// encodeMetadata packs ProbePort and HTTPPort into 4 bytes (big-endian uint16 each).
func encodeMetadata(probePort, httpPort int) []byte {
	buf := make([]byte, metadataSize)
	binary.BigEndian.PutUint16(buf[0:2], uint16(probePort))
	binary.BigEndian.PutUint16(buf[2:4], uint16(httpPort))
	return buf
}

// decodeMetadata unpacks ProbePort and HTTPPort from the binary metadata.
func decodeMetadata(data []byte) (probePort, httpPort int, err error) {
	if len(data) < metadataSize {
		return 0, 0, fmt.Errorf("metadata too short: %d bytes", len(data))
	}
	probePort = int(binary.BigEndian.Uint16(data[0:2]))
	httpPort = int(binary.BigEndian.Uint16(data[2:4]))
	return probePort, httpPort, nil
}

// delegate implements memberlist.Delegate to provide node metadata.
type delegate struct {
	meta []byte
}

func (d *delegate) NodeMeta(limit int) []byte {
	if len(d.meta) > limit {
		return d.meta[:limit]
	}
	return d.meta
}

func (d *delegate) NotifyMsg([]byte)            {}
func (d *delegate) GetBroadcasts(int, int) [][]byte { return nil }
func (d *delegate) LocalState(bool) []byte       { return nil }
func (d *delegate) MergeRemoteState([]byte, bool) {}

// eventDelegate implements memberlist.EventDelegate for join/leave tracking.
type eventDelegate struct {
	c *Cluster
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	ed.c.addNode(node)
	slog.Info("node joined cluster", "name", node.Name, "addr", node.Addr)
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	ed.c.removeNode(node.Name)
	slog.Info("node left cluster", "name", node.Name)
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	ed.c.addNode(node)
	slog.Info("node updated", "name", node.Name)
}

// addNode upserts a node in the local tracking map.
func (c *Cluster) addNode(node *memberlist.Node) {
	probePort, httpPort, err := decodeMetadata(node.Meta)
	if err != nil {
		slog.Warn("failed to decode node metadata", "name", node.Name, "err", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[node.Name] = NodeInfo{
		Name:      node.Name,
		Addr:      node.Addr.String(),
		ProbePort: probePort,
		HTTPPort:  httpPort,
	}
}

// removeNode deletes a node from the local tracking map.
func (c *Cluster) removeNode(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, name)
}

// NewCluster creates a new memberlist-backed cluster.
func NewCluster(cfg ClusterConfig) (*Cluster, error) {
	c := &Cluster{
		nodes:          make(map[string]NodeInfo),
		localProbePort: cfg.ProbePort,
		localHTTPPort:  cfg.HTTPPort,
	}

	mlCfg := memberlist.DefaultLANConfig()
	mlCfg.Name = cfg.NodeName
	mlCfg.BindAddr = cfg.BindAddr
	mlCfg.BindPort = cfg.GossipPort
	mlCfg.AdvertisePort = cfg.GossipPort
	mlCfg.LogOutput = &slogWriter{}

	mlCfg.Delegate = &delegate{
		meta: encodeMetadata(cfg.ProbePort, cfg.HTTPPort),
	}
	mlCfg.Events = &eventDelegate{c: c}

	ml, err := memberlist.Create(mlCfg)
	if err != nil {
		return nil, fmt.Errorf("memberlist create: %w", err)
	}
	c.ml = ml

	return c, nil
}

// Join attempts to join an existing cluster via the given peer addresses.
func (c *Cluster) Join(peers []string) error {
	if len(peers) == 0 {
		return nil
	}
	n, err := c.ml.Join(peers)
	if err != nil {
		return fmt.Errorf("join %d peers: %w", len(peers), err)
	}
	slog.Info("joined cluster", "contacted", n)
	return nil
}

// Members returns info about all alive members (including self).
func (c *Cluster) Members() []NodeInfo {
	members := c.ml.Members()
	result := make([]NodeInfo, 0, len(members))
	for _, m := range members {
		probePort, httpPort, err := decodeMetadata(m.Meta)
		if err != nil {
			continue
		}
		result = append(result, NodeInfo{
			Name:      m.Name,
			Addr:      m.Addr.String(),
			ProbePort: probePort,
			HTTPPort:  httpPort,
		})
	}
	return result
}

// LocalNode returns the name of the local node.
func (c *Cluster) LocalNode() string {
	return c.ml.LocalNode().Name
}

// Shutdown gracefully leaves the cluster and shuts down memberlist.
func (c *Cluster) Shutdown() error {
	if err := c.ml.Leave(5e9); err != nil { // 5 second timeout
		slog.Warn("memberlist leave error", "err", err)
	}
	return c.ml.Shutdown()
}

// slogWriter adapts slog for memberlist's io.Writer-based logging.
type slogWriter struct{}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	slog.Debug(string(p))
	return len(p), nil
}
