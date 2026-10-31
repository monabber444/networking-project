package cluster

import (
	"testing"
	"time"
)

func TestSingleNodeCluster(t *testing.T) {
	c, err := NewCluster(ClusterConfig{
		NodeName:   "test-node-1",
		BindAddr:   "127.0.0.1",
		GossipPort: 17946,
		ProbePort:  19000,
		HTTPPort:   18080,
	})
	if err != nil {
		t.Fatalf("NewCluster: %v", err)
	}
	defer c.Shutdown()

	members := c.Members()
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Name != "test-node-1" {
		t.Errorf("name = %q, want %q", members[0].Name, "test-node-1")
	}
	if members[0].ProbePort != 19000 {
		t.Errorf("ProbePort = %d, want 19000", members[0].ProbePort)
	}
	if members[0].HTTPPort != 18080 {
		t.Errorf("HTTPPort = %d, want 18080", members[0].HTTPPort)
	}
}

func TestTwoNodeJoin(t *testing.T) {
	c1, err := NewCluster(ClusterConfig{
		NodeName:   "node-a",
		BindAddr:   "127.0.0.1",
		GossipPort: 17950,
		ProbePort:  19100,
		HTTPPort:   18100,
	})
	if err != nil {
		t.Fatalf("NewCluster c1: %v", err)
	}
	defer c1.Shutdown()

	c2, err := NewCluster(ClusterConfig{
		NodeName:   "node-b",
		BindAddr:   "127.0.0.1",
		GossipPort: 17951,
		ProbePort:  19101,
		HTTPPort:   18101,
	})
	if err != nil {
		t.Fatalf("NewCluster c2: %v", err)
	}
	defer c2.Shutdown()

	if err := c2.Join([]string{"127.0.0.1:17950"}); err != nil {
		t.Fatalf("Join: %v", err)
	}

	// Give memberlist a moment to synchronize.
	time.Sleep(500 * time.Millisecond)

	m1 := c1.Members()
	m2 := c2.Members()

	if len(m1) != 2 {
		t.Errorf("c1 members = %d, want 2", len(m1))
	}
	if len(m2) != 2 {
		t.Errorf("c2 members = %d, want 2", len(m2))
	}
}

func TestMetadataEncodingDecoding(t *testing.T) {
	data := encodeMetadata(9000, 8080)
	probePort, httpPort, err := decodeMetadata(data)
	if err != nil {
		t.Fatalf("decodeMetadata: %v", err)
	}
	if probePort != 9000 {
		t.Errorf("probePort = %d, want 9000", probePort)
	}
	if httpPort != 8080 {
		t.Errorf("httpPort = %d, want 8080", httpPort)
	}
}

func TestMetadataDecodeTooShort(t *testing.T) {
	_, _, err := decodeMetadata([]byte{0x00})
	if err == nil {
		t.Error("expected error for short metadata")
	}
}

func TestNodeLeaveDetection(t *testing.T) {
	c1, err := NewCluster(ClusterConfig{
		NodeName:   "stay-node",
		BindAddr:   "127.0.0.1",
		GossipPort: 17960,
		ProbePort:  19200,
		HTTPPort:   18200,
	})
	if err != nil {
		t.Fatalf("NewCluster c1: %v", err)
	}
	defer c1.Shutdown()

	c2, err := NewCluster(ClusterConfig{
		NodeName:   "leave-node",
		BindAddr:   "127.0.0.1",
		GossipPort: 17961,
		ProbePort:  19201,
		HTTPPort:   18201,
	})
	if err != nil {
		t.Fatalf("NewCluster c2: %v", err)
	}

	if err := c2.Join([]string{"127.0.0.1:17960"}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if len(c1.Members()) != 2 {
		t.Fatalf("expected 2 members before leave, got %d", len(c1.Members()))
	}

	// Node 2 leaves.
	if err := c2.Shutdown(); err != nil {
		t.Fatalf("Shutdown c2: %v", err)
	}

	// Wait for leave detection.
	time.Sleep(2 * time.Second)

	members := c1.Members()
	if len(members) != 1 {
		t.Errorf("expected 1 member after leave, got %d", len(members))
	}
}
